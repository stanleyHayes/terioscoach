import { beforeEach, describe, expect, it, vi } from "vitest";
import type { RefreshCallbacks, Session } from "@/lib/api";
import {
  documentsApi,
  formatBytes,
  formsApi,
  notesApi,
  paymentsApi,
  reviewableBookings,
  reviewsApi,
  type ClientReview,
} from "./portal";

const authedRequestMock = vi.fn();

vi.mock("@/lib/api", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...original,
    authedRequest: (...args: unknown[]) => authedRequestMock(...args),
  };
});

const session: Session = {
  accessToken: "access",
  accessTokenExpiresAt: "2099-01-01T00:00:00Z",
  refreshToken: "refresh",
};
const callbacks: RefreshCallbacks = { onTokensRefreshed: vi.fn() };

function lastCall(): [string, Session, RefreshCallbacks, Record<string, unknown>?] {
  return authedRequestMock.mock.calls.at(-1) as never;
}

beforeEach(() => {
  authedRequestMock.mockReset();
});

describe("formsApi", () => {
  it("lists only the client's own forms", async () => {
    authedRequestMock.mockResolvedValue({ items: [{ id: "sub-1" }] });

    await expect(formsApi.listMine(session, callbacks)).resolves.toEqual([{ id: "sub-1" }]);
    // /mine, not /forms — the route itself is what scopes this to the
    // caller, rather than a client id the browser could change.
    expect(lastCall()[0]).toBe("/v1/forms/mine");
  });

  it("fetches the definition alongside the submission", async () => {
    authedRequestMock.mockResolvedValue({ submission: {}, form: {}, integrityOk: true });

    await formsApi.get(session, callbacks, "sub-1");

    // The form may have been reworded since it was sent; what has to be
    // rendered is the version that was actually answered.
    expect(lastCall()[0]).toBe("/v1/forms/mine/sub-1");
  });

  it("submits through its own one-way route", async () => {
    authedRequestMock.mockResolvedValue({ submission: { id: "sub-1", status: "submitted" } });

    const result = await formsApi.submit(session, callbacks, "sub-1", {
      answers: { full_name: { value: "Ama" } },
      signature: { typedName: "Ama Serwaa", imageData: "data:image/png;base64,AAA" },
    });

    expect(result.status).toBe("submitted");
    const [path, , , options] = lastCall();
    expect(path).toBe("/v1/forms/mine/sub-1/submit");
    // POST to /submit, never a PUT over the record: a signed consent form
    // that can be edited afterwards is not a consent record.
    expect(options).toMatchObject({ method: "POST" });
  });
});

describe("documentsApi", () => {
  it("lists only the client's own documents", async () => {
    authedRequestMock.mockResolvedValue({ items: [{ id: "doc-1" }] });

    await documentsApi.listMine(session, callbacks);

    expect(lastCall()[0]).toBe("/v1/documents/mine");
  });

  it("fetches a download link on demand and returns only the URL", async () => {
    authedRequestMock.mockResolvedValue({ url: "https://res.cloudinary.com/signed", expiresIn: 3600 });

    const url = await documentsApi.downloadUrl(session, callbacks, "doc-1");

    // Signed links are short-lived and fetched per click, so one never sits
    // in the rendered page long enough to be shoulder-surfed or shared.
    expect(url).toBe("https://res.cloudinary.com/signed");
    expect(lastCall()[0]).toBe("/v1/documents/mine/doc-1/url");
  });
});

describe("paymentsApi", () => {
  it("lists only the client's own payments", async () => {
    authedRequestMock.mockResolvedValue({ items: [{ id: "pay-1" }] });

    await paymentsApi.listMine(session, callbacks);

    expect(lastCall()[0]).toBe("/v1/payments/mine");
  });

  it("initializes a checkout from a booking id alone", async () => {
    authedRequestMock.mockResolvedValue({
      authorizationUrl: "https://checkout.paystack.com/abc",
      reference: "terios_bk-1_1",
    });

    const url = await paymentsApi.initialize(session, callbacks, "bk-1");

    expect(url).toBe("https://checkout.paystack.com/abc");
    const [path, , , options] = lastCall();
    expect(path).toBe("/v1/payments/initialize");
    // Only the booking id crosses the wire. Amount and currency are the
    // server's to decide, or the price becomes a browser field.
    expect(options).toMatchObject({ method: "POST", body: { bookingId: "bk-1" } });
    expect(Object.keys(options?.body as object)).toEqual(["bookingId"]);
  });
});

