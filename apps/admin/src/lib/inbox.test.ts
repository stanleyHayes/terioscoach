import { beforeEach, describe, expect, it, vi } from "vitest";
import type { RefreshCallbacks, Session } from "@/lib/api";
import {
  ENQUIRY_STATUSES,
  REVIEW_STATUSES,
  enquiriesApi,
  reviewsApi,
  type Enquiry,
  type Review,
} from "./inbox";

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

function enquiry(overrides: Partial<Enquiry> = {}): Enquiry {
  return {
    id: "enq-1",
    name: "Kojo Mensah",
    email: "kojo@example.com",
    message: "Do you offer evening sessions?",
    status: "new",
    createdAt: "2026-08-10T09:00:00.000Z",
    updatedAt: "2026-08-10T09:00:00.000Z",
    ...overrides,
  };
}

function review(overrides: Partial<Review> = {}): Review {
  return {
    id: "rev-1",
    bookingId: "bk-1",
    clientId: "client-1",
    rating: 5,
    comment: "Calm and unhurried.",
    status: "pending",
    createdAt: "2026-08-10T09:00:00.000Z",
    updatedAt: "2026-08-10T09:00:00.000Z",
    ...overrides,
  };
}

beforeEach(() => {
  authedRequestMock.mockReset();
});

describe("enquiriesApi", () => {
  it("lists every enquiry when no status is given", async () => {
    authedRequestMock.mockResolvedValue({ items: [enquiry()] });

    await enquiriesApi.list(session, callbacks);

    // No trailing "?status=undefined" — that would filter on a status the
    // server does not know and quietly return nothing.
    expect(lastCall()[0]).toBe("/v1/admin/enquiries");
  });

  it.each(ENQUIRY_STATUSES)("filters the inbox by %s", async (status) => {
    authedRequestMock.mockResolvedValue({ items: [] });

    await enquiriesApi.list(session, callbacks, status);

    expect(lastCall()[0]).toBe(`/v1/admin/enquiries?status=${status}`);
  });

  it("reads the unread count for the sidebar badge", async () => {
    authedRequestMock.mockResolvedValue({ count: 3 });

    await expect(enquiriesApi.unreadCount(session, callbacks)).resolves.toBe(3);
    expect(lastCall()[0]).toBe("/v1/admin/enquiries/unread-count");
  });

  it("reports zero rather than nothing when the inbox is clear", async () => {
    authedRequestMock.mockResolvedValue({ count: 0 });

    // A badge that renders `undefined` on an empty inbox is the bug this
    // catches — 0 is falsy and easy to lose on the way through.
    await expect(enquiriesApi.unreadCount(session, callbacks)).resolves.toBe(0);
  });

  it.each(ENQUIRY_STATUSES)("moves an enquiry to %s with PATCH", async (status) => {
    authedRequestMock.mockResolvedValue({ enquiry: enquiry({ status }) });

    const result = await enquiriesApi.setStatus(session, callbacks, "enq-1", status);

    expect(result.status).toBe(status);
    const [path, , , options] = lastCall();
    expect(path).toBe("/v1/admin/enquiries/enq-1");
    expect(options).toMatchObject({ method: "PATCH", body: { status } });
  });

  it("deletes an enquiry, which the API answers with no content", async () => {
    authedRequestMock.mockResolvedValue(undefined);

    await expect(enquiriesApi.remove(session, callbacks, "enq-1")).resolves.toBeUndefined();
    const [path, , , options] = lastCall();
    expect(path).toBe("/v1/admin/enquiries/enq-1");
    expect(options).toMatchObject({ method: "DELETE" });
  });
});

describe("reviewsApi", () => {
  it("lists every review when no status is given", async () => {
    authedRequestMock.mockResolvedValue({ items: [review()] });

    await reviewsApi.list(session, callbacks);

    expect(lastCall()[0]).toBe("/v1/admin/reviews");
  });

  it.each(REVIEW_STATUSES)("filters the moderation queue by %s", async (status) => {
    authedRequestMock.mockResolvedValue({ items: [] });

    await reviewsApi.list(session, callbacks, status);

    expect(lastCall()[0]).toBe(`/v1/admin/reviews?status=${status}`);
  });

  it("approves through the approve route", async () => {
    authedRequestMock.mockResolvedValue({ review: review({ status: "approved" }) });

    const result = await reviewsApi.moderate(session, callbacks, "rev-1", true);

    expect(result.status).toBe("approved");
    const [path, , , options] = lastCall();
    // Approving is what publishes a client's words to the public site, so
    // it is its own URL rather than a flag on a shared one.
    expect(path).toBe("/v1/admin/reviews/rev-1/approve");
    expect(options).toMatchObject({ method: "POST" });
  });

  it("rejects through the reject route", async () => {
    authedRequestMock.mockResolvedValue({ review: review({ status: "rejected" }) });

    const result = await reviewsApi.moderate(session, callbacks, "rev-1", false);

    expect(result.status).toBe("rejected");
    expect(lastCall()[0]).toBe("/v1/admin/reviews/rev-1/reject");
  });
});
