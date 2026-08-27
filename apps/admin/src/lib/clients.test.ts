import { beforeEach, describe, expect, it, vi } from "vitest";
import type { RefreshCallbacks, Session } from "@/lib/api";
import {
  clientsApi,
  notesApi,
  splitClientBookings,
  type ClientBooking,
  type ClientRecord,
  type SessionNote,
} from "./clients";

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

/** The path, and then the options — which is where method and body live. */
function lastCall(): [string, Session, RefreshCallbacks, Record<string, unknown>?] {
  return authedRequestMock.mock.calls.at(-1) as never;
}

function record(overrides: Partial<ClientRecord> = {}): ClientRecord {
  return {
    id: "client-1",
    name: "Ama Serwaa",
    email: "ama@example.com",
    tags: ["returning"],
    practiceNotes: "Prefers mornings.",
    recentBookings: [],
    payments: { totalPaidKobo: 50000, totalRefundedKobo: 0, paymentCount: 2, currency: "GHS" },
    documentCount: 1,
    formSubmissionCount: 3,
    ...overrides,
  };
}

function note(overrides: Partial<SessionNote> = {}): SessionNote {
  return {
    id: "note-1",
    bookingId: "bk-1",
    clientId: "client-1",
    practitionerId: "prac-1",
    privateNotes: "Reported shoulder tension.",
    sharedFeedback: "Keep up the stretches.",
    sharedResources: ["https://example.com/stretches"],
    createdAt: "2026-08-01T10:00:00.000Z",
    updatedAt: "2026-08-01T10:00:00.000Z",
    ...overrides,
  };
}

beforeEach(() => {
  authedRequestMock.mockReset();
});

describe("clientsApi", () => {
  it("unwraps the client list", async () => {
    authedRequestMock.mockResolvedValue({ items: [{ id: "client-1" }] });

    await expect(clientsApi.list(session, callbacks)).resolves.toEqual([{ id: "client-1" }]);
    expect(lastCall()[0]).toBe("/v1/clients");
  });

  it("reads one client record by id", async () => {
    authedRequestMock.mockResolvedValue({ record: record() });

    const result = await clientsApi.get(session, callbacks, "client-1");

    expect(result.email).toBe("ama@example.com");
    expect(lastCall()[0]).toBe("/v1/clients/client-1");
  });

  it("PATCHes only the practice-side fields", async () => {
    authedRequestMock.mockResolvedValue({ profile: { id: "client-1", tags: [] } });

    await clientsApi.updateProfile(session, callbacks, "client-1", {
      phone: "+233200000000",
      tags: ["vip"],
    });

    const [path, , , options] = lastCall();
    expect(path).toBe("/v1/clients/client-1");
    expect(options).toMatchObject({
      method: "PATCH",
      body: { phone: "+233200000000", tags: ["vip"] },
    });
    // Name and email belong to the client's own account, not the practice
    // record; sending them here would be the dashboard editing someone's
    // identity.
    expect(Object.keys(options?.body as object)).toEqual(["phone", "tags"]);
  });
});

describe("notesApi", () => {
  it("reads the full practitioner note", async () => {
    authedRequestMock.mockResolvedValue({ note: note() });

    const result = await notesApi.get(session, callbacks, "bk-1");

    expect(result.privateNotes).toBe("Reported shoulder tension.");
    expect(lastCall()[0]).toBe("/v1/bookings/bk-1/notes");
  });

  it("saves with PUT, replacing the content wholesale", async () => {
    authedRequestMock.mockResolvedValue({ note: note() });

    await notesApi.save(session, callbacks, "bk-1", {
      privateNotes: "Updated.",
      sharedFeedback: "",
      sharedResources: [],
    });

    const [path, , , options] = lastCall();
    expect(path).toBe("/v1/bookings/bk-1/notes");
    expect(options).toMatchObject({ method: "PUT" });
    // sharedAt is never in the body: editing after sharing must not
    // re-stamp it, and there is no unshare.
    expect(options?.body).not.toHaveProperty("sharedAt");
  });

  it("shares through its own route, which is what makes sharing deliberate", async () => {
    authedRequestMock.mockResolvedValue({ note: note({ sharedAt: "2026-08-02T09:00:00.000Z" }) });

    const result = await notesApi.share(session, callbacks, "bk-1");

    expect(result.sharedAt).toBe("2026-08-02T09:00:00.000Z");
    const [path, , , options] = lastCall();
    // A save cannot share by accident, because sharing is a different URL.
    expect(path).toBe("/v1/bookings/bk-1/notes/share");
    expect(options).toMatchObject({ method: "POST" });
  });
});

