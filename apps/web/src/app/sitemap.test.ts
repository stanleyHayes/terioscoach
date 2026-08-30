import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const listPosts = vi.hoisted(() => vi.fn());
vi.mock("@/lib/content", () => ({ listPosts }));

async function loadSitemap() {
  vi.resetModules();
  const loaded = await import("./sitemap");
  return loaded.default();
}

const original = { ...process.env };

beforeEach(() => {
  vi.clearAllMocks();
  process.env.NEXT_PUBLIC_SITE_URL = "https://terioscoach.com";
  listPosts.mockResolvedValue([]);
});

afterEach(() => {
  process.env = { ...original };
});

describe("sitemap", () => {
  it("lists the public routes as absolute URLs", async () => {
    const entries = await loadSitemap();
    const urls = entries.map((entry) => entry.url);

    expect(urls).toContain("https://terioscoach.com/");
    expect(urls).toContain("https://terioscoach.com/services");
    expect(urls).toContain("https://terioscoach.com/blog");
    expect(urls).toContain("https://terioscoach.com/faq");
    expect(urls).toContain("https://terioscoach.com/contact");
  });

  it("leaves the authenticated portal out entirely", async () => {
    const entries = await loadSitemap();
    const urls = entries.map((entry) => entry.url);

    for (const path of ["/portal", "/portal/", "/login", "/register"]) {
      expect(urls.some((url) => url.endsWith(path))).toBe(false);
    }
  });

  it("includes published articles with their publication date", async () => {
    listPosts.mockResolvedValue([
      { id: "1", slug: "resting-well", title: "Resting well", tags: [], publishedAt: "2026-08-03T09:00:00Z" },
    ]);

    const entries = await loadSitemap();
    const article = entries.find((entry) => entry.url.endsWith("/blog/resting-well"));

    expect(article).toBeDefined();
    expect((article!.lastModified as Date).toISOString()).toBe("2026-08-03T09:00:00.000Z");
  });

  it("still returns the static routes when the content API is down", async () => {
    listPosts.mockRejectedValue(new Error("network"));

    const entries = await loadSitemap();

    // Losing the article list should not cost the crawler the whole sitemap.
    expect(entries.length).toBeGreaterThan(0);
    expect(entries.map((e) => e.url)).toContain("https://terioscoach.com/");
  });
});
