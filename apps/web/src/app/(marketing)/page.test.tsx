import { render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import Home from "./page";

// The home page reads approved social proof from the CMS and reviews APIs.
const listTestimonials = vi.hoisted(() => vi.fn());
const listReviews = vi.hoisted(() => vi.fn());
const getReviewSummary = vi.hoisted(() => vi.fn());
const getPage = vi.hoisted(() => vi.fn());
const listServices = vi.hoisted(() => vi.fn());
vi.mock("@/lib/content", () => ({ listTestimonials, listReviews, getReviewSummary, getPage }));
vi.mock("@/lib/api", () => ({ listServices }));

/** Renders the async server component. */
async function renderHome() {
  render(await Home());
}

describe("Home page", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listTestimonials.mockResolvedValue([
      { id: "t1", authorName: "A long-term client", authorRole: "Coaching", quote: "Calm." },
      { id: "t2", authorName: "A client abroad", quote: "Looked after." },
    ]);
    listReviews.mockResolvedValue([]);
    getReviewSummary.mockResolvedValue({ count: 2, average: 5, distribution: { "5": 2 } });
    getPage.mockResolvedValue({ slug: "home", coverImage: "/custom-home.webp" });
    listServices.mockResolvedValue([
      { id: "svc-1", name: "Wellness coaching", description: "Personal support.", imageUrl: "https://images.example/coaching.webp", durationMinutes: 60, priceKobo: 10000, currency: "USD", sortOrder: 0 },
      { id: "svc-2", name: "Nursing consultations", description: "Clinical guidance.", imageUrl: "", durationMinutes: 45, priceKobo: 8000, currency: "USD", sortOrder: 1 },
    ]);
  });

  it("renders the hero headline and lead", async () => {
    await renderHome();
    expect(
      screen.getByRole("heading", {
        level: 1,
        name: /care that lets you exhale/i,
      }),
    ).toBeTruthy();
    expect(screen.getByAltText(/registered nurse and wellness coach/i).getAttribute("src"))
      .toContain("/_next/image?url=%2Fcustom-home.webp");
  });

  it("links hero CTAs to the right routes", async () => {
    await renderHome();
    const bookLinks = screen.getAllByRole("link", { name: /book a session/i });
    expect(bookLinks.length).toBeGreaterThanOrEqual(1);
    for (const link of bookLinks) {
      expect(link.getAttribute("href")).toBe("/work-with-me");
    }
    expect(
      screen.getByRole("link", { name: /explore services/i }).getAttribute("href"),
    ).toBe("/services");
  });

  it("renders the trust strip proof points", async () => {
    await renderHome();
    const trust = screen.getByRole("region", {
      name: /why clients trust terios/i,
    });
    for (const label of [
      /sessions by video/i,
      /clients worldwide/i,
      /secure client portal/i,
    ]) {
      expect(within(trust).getByText(label)).toBeTruthy();
    }
  });

  it("renders the live services preview with dashboard images and booking links", async () => {
    await renderHome();
    const services = screen.getByRole("region", {
      name: /care that fits the season you are in/i,
    });
    expect(within(services).getByText(/wellness coaching/i)).toBeTruthy();
    expect(within(services).getByText(/nursing consultations/i)).toBeTruthy();
    const cardLinks = within(services).getAllByRole("link");
    expect(cardLinks).toHaveLength(2);
    expect(cardLinks[0]?.getAttribute("href")).toBe("/work-with-me?service=svc-1");
    expect(cardLinks[1]?.getAttribute("href")).toBe("/work-with-me?service=svc-2");
    expect(services.querySelector('img[src="https://images.example/coaching.webp"]')).toBeTruthy();
  });

  it("links the approach teaser to /about", async () => {
    await renderHome();
    expect(
      screen.getByRole("link", { name: /about the practice/i }).getAttribute("href"),
    ).toBe("/about");
  });

  it("shows the approved testimonials the API returned", async () => {
    await renderHome();

    const section = screen.getByRole("region", { name: /what clients say/i });
    expect(within(section).getAllByRole("figure")).toHaveLength(2);
    expect(within(section).getByText(/a long-term client/i)).toBeTruthy();
    expect(within(section).getByText(/5.0/)).toBeTruthy();
  });

  it("gives a single testimonial a dedicated editorial layout", async () => {
    listTestimonials.mockResolvedValue([
      { id: "t1", authorName: "", quote: "I felt heard and supported." },
    ]);

    await renderHome();

    const section = screen.getByRole("region", { name: /what clients say/i });
    expect(within(section).getByText(/client reflection/i)).toBeTruthy();
    expect(within(section).getByText(/i felt heard and supported/i)).toBeTruthy();
    expect(within(section).getAllByText(/terios client/i).length).toBeGreaterThan(0);
  });

  it("shows client reviews alongside the curated quotes", async () => {
    listReviews.mockResolvedValue([
      {
        id: "r1",
        authorName: "Ama",
        serviceName: "Deep Tissue Massage",
        rating: 5,
        comment: "Wonderful session.",
        createdAt: "2026-08-01T09:00:00Z",
      },
    ]);

    await renderHome();

    const section = screen.getByRole("region", { name: /what clients say/i });
    expect(within(section).getByText(/wonderful session/i)).toBeTruthy();
    expect(within(section).getByText(/deep tissue massage/i)).toBeTruthy();
  });

  it("omits the section entirely when nothing has been approved", async () => {
    listTestimonials.mockResolvedValue([]);
    listReviews.mockResolvedValue([]);
    getReviewSummary.mockResolvedValue({ count: 0, average: 0, distribution: {} });

    await renderHome();

    expect(screen.queryByRole("region", { name: /what clients say/i })).toBeNull();
  });

  it("still renders the page when the content API is down", async () => {
    listTestimonials.mockRejectedValue(new Error("network"));
    listReviews.mockRejectedValue(new Error("network"));
    getReviewSummary.mockRejectedValue(new Error("network"));

    await renderHome();

    // The hero and the closing CTA are the page's job; social proof is not
    // worth taking the home page down for.
    expect(screen.getByRole("heading", { level: 1 })).toBeTruthy();
    expect(screen.queryByRole("region", { name: /what clients say/i })).toBeNull();
  });

  it("ends with a closing CTA to /work-with-me", async () => {
    await renderHome();
    const cta = screen.getByRole("region", { name: /begin where you are/i });
    expect(
      within(cta).getByRole("link", { name: /book a session/i }).getAttribute("href"),
    ).toBe("/work-with-me");
  });
});
