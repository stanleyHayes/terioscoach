import { beforeEach, describe, expect, it, vi } from "vitest";
import { getSiteCopy } from "./site-copy";
import { websiteContent, siteCopy, validContentUrl, parseSiteValues } from "../../../../shared/site-content";
const getPage = vi.hoisted(() => vi.fn());
vi.mock("./content", () => ({ getPage }));
beforeEach(() => { getPage.mockReset(); });
describe("website content", () => {
  it("uses published overrides ahead of legacy content without caching", async () => {
    getPage.mockImplementation(async (slug: string) => slug === "home" ? { body: "Legacy lead" } : { body: JSON.stringify({ "hero-description": "New lead", "field-017": "https://cdn.example/photo.webp" }) });
    const copy = await getSiteCopy("home");
    expect(copy("hero-description", "Default")).toBe("New lead");
    expect(copy("field-017", "/default.webp")).toBe("https://cdn.example/photo.webp");
    getPage.mockResolvedValue({ body: JSON.stringify({ "hero-description": "Latest lead" }) });
    expect((await getSiteCopy("home"))("hero-description", "Default")).toBe("Latest lead");
  });
  it("preserves defaults and intentional empty text, rejecting unsafe links", async () => {
    getPage.mockRejectedValue(new Error("Missing"));
    expect((await getSiteCopy("home"))("hero-description", "Default")).toBe("Default");
    expect(siteCopy("home", { "hero-description": "" })("hero-description", "Default")).toBe("");
    expect(validContentUrl("javascript:alert(1)")).toBe(false);
    expect(validContentUrl("//evil.example")).toBe(false);
    expect(validContentUrl("https://cdn.example/image.webp", true)).toBe(true);
    expect(parseSiteValues('{"a":"text","b":false}')).toEqual({ a: "text" });
  });
  it("covers every public page and keeps field keys unique", () => {
    for (const scope of ["home", "about", "work-with-me", "services", "blog", "faq", "contact", "privacy", "terms", "header", "footer"]) {
      const fields = websiteContent[scope].fields;
      expect(fields.length).toBeGreaterThan(0);
      expect(new Set(fields.map(field => field.key)).size).toBe(fields.length);
    }
  });
});
