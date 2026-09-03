import { describe, expect, it } from "vitest";
import {
  formatDuration,
  formatMoney,
  minorToMajorString,
  parseMajorToMinor,
} from "./format";

describe("formatMoney", () => {
	it("formats minor units as USD major units", () => {
	  expect(formatMoney(123450)).toBe("$1,234.50");
	  expect(formatMoney(0)).toBe("$0.00");
	  expect(formatMoney(25000)).toBe("$250.00");
	  expect(formatMoney(5)).toBe("$0.05");
  });

  it("honours other ISO currencies", () => {
    expect(formatMoney(9900, "USD")).toBe("$99.00");
  });
});

describe("minorToMajorString", () => {
  it("rounds to a plain major-units string for editing", () => {
    expect(minorToMajorString(25000)).toBe("250");
    expect(minorToMajorString(25050)).toBe("250.5");
    expect(minorToMajorString(25005)).toBe("250.05");
    expect(minorToMajorString(0)).toBe("0");
  });
});

describe("parseMajorToMinor", () => {
  it("converts valid major-units input to integer minor units", () => {
    expect(parseMajorToMinor("250")).toBe(25000);
    expect(parseMajorToMinor("250.5")).toBe(25050);
    expect(parseMajorToMinor("250.50")).toBe(25050);
    expect(parseMajorToMinor(" 250.50 ")).toBe(25050);
    expect(parseMajorToMinor("1,250.00")).toBe(125000);
    expect(parseMajorToMinor("0")).toBe(0);
  });

  it("rejects invalid input", () => {
    expect(parseMajorToMinor("")).toBeNull();
    expect(parseMajorToMinor("abc")).toBeNull();
    expect(parseMajorToMinor("-10")).toBeNull();
    expect(parseMajorToMinor("10.999")).toBeNull();
    expect(parseMajorToMinor("10.")).toBeNull();
    expect(parseMajorToMinor(".50")).toBeNull();
  });
});

describe("formatDuration", () => {
  it("formats sub-hour durations in minutes", () => {
    expect(formatDuration(5)).toBe("5 min");
    expect(formatDuration(45)).toBe("45 min");
  });

  it("formats whole and partial hours", () => {
    expect(formatDuration(60)).toBe("1 hr");
    expect(formatDuration(90)).toBe("1 hr 30 min");
    expect(formatDuration(480)).toBe("8 hr");
  });
});
