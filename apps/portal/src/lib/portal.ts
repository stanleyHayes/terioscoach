/**
 * Typed client for the authenticated portal surfaces
 * (design/api-contract.md §Forms BE-10, §Documents BE-11,
 * §Payments BE-06, §Reviews BE-14, §Session Notes BE-08).
 *
 *   GET  /v1/forms/mine                  → {items}
 *   GET  /v1/forms/mine/{id}             → {submission, form, integrityOk}
 *   POST /v1/forms/mine/{id}/submit      → {submission}
 *   GET  /v1/documents/mine              → {items}   (shared only)
 *   GET  /v1/documents/mine/{id}/url     → {url, expiresIn}
 *   GET  /v1/payments/mine               → {items}
 *   POST /v1/payments/initialize         → {authorizationUrl, reference}
 *   GET  /v1/reviews/mine                → {items}
 *   POST /v1/reviews                     → 201 {review}
 *   PATCH /v1/reviews/{id}               → {review}  (pending only)
 *   GET  /v1/bookings/{id}/notes         → {note}    (shared projection)
 *
 * Every call goes through authedRequest (single 401 → refresh → retry).
 */

import { authedRequest, type RefreshCallbacks, type Session } from "./api";

// ---- Forms and signatures (CX-07) ----

export type FieldType =
  | "text"
  | "textarea"
  | "number"
  | "date"
  | "select"
  | "radio"
  | "checkbox"
  | "signature";

export interface FormField {
  key: string;
  label: string;
  type: FieldType;
  required: boolean;
  helpText?: string;
  options: string[];
}

export interface FormDefinition {
  id: string;
  title: string;
  description?: string;
  fields: FormField[];
}

export interface Answer {
  value?: string;
  values?: string[];
}

export type SubmissionStatus = "assigned" | "submitted";

export interface FormSubmission {
  id: string;
  formId: string;
  formTitle: string;
  bookingId?: string;
  status: SubmissionStatus;
  answers: Record<string, Answer>;
  signature?: { typedName: string; signedAt: string };
  assignedAt: string;
  submittedAt?: string;
}

export interface SubmissionView {
  submission: FormSubmission;
  form: FormDefinition;
  integrityOk: boolean;
  signatureImage?: string;
}

export interface SubmitFormInput {
  answers: Record<string, Answer>;
  signature?: { typedName: string; imageData: string };
}

export const formsApi = {
  async listMine(session: Session, callbacks: RefreshCallbacks): Promise<FormSubmission[]> {
    const { items } = await authedRequest<{ items: FormSubmission[] }>(
      "/v1/forms/mine",
      session,
      callbacks,
    );
    return items;
  },

  /** The definition travels with the submission, because a form may have
   * been edited since it was sent and what must be rendered is the version
   * that was actually answered. */
  get(
    session: Session,
    callbacks: RefreshCallbacks,
    submissionId: string,
  ): Promise<SubmissionView> {
    return authedRequest<SubmissionView>(`/v1/forms/mine/${submissionId}`, session, callbacks);
  },

  /** One-way: a signed consent record cannot be edited afterwards. */
  async submit(
    session: Session,
    callbacks: RefreshCallbacks,
    submissionId: string,
    input: SubmitFormInput,
  ): Promise<FormSubmission> {
    const { submission } = await authedRequest<{ submission: FormSubmission }>(
      `/v1/forms/mine/${submissionId}/submit`,
      session,
      callbacks,
      { method: "POST", body: input },
    );
    return submission;
  },
};

// ---- Documents (CX-09) ----

export interface ClientDocument {
  id: string;
  title: string;
  filename: string;
  format?: string;
  bytes: number;
  createdAt: string;
}

