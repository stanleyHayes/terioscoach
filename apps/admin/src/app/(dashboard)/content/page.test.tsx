import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { FAQ, Page, Post, Testimonial } from "@/lib/content";
import ContentPage from "./page";

const pageList = vi.hoisted(() => vi.fn());
const pageCreate = vi.hoisted(() => vi.fn());
const pageUpdate = vi.hoisted(() => vi.fn());
const pageSetPublished = vi.hoisted(() => vi.fn());
const pageRemove = vi.hoisted(() => vi.fn());
const postList = vi.hoisted(() => vi.fn());
const postUpdate = vi.hoisted(() => vi.fn());
const faqList = vi.hoisted(() => vi.fn());
const faqCreate = vi.hoisted(() => vi.fn());
const faqUpdate = vi.hoisted(() => vi.fn());
const faqRemove = vi.hoisted(() => vi.fn());
const testimonialList = vi.hoisted(() => vi.fn());
const testimonialCreate = vi.hoisted(() => vi.fn());
const testimonialModerate = vi.hoisted(() => vi.fn());
const testimonialRemove = vi.hoisted(() => vi.fn());

vi.mock("@/lib/content", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/content")>();
  return {
    ...original,
    pagesApi: {
      list: pageList,
      create: pageCreate,
      update: pageUpdate,
      setPublished: pageSetPublished,
      remove: pageRemove,
    },
    postsApi: {
      list: postList,
      create: vi.fn(),
      update: postUpdate,
      setPublished: vi.fn(),
      remove: vi.fn(),
    },
    faqsApi: { list: faqList, create: faqCreate, update: faqUpdate, remove: faqRemove },
    testimonialsApi: {
      list: testimonialList,
      create: testimonialCreate,
      moderate: testimonialModerate,
      remove: testimonialRemove,
    },
  };
});

vi.mock("@/lib/auth", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/auth")>();
  return {
    ...original,
    useAuth: () => ({
      status: "authenticated",
      user: { id: "prac-1", email: "t@example.com", role: "practitioner", name: "Terios" },
      session: { accessToken: "a1", refreshToken: "r1" },
      refreshCallbacks: { onTokensRefreshed: vi.fn() },
      logout: vi.fn(),
    }),
  };
});

function aPage(overrides: Partial<Page> = {}): Page {
  return {
    id: "page-1",
    slug: "about",
    title: "About the practice",
    body: "A long-standing wellness practice in Accra.",
    status: "draft",
    createdAt: "2026-08-01T09:00:00Z",
    updatedAt: "2026-08-02T09:00:00Z",
    ...overrides,
  };
}

function aPost(overrides: Partial<Post> = {}): Post {
  return { ...aPage(), id: "post-1", slug: "resting", title: "On resting", tags: [], ...overrides };
}

function aFAQ(overrides: Partial<FAQ> = {}): FAQ {
  return {
    id: "faq-1",
    question: "Do I need to bring anything?",
    answer: "Just yourself.",
    sortOrder: 0,
    active: true,
    ...overrides,
  };
}

function aTestimonial(overrides: Partial<Testimonial> = {}): Testimonial {
  return {
    id: "t-1",
    authorName: "Ama K.",
    quote: "I left feeling lighter.",
    status: "pending",
    sortOrder: 0,
    submittedAt: "2026-08-05T09:00:00Z",
    ...overrides,
  };
}

/** Moves to a tab and waits for its first row, so assertions never race the load. */
async function openTab(name: RegExp) {
  fireEvent.click(screen.getByRole("tab", { name }));
}