describe("splitClientBookings", () => {
  const now = new Date("2026-08-11T12:00:00.000Z");

  function booking(overrides: Partial<ClientBooking>): ClientBooking {
    return {
      id: "bk",
      serviceId: "svc-1",
      startAt: "2026-08-11T09:00:00.000Z",
      endAt: "2026-08-11T10:00:00.000Z",
      status: "confirmed",
      ...overrides,
    };
  }

  it("puts a session still running in upcoming, not past", () => {
    // The split is on endAt, not startAt: a session that began an hour ago
    // and has not finished is still ahead of the practitioner, not behind.
    const running = booking({
      id: "running",
      startAt: "2026-08-11T11:30:00.000Z",
      endAt: "2026-08-11T12:30:00.000Z",
    });

    const { upcoming, past } = splitClientBookings([running], now);

    expect(upcoming.map((b) => b.id)).toEqual(["running"]);
    expect(past).toEqual([]);
  });

  it("treats a cancelled future booking as past", () => {
    const cancelled = booking({
      id: "cancelled",
      status: "cancelled",
      startAt: "2026-08-20T09:00:00.000Z",
      endAt: "2026-08-20T10:00:00.000Z",
    });

    const { upcoming, past } = splitClientBookings([cancelled], now);

    // It is not something to prepare for, so it belongs in the history.
    expect(upcoming).toEqual([]);
    expect(past.map((b) => b.id)).toEqual(["cancelled"]);
  });

  it("orders upcoming soonest-first and past most-recent-first", () => {
    const bookings = [
      booking({ id: "later", startAt: "2026-08-25T09:00:00.000Z", endAt: "2026-08-25T10:00:00.000Z" }),
      booking({ id: "sooner", startAt: "2026-08-13T09:00:00.000Z", endAt: "2026-08-13T10:00:00.000Z" }),
      booking({ id: "old", startAt: "2026-07-01T09:00:00.000Z", endAt: "2026-07-01T10:00:00.000Z" }),
      booking({ id: "recent", startAt: "2026-08-05T09:00:00.000Z", endAt: "2026-08-05T10:00:00.000Z" }),
    ];

    const { upcoming, past } = splitClientBookings(bookings, now);

    expect(upcoming.map((b) => b.id)).toEqual(["sooner", "later"]);
    expect(past.map((b) => b.id)).toEqual(["recent", "old"]);
  });

  it("loses nothing, whatever the statuses", () => {
    const bookings: ClientBooking[] = [
      booking({ id: "a", status: "completed" }),
      booking({ id: "b", status: "no_show" }),
      booking({ id: "c", status: "cancelled" }),
      booking({ id: "d", startAt: "2026-09-01T09:00:00.000Z", endAt: "2026-09-01T10:00:00.000Z" }),
    ];

    const { upcoming, past } = splitClientBookings(bookings, now);

    // Every booking lands in exactly one half — a client's history with a
    // missing session is worse than an unsorted one.
    expect(upcoming.length + past.length).toBe(bookings.length);
    expect([...upcoming, ...past].map((b) => b.id).sort()).toEqual(["a", "b", "c", "d"]);
  });

  it("handles a client with no sessions yet", () => {
    expect(splitClientBookings([], now)).toEqual({ upcoming: [], past: [] });
  });
});
