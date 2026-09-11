/**
 * Typed client for service agreements, practice side
 * (design/api-contract.md §Agreements BE-14).
 *
 *   GET   /v1/admin/agreements                   → {items} incl. retired
 *   POST  /v1/admin/agreements                   → {agreement}
 *   PATCH /v1/admin/agreements/{id}              → {agreement}
 *   GET   /v1/admin/clients/{id}/agreements      → {items} that client's signatures
 *   GET   /v1/agreements/signatures/{id}/pdf     → the signed agreement as a PDF
 *
 * A service points at an agreement through `Service.agreementId`; several
 * services can share one, and a client signs each agreement once.
 */

import { authedRequest, type RefreshCallbacks, type Session } from "@/lib/api";

export interface Agreement {
  id: string;
  key: string;
  title: string;
  /** Plain text: "## " opens a heading, everything else is a paragraph. */
  body: string;
  /** Bumped by every change to the text; signatures are pinned to one. */
  version: number;
  active: boolean;
  updatedAt: string;
}

export interface AgreementSignature {
  id: string;
  agreementId: string;
  agreementTitle: string;
  agreementVersion: number;
  clientId: string;
  clientName: string;
  /** Exactly what the client typed. */
  signedName: string;
  bookingId?: string;
  signedAt: string;
}

export interface AgreementPatch {
  title?: string;
  body?: string;
  active?: boolean;
}

export const agreementsApi = {
  async list(session: Session, callbacks: RefreshCallbacks): Promise<Agreement[]> {
    const { items } = await authedRequest<{ items: Agreement[] }>(
      "/v1/admin/agreements",
      session,
      callbacks,
    );
    return items;
  },

  async create(
    session: Session,
    callbacks: RefreshCallbacks,
    draft: { key: string; title: string; body: string },
  ): Promise<Agreement> {
    const { agreement } = await authedRequest<{ agreement: Agreement }>(
      "/v1/admin/agreements",
      session,
      callbacks,
      { method: "POST", body: draft },
    );
    return agreement;
  },

  async update(
    session: Session,
    callbacks: RefreshCallbacks,
    id: string,
    patch: AgreementPatch,
  ): Promise<Agreement> {
    const { agreement } = await authedRequest<{ agreement: Agreement }>(
      `/v1/admin/agreements/${id}`,
      session,
      callbacks,
      { method: "PATCH", body: patch },
    );
    return agreement;
  },

  async signaturesForClient(
    session: Session,
    callbacks: RefreshCallbacks,
    clientId: string,
  ): Promise<AgreementSignature[]> {
    const { items } = await authedRequest<{ items: AgreementSignature[] }>(
      `/v1/admin/clients/${clientId}/agreements`,
      session,
      callbacks,
    );
    return items;
  },
};

/**
 * The URL of a signed agreement's PDF.
 *
 * The route is authenticated, so this is opened by fetching it with the
 * access token and handing the browser a blob — a bare link would arrive
 * without credentials and be refused.
 */
export function signedAgreementPath(signatureId: string): string {
  return `/v1/agreements/signatures/${signatureId}/pdf`;
}