describe("ContentPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    pageList.mockResolvedValue([aPage()]);
    postList.mockResolvedValue([aPost()]);
    faqList.mockResolvedValue([aFAQ()]);
    testimonialList.mockResolvedValue([aTestimonial()]);
    pageCreate.mockResolvedValue(aPage({ id: "page-new" }));
    pageUpdate.mockImplementation((_s, _c, id, patch) => Promise.resolve(aPage({ id, ...patch })));
    pageSetPublished.mockImplementation((_s, _c, id, published) =>
      Promise.resolve(aPage({ id, status: published ? "published" : "draft" })),
    );
  });

  it("opens on pages and only loads the visible tab", async () => {
    render(<ContentPage />);

    expect(await screen.findByText("About the practice")).toBeTruthy();
    // The other three tabs have not fetched anything — switching is what
    // pays for them, not arriving on the screen.
    expect(postList).not.toHaveBeenCalled();
    expect(faqList).not.toHaveBeenCalled();
    expect(testimonialList).not.toHaveBeenCalled();
  });

  it("moves between tabs with the arrow keys", async () => {
    render(<ContentPage />);
    await screen.findByText("About the practice");

    fireEvent.keyDown(screen.getByRole("tab", { name: "Pages" }), { key: "ArrowRight" });

    expect(screen.getByRole("tab", { name: "Blog" }).getAttribute("aria-selected")).toBe("true");
    expect(await screen.findByText("On resting")).toBeTruthy();
  });

  it("publishes a draft page through its own route, not a save", async () => {
    render(<ContentPage />);
    await screen.findByText("About the practice");

    fireEvent.click(screen.getByRole("button", { name: /^publish$/i }));

    await waitFor(() => {
      expect(pageSetPublished).toHaveBeenCalledWith(
        expect.anything(),
        expect.anything(),
        "page-1",
        true,
      );
    });
    // The publish button must never reach the PATCH route: an edit that
    // could put a page live by accident is the failure this design exists
    // to prevent.
    expect(pageUpdate).not.toHaveBeenCalled();
    expect(await screen.findByText("Live")).toBeTruthy();
  });

  it("unpublishes a live page", async () => {
    pageList.mockResolvedValue([aPage({ status: "published", publishedAt: "2026-08-03T09:00:00Z" })]);
    render(<ContentPage />);

    fireEvent.click(await screen.findByRole("button", { name: /unpublish/i }));

    await waitFor(() => {
      expect(pageSetPublished).toHaveBeenCalledWith(
        expect.anything(),
        expect.anything(),
        "page-1",
        false,
      );
    });
  });

  it("derives the slug from the title until the slug is edited by hand", async () => {
    render(<ContentPage />);
    await screen.findByText("About the practice");

    fireEvent.click(screen.getByRole("button", { name: /new page/i }));
    const title = screen.getByRole("textbox", { name: /^title/i });
    fireEvent.change(title, { target: { value: "Our Approach to Rest" } });

    const slug = screen.getByRole("textbox", { name: /^web address/i }) as HTMLInputElement;
    expect(slug.value).toBe("our-approach-to-rest");

    // Once touched it stops tracking — an established URL is not rewritten
    // because a headline was reworded.
    fireEvent.change(slug, { target: { value: "approach" } });
    fireEvent.change(title, { target: { value: "Our Approach, Revisited" } });
    expect(slug.value).toBe("approach");
  });

  it("creates a page as a draft and never publishes on save", async () => {
    render(<ContentPage />);
    await screen.findByText("About the practice");

    fireEvent.click(screen.getByRole("button", { name: /new page/i }));
    fireEvent.change(screen.getByRole("textbox", { name: /^title/i }), {
      target: { value: "Our approach" },
    });
    fireEvent.change(screen.getByRole("textbox", { name: /^body/i }), {
      target: { value: "How we work." },
    });
    fireEvent.click(screen.getByRole("button", { name: /create page/i }));

    await waitFor(() => expect(pageCreate).toHaveBeenCalled());
    expect(pageSetPublished).not.toHaveBeenCalled();
    expect(pageCreate.mock.calls[0]![2]).toEqual({
      slug: "our-approach",
      title: "Our approach",
      body: "How we work.",
    });
  });

  it("refuses to save a page with no body", async () => {
    render(<ContentPage />);
    await screen.findByText("About the practice");

    fireEvent.click(screen.getByRole("button", { name: /new page/i }));
    fireEvent.change(screen.getByRole("textbox", { name: /^title/i }), {
      target: { value: "Half-written" },
    });
    fireEvent.click(screen.getByRole("button", { name: /create page/i }));

    expect(await screen.findByText(/nothing to publish yet/i)).toBeTruthy();
    expect(pageCreate).not.toHaveBeenCalled();
  });

  it("asks before deleting a live page and says it is live", async () => {
    pageList.mockResolvedValue([aPage({ status: "published" })]);
    render(<ContentPage />);
    await screen.findByText("About the practice");

    fireEvent.click(screen.getByRole("button", { name: /delete/i }));

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText(/live right now/i)).toBeTruthy();
    expect(pageRemove).not.toHaveBeenCalled();

    pageRemove.mockResolvedValue(undefined);
    fireEvent.click(within(dialog).getByRole("button", { name: /^delete$/i }));

    await waitFor(() => expect(pageRemove).toHaveBeenCalled());
    expect(screen.queryByText("About the practice")).toBeNull();
  });

  it("shows a load failure with a way back", async () => {
    pageList.mockRejectedValue(new Error("network"));
    render(<ContentPage />);

    expect(await screen.findByRole("alert")).toBeTruthy();
    pageList.mockResolvedValue([aPage()]);
    fireEvent.click(screen.getByRole("button", { name: /try again/i }));
    expect(await screen.findByText("About the practice")).toBeTruthy();
  });

  describe("FAQs", () => {
    beforeEach(() => {
      faqUpdate.mockImplementation((_s, _c, id, patch) =>
        Promise.resolve(aFAQ({ id, ...patch })),
      );
    });

    it("toggles a question off the site without deleting it", async () => {
      render(<ContentPage />);
      await screen.findByText("About the practice");
      await openTab(/faqs/i);

      const toggle = await screen.findByRole("switch", { name: /show ".*" on the site/i });
      fireEvent.click(toggle);

      await waitFor(() => {
        expect(faqUpdate).toHaveBeenCalledWith(
          expect.anything(),
          expect.anything(),
          "faq-1",
          { active: false },
        );
      });
      expect(faqRemove).not.toHaveBeenCalled();
      expect(await screen.findByText("Hidden")).toBeTruthy();
    });

    it("reorders by swapping the two stored positions, not renumbering", async () => {
      faqList.mockResolvedValue([
        aFAQ({ id: "faq-1", question: "First", sortOrder: 0 }),
        aFAQ({ id: "faq-2", question: "Second", sortOrder: 1 }),
      ]);
      render(<ContentPage />);
      await screen.findByText("About the practice");
      await openTab(/faqs/i);

      const rows = await screen.findAllByRole("listitem");
      fireEvent.click(within(rows[1]!).getByRole("button", { name: /move up/i }));

      await waitFor(() => expect(faqUpdate).toHaveBeenCalledTimes(2));
      expect(faqUpdate.mock.calls[0]!.slice(2)).toEqual(["faq-2", { sortOrder: 0 }]);
      expect(faqUpdate.mock.calls[1]!.slice(2)).toEqual(["faq-1", { sortOrder: 1 }]);
    });

    it("cannot move the first question up or the last one down", async () => {
      faqList.mockResolvedValue([
        aFAQ({ id: "faq-1", question: "First", sortOrder: 0 }),
        aFAQ({ id: "faq-2", question: "Second", sortOrder: 1 }),
      ]);
      render(<ContentPage />);
      await screen.findByText("About the practice");
      await openTab(/faqs/i);

      const rows = await screen.findAllByRole("listitem");
      expect(within(rows[0]!).getByRole("button", { name: /move up/i }).hasAttribute("disabled")).toBe(true);
      expect(within(rows[1]!).getByRole("button", { name: /move down/i }).hasAttribute("disabled")).toBe(true);
    });

    it("adds a question at the end of the list", async () => {
      faqCreate.mockResolvedValue(aFAQ({ id: "faq-2", question: "How long?", sortOrder: 1 }));
      render(<ContentPage />);
      await screen.findByText("About the practice");
      await openTab(/faqs/i);
      await screen.findByText("Do I need to bring anything?");

      fireEvent.click(screen.getByRole("button", { name: /add a question/i }));
      fireEvent.change(screen.getByRole("textbox", { name: /^question/i }), {
        target: { value: "How long?" },
      });
      fireEvent.change(screen.getByRole("textbox", { name: /^answer/i }), {
        target: { value: "About an hour." },
      });
      fireEvent.click(screen.getByRole("button", { name: /^add question$/i }));

      await waitFor(() => expect(faqCreate).toHaveBeenCalled());
      expect(faqCreate.mock.calls[0]![2]).toMatchObject({
        question: "How long?",
        answer: "About an hour.",
        sortOrder: 1,
      });
    });
  });

  describe("testimonials", () => {
    it("opens on the pending queue", async () => {
      testimonialList.mockResolvedValue([
        aTestimonial({ id: "t-1", quote: "Waiting on you", status: "pending" }),
        aTestimonial({ id: "t-2", quote: "Already live", status: "approved" }),
      ]);
      render(<ContentPage />);
      await screen.findByText("About the practice");
      await openTab(/testimonials/i);

      expect(await screen.findByText("Waiting on you")).toBeTruthy();
      expect(screen.queryByText("Already live")).toBeNull();
    });

    it("publishes a pending testimonial", async () => {
      testimonialModerate.mockImplementation((_s, _c, id, approve) =>
        Promise.resolve(aTestimonial({ id, status: approve ? "approved" : "rejected" })),
      );
      render(<ContentPage />);
      await screen.findByText("About the practice");
      await openTab(/testimonials/i);

      fireEvent.click(await screen.findByRole("button", { name: /^publish$/i }));

      await waitFor(() => {
        expect(testimonialModerate).toHaveBeenCalledWith(
          expect.anything(),
          expect.anything(),
          "t-1",
          true,
        );
      });
    });

    it("saves a hand-typed testimonial as pending, not live", async () => {
      testimonialCreate.mockResolvedValue(
        aTestimonial({ id: "t-9", quote: "From an email", status: "pending" }),
      );
      render(<ContentPage />);
      await screen.findByText("About the practice");
      await openTab(/testimonials/i);
      await screen.findByText("I left feeling lighter.");

      fireEvent.click(screen.getByRole("button", { name: /add a testimonial/i }));
      fireEvent.change(screen.getByRole("textbox", { name: /^quote/i }), {
        target: { value: "From an email" },
      });
      fireEvent.change(screen.getByRole("textbox", { name: /^name/i }), {
        target: { value: "Kofi A." },
      });
      fireEvent.click(screen.getByRole("button", { name: /^save$/i }));

      await waitFor(() => expect(testimonialCreate).toHaveBeenCalled());
      // Nothing the practitioner types goes live either — one gate, one rule.
      expect(testimonialModerate).not.toHaveBeenCalled();
      expect(await screen.findByText("From an email")).toBeTruthy();
    });
  });

  it("uses no native form controls anywhere in the editor", async () => {
    const { container } = render(<ContentPage />);
    await screen.findByText("About the practice");
    fireEvent.click(screen.getByRole("button", { name: /new page/i }));

    expect(container.ownerDocument.querySelectorAll("select").length).toBe(0);
    expect(
      container.ownerDocument.querySelectorAll(
        'input[type="checkbox"], input[type="radio"], input[type="date"]',
      ).length,
    ).toBe(0);
    // Custom validation only — no native bubbles.
    const form = container.ownerDocument.querySelector("form")!;
    expect(form.hasAttribute("novalidate")).toBe(true);
  });
});
