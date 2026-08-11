import { beforeEach, describe, expect, it, vi } from "vitest";
import type { RefreshCallbacks, Session } from "@/lib/api";
import {
  addDaysCivil,
  dateKey,
  formatCivilDate,
  formatTime,
  formatWeekRange,
  minutesToTimeString,
  mondayOfWeek,
  parseDateInput,
  parseTimeInput,
  scheduleApi,
  todayCivil,
  wallClockToUtcIso,
  weekdayShortName,
  zonedParts,
  type AvailabilityRule,
  type Booking,
} from "./schedule";

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

function booking(overrides: Partial<Booking> = {}): Booking {
  return {
    id: "bk-1",
    clientId: "client-1",
    practitionerId: "prac-1",
    serviceId: "svc-1",
    startAt: "2026-08-11T09:00:00.000Z",
    endAt: "2026-08-11T10:00:00.000Z",
    status: "confirmed",
    createdAt: "2026-08-01T10:00:00.000Z",
    updatedAt: "2026-08-01T10:00:00.000Z",
    ...overrides,
  };
}

beforeEach(() => {
  authedRequestMock.mockReset();
});

describe("scheduleApi", () => {
  it("getRules GETs /v1/availability/rules and unwraps rules", async () => {
    const rules: AvailabilityRule[] = [
      { weekday: 1, windows: [{ startMin: 540, endMin: 1020 }], bufferMinutes: 15 },
    ];
    authedRequestMock.mockResolvedValueOnce({ rules });

    await expect(scheduleApi.getRules(session, callbacks)).resolves.toEqual(rules);
    expect(authedRequestMock).toHaveBeenCalledWith(
      "/v1/availability/rules",
      session,
      callbacks,
    );
  });

  it("putRules PUTs the full replacement set and unwraps rules", async () => {
    const rules: AvailabilityRule[] = [
      {
        weekday: 2,
        windows: [
          { startMin: 540, endMin: 720 },
          { startMin: 780, endMin: 1020 },
        ],
        bufferMinutes: 0,
      },
    ];
    authedRequestMock.mockResolvedValueOnce({ rules });

    await expect(scheduleApi.putRules(session, callbacks, rules)).resolves.toEqual(rules);
    expect(authedRequestMock).toHaveBeenCalledWith(
      "/v1/availability/rules",
      session,
      callbacks,
      { method: "PUT", body: { rules } },
    );
  });

  it("addTimeOff POSTs the draft and unwraps timeOff", async () => {
    const timeOff = {
      id: "to-1",
      practitionerId: "prac-1",
      startAt: "2026-09-01T00:00:00.000Z",
      endAt: "2026-09-04T00:00:00.000Z",
      reason: "Family trip",
      createdAt: "2026-08-01T10:00:00.000Z",
    };
    authedRequestMock.mockResolvedValueOnce({ timeOff });
    const draft = {
      startAt: timeOff.startAt,
      endAt: timeOff.endAt,
      reason: "Family trip",
    };

    await expect(scheduleApi.addTimeOff(session, callbacks, draft)).resolves.toEqual(
      timeOff,
    );
    expect(authedRequestMock).toHaveBeenCalledWith(
      "/v1/availability/time-off",
      session,
      callbacks,
      { method: "POST", body: draft },
    );
  });

  it("listBookings builds the from/to/status query and unwraps items", async () => {
    const items = [booking()];
    authedRequestMock.mockResolvedValueOnce({ items });

    await expect(
      scheduleApi.listBookings(session, callbacks, {
        from: "2026-08-10T00:00:00.000Z",
        to: "2026-08-17T00:00:00.000Z",
        status: "confirmed",
      }),
    ).resolves.toEqual(items);
    expect(authedRequestMock).toHaveBeenCalledWith(
      `/v1/bookings?${new URLSearchParams({
        from: "2026-08-10T00:00:00.000Z",
        to: "2026-08-17T00:00:00.000Z",
        status: "confirmed",
      }).toString()}`,
      session,
      callbacks,
    );
  });

  it("listBookings omits optional params when absent", async () => {
    authedRequestMock.mockResolvedValueOnce({ items: [] });

    await scheduleApi.listBookings(session, callbacks);
    expect(authedRequestMock).toHaveBeenCalledWith("/v1/bookings", session, callbacks);
  });

  it("rescheduleBooking POSTs startAt + tz and unwraps booking", async () => {
    const updated = booking({ startAt: "2026-08-12T14:00:00.000Z" });
    authedRequestMock.mockResolvedValueOnce({ booking: updated });

    await expect(
      scheduleApi.rescheduleBooking(
        session,
        callbacks,
        "bk-1",
        "2026-08-12T14:00:00.000Z",
      ),
    ).resolves.toEqual(updated);
    expect(authedRequestMock).toHaveBeenCalledWith(
      "/v1/bookings/bk-1/reschedule",
      session,
      callbacks,
      {
        method: "POST",
        body: { startAt: "2026-08-12T14:00:00.000Z", tz: "Africa/Accra" },
      },
    );
  });

  it.each([
    ["cancelBooking", "cancel"],
    ["completeBooking", "complete"],
    ["noShowBooking", "no-show"],
  ] as const)("%s POSTs /v1/bookings/{id}/%s with no body", async (method, route) => {
    const updated = booking();
    authedRequestMock.mockResolvedValueOnce({ booking: updated });

    await expect(
      scheduleApi[method](session, callbacks, "bk-1"),
    ).resolves.toEqual(updated);
    expect(authedRequestMock).toHaveBeenCalledWith(
      `/v1/bookings/bk-1/${route}`,
      session,
      callbacks,
      { method: "POST" },
    );
  });
});

