import { beforeEach, describe, expect, it, vi } from "vitest";
import type { RefreshCallbacks, Session } from "@/lib/api";
import {
  currentMonthRange,
  paymentsApi,
  peakIncome,
  reportsApi,
  shiftMonthRange,
  type PeriodIncome,
} from "./insights";

const authedRequestMock = vi.fn();

vi.mock("@/lib/api", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...original,
    authedRequest: (...args: unknown[]) => authedRequestMock(...args),
  };
});

const session: Session = { accessToken: "access", refreshToken: "refresh" };
const callbacks: RefreshCallbacks = { onTokensRefreshed: vi.fn() };

function lastCall(): [string, Session, RefreshCallbacks, Record<string, unknown>?] {
  return authedRequestMock.mock.calls.at(-1) as never;
}

beforeEach(() => {
  authedRequestMock.mockReset();
});

describe("paymentsApi", () => {
  it("asks for everything when no range is given", async () => {
    authedRequestMock.mockResolvedValue({ items: [] });

    await paymentsApi.list(session, callbacks);

    expect(lastCall()[0]).toBe("/v1/payments");
  });

  it.each([
    [{ from: "2026-08-01T00:00:00Z" }, "?from=2026-08-01T00%3A00%3A00Z"],
    [{ to: "2026-09-01T00:00:00Z" }, "?to=2026-09-01T00%3A00%3A00Z"],
    [
      { from: "2026-08-01T00:00:00Z", to: "2026-09-01T00:00:00Z" },
      "?from=2026-08-01T00%3A00%3A00Z&to=2026-09-01T00%3A00%3A00Z",
    ],
  ])("passes %o through as a query string", async (range, expected) => {
    authedRequestMock.mockResolvedValue({ items: [] });

    await paymentsApi.list(session, callbacks, range);

    // Timestamps carry colons; unencoded they would arrive as a different
    // value than the one asked for.
    expect(lastCall()[0]).toBe(`/v1/payments${expected}`);
  });

  it("refunds through POST and returns the updated payment", async () => {
    authedRequestMock.mockResolvedValue({ payment: { id: "pay-1", status: "refunded" } });

    const result = await paymentsApi.refund(session, callbacks, "pay-1");

    expect(result.status).toBe("refunded");
    const [path, , , options] = lastCall();
    expect(path).toBe("/v1/payments/pay-1/refund");
    expect(options).toMatchObject({ method: "POST" });
  });
});

describe("reportsApi", () => {
  it("asks for the default window when no range is given", async () => {
    authedRequestMock.mockResolvedValue({});

    await reportsApi.practice(session, callbacks);

    expect(lastCall()[0]).toBe("/v1/admin/reports/practice");
  });

  it("carries from, to and granularity", async () => {
    authedRequestMock.mockResolvedValue({});

    await reportsApi.practice(session, callbacks, {
      from: "2026-08-01T00:00:00Z",
      to: "2026-09-01T00:00:00Z",
      granularity: "week",
    });

    expect(lastCall()[0]).toBe(
      "/v1/admin/reports/practice?from=2026-08-01T00%3A00%3A00Z&to=2026-09-01T00%3A00%3A00Z&granularity=week",
    );
  });

  it("defaults the upcoming-load window to a fortnight", async () => {
    authedRequestMock.mockResolvedValue({ items: [{ date: "2026-08-11", sessions: 2 }] });

    const result = await reportsApi.upcomingLoad(session, callbacks);

    expect(result).toEqual([{ date: "2026-08-11", sessions: 2 }]);
    expect(lastCall()[0]).toBe("/v1/admin/reports/upcoming-load?days=14");
  });

  it("takes an explicit upcoming-load window", async () => {
    authedRequestMock.mockResolvedValue({ items: [] });

    await reportsApi.upcomingLoad(session, callbacks, 30);

    expect(lastCall()[0]).toBe("/v1/admin/reports/upcoming-load?days=30");
  });
});

describe("currentMonthRange", () => {
  it("is half-open, so adjacent months add up to no more than the year", () => {
    const august = currentMonthRange(new Date("2026-08-17T13:45:00.000Z"));

    expect(august.from).toBe("2026-08-01T00:00:00.000Z");
    // The 1st of September, not the 31st of August: a closed window either
    // double-counts the boundary day or drops it.
    expect(august.to).toBe("2026-09-01T00:00:00.000Z");
  });

  it("rolls the year over in December", () => {
    const december = currentMonthRange(new Date("2026-12-31T23:59:59.000Z"));

    expect(december.from).toBe("2026-12-01T00:00:00.000Z");
    expect(december.to).toBe("2027-01-01T00:00:00.000Z");
  });
});

describe("shiftMonthRange", () => {
  it("steps back a month", () => {
    const range = { from: "2026-08-01T00:00:00.000Z", to: "2026-09-01T00:00:00.000Z" };

    expect(shiftMonthRange(range, -1)).toEqual({
      from: "2026-07-01T00:00:00.000Z",
      to: "2026-08-01T00:00:00.000Z",
    });
  });

  it("steps forward across a year boundary", () => {
    const range = { from: "2026-12-01T00:00:00.000Z", to: "2027-01-01T00:00:00.000Z" };

    expect(shiftMonthRange(range, 1)).toEqual({
      from: "2027-01-01T00:00:00.000Z",
      to: "2027-02-01T00:00:00.000Z",
    });
  });

  it("lands on a real month end when stepping back from a 31-day month", () => {
    // February has no 31st. Anchoring on the 1st is what keeps this from
    // sliding into March.
    const range = { from: "2026-03-01T00:00:00.000Z", to: "2026-04-01T00:00:00.000Z" };

    expect(shiftMonthRange(range, -1)).toEqual({
      from: "2026-02-01T00:00:00.000Z",
      to: "2026-03-01T00:00:00.000Z",
    });
  });

  it("stays put on zero", () => {
    const range = { from: "2026-08-01T00:00:00.000Z", to: "2026-09-01T00:00:00.000Z" };

    expect(shiftMonthRange(range, 0)).toEqual(range);
  });
});

describe("peakIncome", () => {
  function bucket(incomeKobo: number): PeriodIncome {
    return { start: "2026-08-01T00:00:00.000Z", sessions: 1, incomeKobo };
  }

  it("is the tallest bar", () => {
    expect(peakIncome([bucket(1000), bucket(45000), bucket(200)])).toBe(45000);
  });

  it("never returns zero, so the chart cannot divide by it", () => {
    // A month with no income is a real month. Scaling by zero would render
    // every bar as NaN% tall.
    expect(peakIncome([bucket(0), bucket(0)])).toBe(1);
    expect(peakIncome([])).toBe(1);
  });
});