describe("reviewsApi", () => {
  it("lists only the client's own reviews", async () => {
    authedRequestMock.mockResolvedValue({ items: [{ id: "rev-1" }] });

    await reviewsApi.listMine(session, callbacks);

    expect(lastCall()[0]).toBe("/v1/reviews/mine");
  });

  it("submits a review against a booking", async () => {
    authedRequestMock.mockResolvedValue({ review: { id: "rev-1", status: "pending" } });

    const result = await reviewsApi.submit(session, callbacks, {
      bookingId: "bk-1",
      rating: 5,
      comment: "Calm and unhurried.",
    });

    // Pending, not published: the practitioner moderates before anything
    // reaches the public site.
    expect(result.status).toBe("pending");
    const [path, , , options] = lastCall();
    expect(path).toBe("/v1/reviews");
    expect(options).toMatchObject({ method: "POST", body: { bookingId: "bk-1", rating: 5 } });
  });

  it("edits an existing review with PATCH", async () => {
    authedRequestMock.mockResolvedValue({ review: { id: "rev-1", rating: 4 } });

    await reviewsApi.update(session, callbacks, "rev-1", { rating: 4 });

    const [path, , , options] = lastCall();
    expect(path).toBe("/v1/reviews/rev-1");
    expect(options).toMatchObject({ method: "PATCH", body: { rating: 4 } });
  });
});

describe("notesApi", () => {
  it("reads the shared half of a session note", async () => {
    authedRequestMock.mockResolvedValue({
      note: { bookingId: "bk-1", sharedFeedback: "Keep stretching.", sharedResources: [], sharedAt: "x" },
    });

    const note = await notesApi.getShared(session, callbacks, "bk-1");

    expect(note.sharedFeedback).toBe("Keep stretching.");
    // The practitioner's private notes are not part of this shape at all —
    // the client's projection has no field that could carry them.
    expect(note).not.toHaveProperty("privateNotes");
    expect(lastCall()[0]).toBe("/v1/bookings/bk-1/notes");
  });
});

describe("formatBytes", () => {
  it.each([
    [0, "0 B"],
    [512, "512 B"],
    [1023, "1023 B"],
    [1024, "1 KB"],
    [1536, "2 KB"],
    [1024 * 1024 - 1, "1024 KB"],
    [1024 * 1024, "1.0 MB"],
    [5 * 1024 * 1024 + 512 * 1024, "5.5 MB"],
  ])("renders %i bytes as %s", (bytes, expected) => {
    expect(formatBytes(bytes)).toBe(expected);
  });
});

describe("reviewableBookings", () => {
  const bookings = [
    { id: "bk-done", status: "completed" },
    { id: "bk-reviewed", status: "completed" },
    { id: "bk-upcoming", status: "confirmed" },
    { id: "bk-cancelled", status: "cancelled" },
    { id: "bk-noshow", status: "no_show" },
  ];
  const reviews = [{ bookingId: "bk-reviewed" } as ClientReview];

  it("offers only completed sessions that have not been reviewed", () => {
    expect(reviewableBookings(bookings, reviews).map((b) => b.id)).toEqual(["bk-done"]);
  });

  it("asks for nothing when every completed session is already reviewed", () => {
    const all = [{ bookingId: "bk-done" }, { bookingId: "bk-reviewed" }] as ClientReview[];
    expect(reviewableBookings(bookings, all)).toEqual([]);
  });

  it("never asks about a session that has not happened", () => {
    // Being invited to review a massage you have not had yet is the kind
    // of prompt that erodes trust in everything else the portal says.
    expect(reviewableBookings(bookings, []).map((b) => b.id)).toEqual(["bk-done", "bk-reviewed"]);
  });

  it("copes with a client who has no sessions at all", () => {
    expect(reviewableBookings([], [])).toEqual([]);
  });
});
