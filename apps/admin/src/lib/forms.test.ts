import { describe, expect, it } from "vitest";
import { fieldKey, fieldProblem, uniqueFieldKey, type FormField } from "@/lib/forms";

function field(overrides: Partial<FormField> = {}): FormField {
  return { key: "k", label: "Question", type: "text", required: false, options: [], ...overrides };
}

/**
 * These mirror `form.NormalizeKey` in the Go domain. The builder shows the
 * key it expects the answer to be stored under; if the two rules disagree,
 * the builder is lying about where the answer lives.
 */
describe("fieldKey", () => {
  it("lowercases and joins words with underscores", () => {
    expect(fieldKey("Any allergies")).toBe("any_allergies");
    expect(fieldKey("Date of birth")).toBe("date_of_birth");
  });

  it("treats hyphens and underscores as separators, like the server does", () => {
    expect(fieldKey("Day-to-day pain")).toBe("day_to_day_pain");
    expect(fieldKey("already_a_key")).toBe("already_a_key");
  });

  it("drops anything else without leaving a separator behind", () => {
    // The server's rule exactly: '?' and '/' are dropped, not converted.
    expect(fieldKey("Any allergies?")).toBe("any_allergies");
    expect(fieldKey("Name/Address")).toBe("nameaddress");
    expect(fieldKey("e.g. Age")).toBe("eg_age");
  });

  it("never leaves a leading, trailing or doubled underscore", () => {
    expect(fieldKey("  spaced  out  ")).toBe("spaced_out");
    expect(fieldKey("--edges--")).toBe("edges");
  });

  it("returns an empty string when nothing usable is left", () => {
    expect(fieldKey("???")).toBe("");
  });
});

describe("uniqueFieldKey", () => {
  it("uses the derived key when it is free", () => {
    expect(uniqueFieldKey("Any allergies", ["notes"])).toBe("any_allergies");
  });

  it("suffixes rather than colliding — the server rejects duplicates", () => {
    expect(uniqueFieldKey("Notes", ["notes"])).toBe("notes_2");
    expect(uniqueFieldKey("Notes", ["notes", "notes_2"])).toBe("notes_3");
  });

  it("falls back to a usable key when the label yields nothing", () => {
    expect(uniqueFieldKey("???", [])).toBe("field");
    expect(uniqueFieldKey("???", ["field"])).toBe("field_2");
  });
});

describe("fieldProblem", () => {
  it("passes a complete field", () => {
    expect(fieldProblem(field())).toBeNull();
    expect(fieldProblem(field({ type: "radio", options: ["Yes", "No"] }))).toBeNull();
  });

  it("requires a label", () => {
    expect(fieldProblem(field({ label: "   " }))).toMatch(/needs a label/);
  });

  it("requires a choice field to have at least one real option", () => {
    for (const type of ["select", "radio", "checkbox"] as const) {
      expect(fieldProblem(field({ type, options: [] }))).toMatch(/at least one option/);
      // Blank boxes are dropped on save, so they do not count as options.
      expect(fieldProblem(field({ type, options: ["", "  "] }))).toMatch(/at least one option/);
      expect(fieldProblem(field({ type, options: ["", "Yes"] }))).toBeNull();
    }
  });

  it("does not demand options from a field that cannot have them", () => {
    expect(fieldProblem(field({ type: "signature" }))).toBeNull();
    expect(fieldProblem(field({ type: "date" }))).toBeNull();
  });
});
