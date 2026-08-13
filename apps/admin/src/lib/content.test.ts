import { describe, expect, it } from "vitest";
import { slugify } from "@/lib/content";

/**
 * The editor shows the URL a title will produce before saving, so this has
 * to agree with the server's own rule. Where it disagrees, the practitioner
 * is shown a link that doesn't exist.
 */
describe("slugify", () => {
  it("lowercases and joins words with hyphens", () => {
    expect(slugify("Our Approach to Rest")).toBe("our-approach-to-rest");
  });

  it("drops punctuation rather than encoding it", () => {
    expect(slugify("What's included?")).toBe("whats-included");
    expect(slugify("Rest, properly.")).toBe("rest-properly");
  });

  it("never produces a leading, trailing or doubled hyphen", () => {
    expect(slugify("  spaced  out  ")).toBe("spaced-out");
    expect(slugify("--edges--")).toBe("edges");
    expect(slugify("a // b")).toBe("a-b");
  });

  it("transliterates accents instead of stripping the letter", () => {
    // "Café" losing its last letter entirely would be worse than "cafe".
    expect(slugify("Café Sessions")).toBe("cafe-sessions");
    expect(slugify("Größe")).toBe("grosse");
  });

  it("is idempotent, so re-slugging an existing slug leaves it alone", () => {
    const once = slugify("Five Ways to Rest — Properly");
    expect(slugify(once)).toBe(once);
  });

  it("returns an empty string when there is nothing usable", () => {
    expect(slugify("!!!")).toBe("");
    expect(slugify("")).toBe("");
  });
});