describe("civil-date helpers", () => {
  it("mondayOfWeek returns the Monday of the containing week", () => {
    // 2026-08-11 is a Tuesday; 2026-08-16 a Sunday — both map to Aug 10.
    expect(mondayOfWeek({ year: 2026, month: 8, day: 11 })).toEqual({
      year: 2026,
      month: 8,
      day: 10,
    });
    expect(mondayOfWeek({ year: 2026, month: 8, day: 16 })).toEqual({
      year: 2026,
      month: 8,
      day: 10,
    });
    expect(mondayOfWeek({ year: 2026, month: 8, day: 10 })).toEqual({
      year: 2026,
      month: 8,
      day: 10,
    });
  });

  it("addDaysCivil crosses month and year boundaries", () => {
    expect(addDaysCivil({ year: 2026, month: 8, day: 31 }, 1)).toEqual({
      year: 2026,
      month: 9,
      day: 1,
    });
    expect(addDaysCivil({ year: 2026, month: 1, day: 1 }, -1)).toEqual({
      year: 2025,
      month: 12,
      day: 31,
    });
  });

  it("dateKey zero-pads to YYYY-MM-DD", () => {
    expect(dateKey({ year: 2026, month: 8, day: 4 })).toBe("2026-08-04");
  });

  it("zonedParts reports wall-clock fields in the practice zone", () => {
    const parts = zonedParts("2026-08-11T15:30:00.000Z", "Africa/Accra");
    expect(parts).toMatchObject({
      year: 2026,
      month: 8,
      day: 11,
      hour: 15,
      minute: 30,
      minutesSinceMidnight: 930,
    });
  });

  it("todayCivil derives the date in the zone, not the browser zone", () => {
    expect(todayCivil("Africa/Accra", new Date("2026-08-11T23:30:00.000Z"))).toEqual({
      year: 2026,
      month: 8,
      day: 11,
    });
    expect(todayCivil("Pacific/Auckland", new Date("2026-08-11T23:30:00.000Z"))).toEqual(
      { year: 2026, month: 8, day: 12 },
    );
  });

  it("wallClockToUtcIso converts zone wall clock to a UTC instant", () => {
    expect(wallClockToUtcIso("2026-08-11", "09:30", "Africa/Accra")).toBe(
      "2026-08-11T09:30:00.000Z",
    );
    // America/New_York is UTC-4 in August (DST-safe via Intl offset).
    expect(wallClockToUtcIso("2026-08-11", "09:30", "America/New_York")).toBe(
      "2026-08-11T13:30:00.000Z",
    );
    expect(wallClockToUtcIso("not-a-date", "09:30", "Africa/Accra")).toBeNull();
    expect(wallClockToUtcIso("2026-08-11", "25:30", "Africa/Accra")).toBeNull();
  });
});

describe("input parsing + formatting", () => {
  it("parseDateInput accepts real YYYY-MM-DD dates only", () => {
    expect(parseDateInput("2026-02-28")).toEqual({ year: 2026, month: 2, day: 28 });
    expect(parseDateInput("2026-02-30")).toBeNull();
    expect(parseDateInput("08/11/2026")).toBeNull();
    expect(parseDateInput("2026-8-1")).toBeNull();
  });

  it("parseTimeInput accepts H:MM and HH:MM within 00:00–23:59", () => {
    expect(parseTimeInput("9:05")).toBe(545);
    expect(parseTimeInput("23:59")).toBe(1439);
    expect(parseTimeInput("24:00")).toBeNull();
    expect(parseTimeInput("9:60")).toBeNull();
    expect(parseTimeInput("9")).toBeNull();
  });

  it("minutesToTimeString zero-pads", () => {
    expect(minutesToTimeString(545)).toBe("09:05");
    expect(minutesToTimeString(0)).toBe("00:00");
  });

  it("formatTime renders the wall clock in the practice zone", () => {
    expect(formatTime("2026-08-11T15:30:00.000Z", "Africa/Accra")).toBe("3:30 PM");
    expect(formatTime("2026-08-11T09:00:00.000Z", "Africa/Accra")).toBe("9:00 AM");
  });

  it("weekdayShortName follows the contract's 0=Sunday", () => {
    expect(weekdayShortName(0)).toBe("Sun");
    expect(weekdayShortName(6)).toBe("Sat");
  });

  it("formatCivilDate uses the brand date format", () => {
    expect(formatCivilDate({ year: 2026, month: 8, day: 11 })).toBe(
      "Tue, Aug 11, 2026",
    );
  });

  it("formatWeekRange collapses shared months and spans years", () => {
    expect(formatWeekRange({ year: 2026, month: 8, day: 10 })).toBe("Aug 10–16, 2026");
    expect(formatWeekRange({ year: 2026, month: 12, day: 28 })).toBe(
      "Dec 28, 2026 – Jan 3, 2027",
    );
  });
});
