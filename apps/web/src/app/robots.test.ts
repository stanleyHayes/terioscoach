import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

/** robots.ts and seo.ts both read env at load, so each case re-imports. */
async function loadRobots() {
  vi.resetModules();
  const loaded = await import("./robots");
  return loaded.default();
}

const original = { ...process.env };

beforeEach(() => {
  vi.resetModules();
  process.env.NEXT_PUBLIC_SITE_URL = "https://terioswellness.com";
});

afterEach(() => {
  process.env = { ...original };
});

describe("robots", () => {
  it("keeps crawlers out of the client portal", async () => {
    process.env.VERCEL_ENV = "production";

    const robots = await loadRobots();
    const rules = Array.isArray(robots.rules) ? robots.rules[0] : robots.rules;

    // The portal is behind authentication: a crawler can only ever reach a
    // login page there, and those URLs in an index help nobody.
    //
    // The list is PRIVATE_PATHS, shared with Analytics so the two cannot
    // drift — which they had, leaving the recovery routes measured. That
    // is why /portal has lost its trailing slash: robots.txt matches on
    // prefix, so "/portal" already covers everything under it, and the
    // shared list has to be usable as a pathname test too.
    expect(rules.disallow).toEqual([
      "/portal",
      "/login",
      "/register",
      "/forgot-password",
      "/reset-password",
    ]);
    expect(rules.allow).toBe("/");
    expect(robots.sitemap).toBe("https://terioswellness.com/sitemap.xml");
  });

  it("refuses everything on a preview deployment", async () => {
    process.env.VERCEL_ENV = "preview";
    delete process.env.NEXT_PUBLIC_ALLOW_INDEXING;

    const robots = await loadRobots();
    const rules = Array.isArray(robots.rules) ? robots.rules[0] : robots.rules;

    expect(rules.disallow).toBe("/");
    expect(rules.allow).toBeUndefined();
    // No sitemap either: nothing here should be crawled at all.
    expect(robots.sitemap).toBeUndefined();
  });
});
