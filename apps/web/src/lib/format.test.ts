import { describe, expect, it } from "vitest";
import {
  browserTimeZone,
  formatDuration,
  formatMoney,
  formatSessionDate,
  formatTimeOfDay,
  formatTimeRange,
  gmtOffsetLabel,
} from "./format";

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

describe("formatTimeOfDay", () => {
  it("formats a UTC instant as wall-clock time in the given zone", () => {
    expect(formatTimeOfDay("2026-08-20T09:30:00Z", "UTC")).toBe("9:30 AM");
    expect(formatTimeOfDay("2026-08-20T15:30:00Z", "UTC")).toBe("3:30 PM");
  });

  it("shifts across zones", () => {
    // 09:30 UTC is 05:30 in New York (EDT, UTC-4 in August).
    expect(formatTimeOfDay("2026-08-20T09:30:00Z", "America/New_York")).toBe("5:30 AM");
  });
});

describe("formatTimeRange", () => {
  it("joins start and end with an en dash", () => {
    expect(formatTimeRange("2026-08-20T09:30:00Z", "2026-08-20T10:15:00Z", "UTC")).toBe(
      "9:30 AM – 10:15 AM",
    );
  });
});

describe("formatSessionDate", () => {
  it("formats a UTC instant as a calendar date in the given zone", () => {
    expect(formatSessionDate("2026-08-12T09:30:00Z", "UTC")).toBe("Wed, Aug 12, 2026");
  });

  it("respects the zone's calendar day (not the UTC day)", () => {
    // 23:30 UTC on Aug 12 is already Aug 13 in Accra? No — Accra is UTC+0;
    // use a zone ahead of UTC: 23:30 UTC Aug 12 → Aug 13 09:30 in Sydney (AEST, UTC+10).
    expect(formatSessionDate("2026-08-12T23:30:00Z", "Australia/Sydney")).toBe("Thu, Aug 13, 2026");
  });
});

describe("gmtOffsetLabel", () => {
  it("normalizes zero offset to an explicit GMT+0", () => {
    expect(gmtOffsetLabel("UTC")).toBe("GMT+0");
    expect(gmtOffsetLabel("Africa/Accra")).toBe("GMT+0");
  });

  it("states DST-aware offsets for the given instant", () => {
    expect(gmtOffsetLabel("America/New_York", new Date("2026-08-12T12:00:00Z"))).toBe("GMT-4");
    expect(gmtOffsetLabel("America/New_York", new Date("2026-01-12T12:00:00Z"))).toBe("GMT-5");
  });

  it("includes minutes for half-hour zones", () => {
    expect(gmtOffsetLabel("Asia/Kolkata")).toBe("GMT+5:30");
  });

  it("falls back to the raw zone name for an invalid zone", () => {
    expect(gmtOffsetLabel("Not/AZone")).toBe("Not/AZone");
  });
});

describe("browserTimeZone", () => {
  it("returns an IANA name", () => {
    expect(browserTimeZone()).toMatch(/^[A-Za-z_]+\/[A-Za-z_]+|UTC$/);
  });
});
