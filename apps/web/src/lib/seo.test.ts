import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

/** seo.ts reads env at module load, so each case needs a fresh import. */
async function loadSeo() {
  vi.resetModules();
  return import("./seo");
}

const original = { ...process.env };

beforeEach(() => {
  vi.resetModules();
});

afterEach(() => {
  vi.unstubAllEnvs();
  process.env = { ...original };
});

describe("SITE_URL", () => {
  it("takes the configured origin", async () => {
    process.env.NEXT_PUBLIC_SITE_URL = "https://staging.terioscoach.com";
    const { SITE_URL } = await loadSeo();
    expect(SITE_URL).toBe("https://staging.terioscoach.com");
  });

  it("strips a trailing slash so joined paths never double up", async () => {
    process.env.NEXT_PUBLIC_SITE_URL = "https://terioscoach.com/";
    const { SITE_URL, absoluteUrl } = await loadSeo();
    expect(SITE_URL).toBe("https://terioscoach.com");
    expect(absoluteUrl("/blog")).toBe("https://terioscoach.com/blog");
  });

  it("joins a path that is missing its leading slash", async () => {
    process.env.NEXT_PUBLIC_SITE_URL = "https://terioscoach.com";
    const { absoluteUrl } = await loadSeo();
    expect(absoluteUrl("blog")).toBe("https://terioscoach.com/blog");
  });
});

describe("isIndexable", () => {
  it("is true on the production origin", async () => {
    process.env.VERCEL_ENV = "production";
    const { isIndexable } = await loadSeo();
    expect(isIndexable()).toBe(true);
  });

  it("is false on a preview deployment", async () => {
    // A preview that gets indexed competes with the real site in search
    // results — the whole reason this check exists.
    process.env.VERCEL_ENV = "preview";
    vi.stubEnv("NODE_ENV", "production");
    delete process.env.NEXT_PUBLIC_ALLOW_INDEXING;
    const { isIndexable } = await loadSeo();
    expect(isIndexable()).toBe(false);
  });

  it("is false in development", async () => {
    delete process.env.VERCEL_ENV;
    delete process.env.NEXT_PUBLIC_ALLOW_INDEXING;
    vi.stubEnv("NODE_ENV", "development");
    const { isIndexable } = await loadSeo();
    expect(isIndexable()).toBe(false);
  });

  it("can be forced on for a deliberately indexable environment", async () => {
    process.env.VERCEL_ENV = "preview";
    process.env.NEXT_PUBLIC_ALLOW_INDEXING = "true";
    const { isIndexable } = await loadSeo();
    expect(isIndexable()).toBe(true);
  });
});
