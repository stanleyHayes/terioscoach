import { describe, expect, it } from "vitest";
import { wallClockCandidates, wallClockToUtcIso } from "./schedule";
describe("DST wall-clock selection", () => {
  it("rejects the spring-forward gap", () => {
    expect(
      wallClockCandidates("2026-03-08", "02:30", "America/New_York"),
    ).toEqual([]);
    expect(
      wallClockToUtcIso("2026-03-08", "02:30", "America/New_York"),
    ).toBeNull();
  });
  it("requires an explicit choice for a repeated wall time", () => {
    expect(
      wallClockCandidates("2026-11-01", "01:30", "America/New_York"),
    ).toEqual(["2026-11-01T05:30:00.000Z", "2026-11-01T06:30:00.000Z"]);
    expect(
      wallClockToUtcIso("2026-11-01", "01:30", "America/New_York"),
    ).toBeNull();
  });
  it("converts midnight across dates and keeps ordinary wall times unique", () => {
    expect(wallClockToUtcIso("2026-09-23", "00:00", "Pacific/Honolulu")).toBe(
      "2026-09-23T10:00:00.000Z",
    );
    expect(wallClockToUtcIso("2026-09-23", "00:00", "Pacific/Auckland")).toBe(
      "2026-09-22T12:00:00.000Z",
    );
  });
});
