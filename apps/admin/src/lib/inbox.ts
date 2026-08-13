/**
 * Typed client for the moderation desks: enquiries and reviews
 * (design/api-contract.md §Enquiries BE-13, §Reviews BE-14).
 *
 *   GET    /v1/admin/enquiries              ?status=  → {items}
 *   GET    /v1/admin/enquiries/unread-count           → {count}
 *   PATCH  /v1/admin/enquiries/{id}         {status}  → {enquiry}
 *   DELETE /v1/admin/enquiries/{id}                   → 204
 *   GET    /v1/admin/reviews                ?status=  → {items}
 *   POST   /v1/admin/reviews/{id}/approve|reject      → {review}
 */

import { authedRequest, type RefreshCallbacks, type Session } from "@/lib/api";

export type EnquiryStatus = "new" | "read" | "replied" | "archived";

export const ENQUIRY_STATUSES: EnquiryStatus[] = ["new", "read", "replied", "archived"];

export interface Enquiry {
  id: string;
  name: string;
  email: string;
  phone?: string;
  subject?: string;
  message: string;
  status: EnquiryStatus;
  createdAt: string;
  updatedAt: string;
}

export const enquiriesApi = {
  async list(
    session: Session,
    callbacks: RefreshCallbacks,
    status?: EnquiryStatus,
  ): Promise<Enquiry[]> {
    const suffix = status ? `?status=${status}` : "";
    const { items } = await authedRequest<{ items: Enquiry[] }>(
      `/v1/admin/enquiries${suffix}`,
      session,
      callbacks,
    );
    return items;
  },

  async unreadCount(session: Session, callbacks: RefreshCallbacks): Promise<number> {
    const { count } = await authedRequest<{ count: number }>(
      "/v1/admin/enquiries/unread-count",
      session,
      callbacks,
    );
    return count;
  },

  async setStatus(
    session: Session,
    callbacks: RefreshCallbacks,
    enquiryId: string,
    status: EnquiryStatus,
  ): Promise<Enquiry> {
    const { enquiry } = await authedRequest<{ enquiry: Enquiry }>(
      `/v1/admin/enquiries/${enquiryId}`,
      session,
      callbacks,
      { method: "PATCH", body: { status } },
    );
    return enquiry;
  },

  remove(
    session: Session,
    callbacks: RefreshCallbacks,
    enquiryId: string,
  ): Promise<void> {
    return authedRequest<void>(`/v1/admin/enquiries/${enquiryId}`, session, callbacks, {
      method: "DELETE",
    });
  },
};

export type ReviewStatus = "pending" | "approved" | "rejected";

export const REVIEW_STATUSES: ReviewStatus[] = ["pending", "approved", "rejected"];

export interface Review {
  id: string;
  bookingId: string;
  clientId: string;
  serviceId?: string;
  rating: number;
  comment?: string;
  status: ReviewStatus;
  moderatedAt?: string;
  createdAt: string;
  updatedAt: string;
}

export const reviewsApi = {
  async list(
    session: Session,
    callbacks: RefreshCallbacks,
    status?: ReviewStatus,
  ): Promise<Review[]> {
    const suffix = status ? `?status=${status}` : "";
    const { items } = await authedRequest<{ items: Review[] }>(
      `/v1/admin/reviews${suffix}`,
      session,
      callbacks,
    );
    return items;
  },

  /** Approving publishes the review to the public site; rejecting takes it
   * off. Both are reversible — moderation is a judgement, not a lifecycle. */
  async moderate(
    session: Session,
    callbacks: RefreshCallbacks,
    reviewId: string,
    approve: boolean,
  ): Promise<Review> {
    const { review } = await authedRequest<{ review: Review }>(
      `/v1/admin/reviews/${reviewId}/${approve ? "approve" : "reject"}`,
      session,
      callbacks,
      { method: "POST" },
    );
    return review;
  },
};
