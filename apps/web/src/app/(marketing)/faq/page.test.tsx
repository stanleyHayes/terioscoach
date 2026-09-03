import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import FAQPage from "./page";

const listFAQs = vi.hoisted(() => vi.fn());
vi.mock("@/lib/content", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/content")>();
  return { ...original, listFAQs };
});

async function renderPage() {
  render(await FAQPage());
}

describe("FAQPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listFAQs.mockResolvedValue([
      {
        id: "1",
        question: "How do I pay?",
        answer: "Card or mobile money.",
        category: "Booking",
        sortOrder: 1,
      },
    ]);
  });

  it("renders the searchable list", async () => {
    await renderPage();

    expect(screen.getByLabelText(/search the questions/i)).toBeTruthy();
    expect(screen.getByRole("button", { name: /how do i pay/i })).toBeTruthy();
    expect(screen.queryByAltText(/quiet mountain lake/i)).toBeNull();
  });

  it("always offers a way to ask something that is not listed", async () => {
    await renderPage();

    expect(screen.getByRole("link", { name: /get in touch/i }).getAttribute("href")).toBe(
      "/contact",
    );
  });

  it("invites a question when nothing is published yet", async () => {
    listFAQs.mockResolvedValue([]);

    await renderPage();

    expect(screen.getByRole("heading", { name: /no questions published yet/i })).toBeTruthy();
    expect(screen.getByRole("link", { name: /ask a question/i })).toBeTruthy();
  });

  it("renders a branded error instead of crashing when the API is down", async () => {
    listFAQs.mockRejectedValue(new Error("network"));

    await renderPage();

    expect(screen.getByRole("alert").textContent).toMatch(/didn’t load/i);
  });
});
