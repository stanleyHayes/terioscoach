import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { API_BASE_URL, ApiError } from "./api";
import {
  createBooking,
  cutoffPassed,
  getSlots,
  myBookings,
  requestCancelBooking,
  requestRescheduleBooking,
  splitBookings,
  type Booking,
} from "./bookings";

function jsonResponse(status: number, body?: unknown): Response {
  return new Response(body === undefined ? null : JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const fetchMock = vi.fn();

const session = {
  accessToken: "a1",
  accessTokenExpiresAt: "2026-08-11T17:00:00Z",
  refreshToken: "r1",
};
const callbacks = { onTokensRefreshed: vi.fn() };

const booking: Booking = {
  id: "b1",
  clientId: "u1",
  practitionerId: "p1",
  serviceId: "s1",
  startAt: "2026-08-20T09:30:00Z",
  endAt: "2026-08-20T10:15:00Z",
  status: "confirmed",
  createdAt: "2026-08-11T12:00:00Z",
  updatedAt: "2026-08-11T12:00:00Z",
};

beforeEach(() => {
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  fetchMock.mockReset();
  callbacks.onTokensRefreshed.mockReset();
});

describe("getSlots", () => {
  it("requests the public slots endpoint with query params and no-store cache", async () => {
    const payload = {
      serviceId: "s1",
      durationMinutes: 45,
      timezone: "Africa/Accra",
      slots: [
        {
          startAt: "2026-08-20T09:30:00Z",
          endAt: "2026-08-20T10:15:00Z",
        },
      ],
    };
    fetchMock.mockResolvedValueOnce(jsonResponse(200, payload));

    const result = await getSlots({
      serviceId: "s1",
      from: "2026-08-20",
      to: "2026-08-20",
      tz: "Africa/Accra",
    });

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe(
      `${API_BASE_URL}/v1/availability/slots?serviceId=s1&from=2026-08-20&to=2026-08-20&tz=Africa%2FAccra`,
    );
    expect(init).toMatchObject({ method: "GET", cache: "no-store" });
    expect(init.headers?.Authorization).toBeUndefined();
    expect(result).toEqual(payload);
  });

  it("maps a 404 to a service_not_found ApiError", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(404, {
        error: { code: "service_not_found", message: "Nope" },
      }),
    );

    const error = await getSlots({
      serviceId: "gone",
      from: "2026-08-20",
      to: "2026-08-20",
      tz: "Africa/Accra",
    }).catch((e) => e);

    expect(error).toMatchObject({ status: 404, code: "service_not_found" });
  });
});

describe("createBooking", () => {
  it("posts {serviceId, startAt, tz} with the bearer token and unwraps {booking}", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(201, { booking }));

    const result = await createBooking(session, callbacks, {
      serviceId: "s1",
      startAt: "2026-08-20T09:30:00Z",
      tz: "Africa/Accra",
      participant: { name: "Client", under18: false, accurate: true },
    });

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe(`${API_BASE_URL}/v1/bookings`);
    expect(init).toMatchObject({
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: expect.any(String),
      },
      body: JSON.stringify({
        serviceId: "s1",
        startAt: "2026-08-20T09:30:00Z",
        tz: "Africa/Accra",
        participant: { name: "Client", under18: false, accurate: true },
      }),
    });
    expect(result).toEqual(booking);
  });

  it("surfaces the 409 slot_unavailable race as an ApiError", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(409, {
        error: { code: "slot_unavailable", message: "Taken" },
      }),
    );

    const error = await createBooking(session, callbacks, {
      serviceId: "s1",
      startAt: "2026-08-20T09:30:00Z",
      tz: "Africa/Accra",
      participant: { name: "Client", under18: false, accurate: true },
    }).catch((e) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({ status: 409, code: "slot_unavailable" });
  });
});

describe("myBookings", () => {
  it("returns the items array from /v1/bookings/mine", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(200, { items: [booking] }));

    const result = await myBookings(session, callbacks);

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe(`${API_BASE_URL}/v1/bookings/mine`);
    expect(init).toMatchObject({
      method: "GET",
      headers: { Authorization: expect.any(String) },
    });
    expect(result).toEqual([booking]);
  });
});

describe("requestRescheduleBooking", () => {
  it("posts the proposed startAt to the reschedule request route", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(202, { booking }));

    const result = await requestRescheduleBooking(session, callbacks, "b1", {
      startAt: "2026-08-21T09:30:00Z",
      tz: "Africa/Accra",
    });

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe(`${API_BASE_URL}/v1/bookings/b1/reschedule-request`);
    expect(init).toMatchObject({
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: expect.any(String),
      },
      body: JSON.stringify({
        startAt: "2026-08-21T09:30:00Z",
        tz: "Africa/Accra",
      }),
    });
    expect(result).toEqual(booking);
  });

  it("maps the 422 cutoff_passed error", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(422, {
        error: { code: "cutoff_passed", message: "Too late" },
      }),
    );

    const error = await requestRescheduleBooking(session, callbacks, "b1", {
      startAt: "2026-08-21T09:30:00Z",
      tz: "Africa/Accra",
    }).catch((e) => e);

    expect(error).toMatchObject({ status: 422, code: "cutoff_passed" });
  });
});

