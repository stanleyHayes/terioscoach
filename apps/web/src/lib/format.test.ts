import { describe, expect, it } from "vitest";
import { formatDuration, formatMoney } from "./format";

describe("formatMoney", () => {
  it("formats GHS minor units with the cedi symbol", () => {
    expect(formatMoney(45000, "GHS")).toBe("GH₵450.00");
  });

  it("defaults to GHS when no currency is given", () => {
    expect(formatMoney(45000)).toBe("GH₵450.00");
  });

  it("groups thousands", () => {
    expect(formatMoney(120000, "GHS")).toBe("GH₵1,200.00");
  });

  it("handles zero", () => {
    expect(formatMoney(0, "GHS")).toBe("GH₵0.00");
  });

  it("formats other ISO 4217 currencies with their narrow symbol", () => {
    expect(formatMoney(123450, "USD")).toBe("$1,234.50");
  });

  it("falls back to code + fixed decimals for an invalid currency code", () => {
    expect(formatMoney(45000, "NOTACODE")).toBe("NOTACODE 450.00");
  });
});

describe("formatDuration", () => {
  it("keeps sub-hour durations in minutes", () => {
    expect(formatDuration(45)).toBe("45 min");
    expect(formatDuration(5)).toBe("5 min");
  });

  it("uses whole hours when there is no remainder", () => {
    expect(formatDuration(60)).toBe("1 h");
    expect(formatDuration(120)).toBe("2 h");
  });

  it("combines hours and remaining minutes", () => {
    expect(formatDuration(90)).toBe("1 h 30 min");
    expect(formatDuration(150)).toBe("2 h 30 min");
  });
});
