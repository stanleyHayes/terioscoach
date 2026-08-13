import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Post } from "@/lib/content";
import BlogPage from "./page";

const listPosts = vi.hoisted(() => vi.fn());
vi.mock("@/lib/content", () => ({ listPosts }));

function post(overrides: Partial<Post> = {}): Post {
  return {
    id: "post-1",
    slug: "resting-well",
    title: "Resting well",
    excerpt: "What recovery actually asks of you.",
    category: "Recovery",
    tags: [],
    publishedAt: "2026-08-03T09:00:00Z",
    ...overrides,
  };
}

/** Renders the async server component. */
async function renderPage(searchParams: { category?: string; tag?: string } = {}) {
  render(await BlogPage({ searchParams: Promise.resolve(searchParams) }));
}

describe("BlogPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listPosts.mockResolvedValue([post()]);
  });

  it("lists published posts with their date and category", async () => {
    await renderPage();

    expect(screen.getByRole("link", { name: "Resting well" }).getAttribute("href")).toBe(
      "/blog/resting-well",
    );
    expect(screen.getByText("Recovery")).toBeTruthy();
    expect(screen.getByText(/what recovery actually asks of you/i)).toBeTruthy();
  });

  it("passes the category and tag filters to the API", async () => {
    await renderPage({ category: "Recovery", tag: "rest" });

    expect(listPosts).toHaveBeenCalledWith({ category: "Recovery", tag: "rest" });
    // A filtered view says so, and offers the way back.
    expect(screen.getByRole("link", { name: /show everything/i })).toBeTruthy();
  });

  it("shows a calm empty state when nothing is published", async () => {
    listPosts.mockResolvedValue([]);

    await renderPage();

    expect(screen.getByRole("heading", { name: /the first note is on its way/i })).toBeTruthy();
  });

  it("points back to the full journal when a filter finds nothing", async () => {
    listPosts.mockResolvedValue([]);

    await renderPage({ tag: "rest" });

    expect(screen.getByRole("heading", { name: /nothing under that heading yet/i })).toBeTruthy();
    expect(screen.getByRole("link", { name: /read everything/i })).toBeTruthy();
  });

  it("renders a branded error instead of crashing when the API is down", async () => {
    listPosts.mockRejectedValue(new Error("network"));

    await renderPage();

    const alert = screen.getByRole("alert");
    expect(alert.textContent).toMatch(/didn’t load/i);
    expect(screen.getByRole("link", { name: /try again/i })).toBeTruthy();
  });
});