describe("requestCancelBooking", () => {
  it("posts reason and timezone to the cancellation request route", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(202, { booking }));

    const result = await requestCancelBooking(session, callbacks, "b1", {
      reason: "Family trip",
      tz: "Africa/Accra",
    });

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe(`${API_BASE_URL}/v1/bookings/b1/cancel-request`);
    expect(init).toMatchObject({
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: expect.any(String),
      },
      body: JSON.stringify({ reason: "Family trip", tz: "Africa/Accra" }),
    });
    expect(result).toEqual(booking);
  });
});

describe("cutoffPassed", () => {
  it("is true inside the 48-hour window and false before it", () => {
    const now = new Date("2026-08-18T09:00:00Z");
    expect(cutoffPassed("2026-08-20T08:59:59Z", now)).toBe(true); // < 48h away
    expect(cutoffPassed("2026-08-20T09:00:00Z", now)).toBe(true); // exactly 48h -> closed
    expect(cutoffPassed("2026-08-20T09:00:01Z", now)).toBe(false); // > 48h away
    expect(cutoffPassed("2026-08-18T09:00:00Z", now)).toBe(true); // already past
  });
});

describe("splitBookings", () => {
  const now = new Date("2026-08-11T12:00:00Z");

  const base = {
    clientId: "u1",
    practitionerId: "p1",
    serviceId: "s1",
    createdAt: "2026-08-01T12:00:00Z",
    updatedAt: "2026-08-01T12:00:00Z",
  };

  it("splits confirmed future bookings (ascending) from terminal/ended ones (descending)", () => {
    const bookings: Booking[] = [
      {
        ...base,
        id: "past-confirmed",
        startAt: "2026-08-10T09:00:00Z",
        endAt: "2026-08-10T09:45:00Z",
        status: "confirmed",
      },
      {
        ...base,
        id: "later",
        startAt: "2026-08-20T09:00:00Z",
        endAt: "2026-08-20T09:45:00Z",
        status: "confirmed",
      },
      {
        ...base,
        id: "done",
        startAt: "2026-08-05T09:00:00Z",
        endAt: "2026-08-05T09:45:00Z",
        status: "completed",
      },
      {
        ...base,
        id: "sooner",
        startAt: "2026-08-12T09:00:00Z",
        endAt: "2026-08-12T09:45:00Z",
        status: "confirmed",
      },
      {
        ...base,
        id: "cancelled",
        startAt: "2026-08-15T09:00:00Z",
        endAt: "2026-08-15T09:45:00Z",
        status: "cancelled",
      },
      {
        ...base,
        id: "noshow",
        startAt: "2026-08-03T09:00:00Z",
        endAt: "2026-08-03T09:45:00Z",
        status: "no_show",
      },
    ];

    const { upcoming, past } = splitBookings(bookings, now);

    expect(upcoming.map((b) => b.id)).toEqual(["sooner", "later"]);
    expect(past.map((b) => b.id)).toEqual([
      "cancelled",
      "past-confirmed",
      "done",
      "noshow",
    ]);
  });
});

describe("appointment lifecycle boundaries", () => {
  it.each([
    ["2026-08-20T09:29:59.999Z", "upcoming"],
    ["2026-08-20T09:30:00Z", "inProgress"],
    ["2026-08-20T10:14:59.999Z", "inProgress"],
    ["2026-08-20T10:15:00Z", "past"],
  ] as const)("classifies the instant %s as %s", (instant, group) => {
    const result = splitBookings([booking], new Date(instant));
    expect(result[group].map((item) => item.id)).toEqual([booking.id]);
    expect(Object.values(result).flat()).toHaveLength(1);
  });
  it("keeps payment pending until the server expires it", () => {
    const pending = { ...booking, status: "pending_payment" as const };
    expect(
      splitBookings([pending], new Date("2026-08-21T12:00:00Z")).pending,
    ).toEqual([pending]);
    const expired = {
      ...pending,
      status: "cancelled" as const,
      paymentExpired: true,
    };
    expect(
      splitBookings([expired], new Date("2026-08-21T12:00:00Z")).past,
    ).toEqual([expired]);
  });
  it.each([
    "America/New_York",
    "America/Chicago",
    "America/Los_Angeles",
    "Pacific/Honolulu",
    "Africa/Accra",
    "UTC",
  ])("display in %s cannot change grouping or the stored instant", (zone) => {
    const original = booking.startAt;
    new Intl.DateTimeFormat("en-US", {
      timeZone: zone,
      dateStyle: "full",
      timeStyle: "long",
    }).format(new Date(original));
    expect(
      splitBookings([booking], new Date("2026-08-20T09:00:00Z")).upcoming,
    ).toEqual([booking]);
    expect(booking.startAt).toBe(original);
  });
});