export const documentsApi = {
  async listMine(session: Session, callbacks: RefreshCallbacks): Promise<ClientDocument[]> {
    const { items } = await authedRequest<{ items: ClientDocument[] }>(
      "/v1/documents/mine",
      session,
      callbacks,
    );
    return items;
  },

  /** A short-lived signed link, fetched on demand rather than embedded in
   * the list — so a link never sits in the page long enough to leak. */
  async downloadUrl(
    session: Session,
    callbacks: RefreshCallbacks,
    documentId: string,
  ): Promise<string> {
    const { url } = await authedRequest<{ url: string; expiresIn: number }>(
      `/v1/documents/mine/${documentId}/url`,
      session,
      callbacks,
    );
    return url;
  },
};

// ---- Payments (CX-10) ----

export type PaymentStatus = "pending" | "success" | "failed" | "refunded";

export interface ClientPayment {
  id: string;
  bookingId: string;
  amountKobo: number;
  currency: string;
  status: PaymentStatus;
  channel?: string;
  paidAt?: string;
  createdAt: string;
}

export const paymentsApi = {
  async listMine(session: Session, callbacks: RefreshCallbacks): Promise<ClientPayment[]> {
    const { items } = await authedRequest<{ items: ClientPayment[] }>(
      "/v1/payments/mine",
      session,
      callbacks,
    );
    return items;
  },

  /** Returns Stripe's hosted checkout URL. Card and mobile-money details
   * never touch this app or the API. */
  async initialize(
    session: Session,
    callbacks: RefreshCallbacks,
    bookingId: string,
  ): Promise<string> {
    const { authorizationUrl } = await authedRequest<{
      authorizationUrl: string;
      reference: string;
    }>("/v1/payments/initialize", session, callbacks, {
      method: "POST",
      body: { bookingId },
    });
    return authorizationUrl;
  },
};

// ---- Reviews (CX-11) ----

export type ReviewStatus = "pending" | "approved" | "rejected";

export interface ClientReview {
  id: string;
  bookingId: string;
  rating: number;
  comment?: string;
  status: ReviewStatus;
  createdAt: string;
}

export const reviewsApi = {
  async listMine(session: Session, callbacks: RefreshCallbacks): Promise<ClientReview[]> {
    const { items } = await authedRequest<{ items: ClientReview[] }>(
      "/v1/reviews/mine",
      session,
      callbacks,
    );
    return items;
  },

  async submit(
    session: Session,
    callbacks: RefreshCallbacks,
    input: { bookingId: string; rating: number; comment?: string },
  ): Promise<ClientReview> {
    const { review } = await authedRequest<{ review: ClientReview }>(
      "/v1/reviews",
      session,
      callbacks,
      { method: "POST", body: input },
    );
    return review;
  },

  /** Editable only while still pending: once moderated, the text is frozen. */
  async update(
    session: Session,
    callbacks: RefreshCallbacks,
    reviewId: string,
    input: { rating?: number; comment?: string },
  ): Promise<ClientReview> {
    const { review } = await authedRequest<{ review: ClientReview }>(
      `/v1/reviews/${reviewId}`,
      session,
      callbacks,
      { method: "PATCH", body: input },
    );
    return review;
  },
};

// ---- Shared session feedback (CX-08) ----

/** The client's projection of a session note: the shared half only. The
 * practitioner's private notes are not part of this shape at all. */
export interface SharedNote {
  bookingId: string;
  sharedFeedback: string;
  sharedResources: string[];
  sharedAt: string;
}

export const notesApi = {
  /** Throws ApiError 404 note_not_found when nothing has been shared —
   * which is indistinguishable from "no note exists", by design. */
  async getShared(
    session: Session,
    callbacks: RefreshCallbacks,
    bookingId: string,
  ): Promise<SharedNote> {
    const { note } = await authedRequest<{ note: SharedNote }>(
      `/v1/bookings/${bookingId}/notes`,
      session,
      callbacks,
    );
    return note;
  },
};

/** Human file size for a document list. */
export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

/** Which sessions can still be reviewed: completed, and not already done. */
export function reviewableBookings<T extends { id: string; status: string }>(
  bookings: T[],
  reviews: ClientReview[],
): T[] {
  const reviewed = new Set(reviews.map((review) => review.bookingId));
  return bookings.filter((booking) => booking.status === "completed" && !reviewed.has(booking.id));
}
