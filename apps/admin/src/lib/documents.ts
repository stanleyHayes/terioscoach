/**
 * Typed client for practitioner-side client documents
 * (design/api-contract.md §Documents BE-11).
 *
 *   POST   /v1/admin/documents/sign-upload  → {url, fields, expiresAt}
 *   POST   /v1/admin/documents              → {document}
 *   GET    /v1/admin/documents?clientId=…   → {items: [document]}
 *   GET    /v1/admin/documents/{id}/url     → {url}
 *   PATCH  /v1/admin/documents/{id}         → {document}
 *   DELETE /v1/admin/documents/{id}         → 204
 *
 * Bytes go straight from the browser to the media store under a signature
 * the API mints, exactly as CMS imagery does — see lib/media.ts. The API is
 * only ever told what landed, so a client handout never sits in its memory.
 */

import { ApiError, authedRequest, type RefreshCallbacks, type Session } from "@/lib/api";

export interface ClientDocument {
  id: string;
  kind: string;
  clientId?: string;
  title: string;
  filename: string;
  format?: string;
  bytes: number;
  visibleToClient: boolean;
  createdAt: string;
  updatedAt: string;
}

interface SignedUpload {
  url: string;
  fields: Record<string, string>;
  expiresAt: string;
}

interface StoreUpload {
  public_id: string;
  bytes: number;
}

/** Matches document.MaxBytes on the API — rejected here to save the round trip. */
export const MAX_DOCUMENT_BYTES = 10 * 1024 * 1024;

const ACCEPTED = [
  "application/pdf",
  "image/jpeg",
  "image/png",
  "image/webp",
];

export const DOCUMENT_ACCEPT_ATTRIBUTE = ACCEPTED.join(",");

/** Why this file cannot be sent, or null when it can. */
export function documentRejectionReason(file: File): string | null {
  if (!ACCEPTED.includes(file.type)) {
    return "Share a PDF, JPEG, PNG, or WebP file.";
  }
  if (file.size > MAX_DOCUMENT_BYTES) {
    return "That file is over 10 MB. Send a smaller copy.";
  }
  return null;
}

export const documentsApi = {
  /**
   * Everything on this client's file, newest first per the API — including
   * the signed agreements the API files itself under `signed_form`. Callers
   * showing "documents I shared" filter to `client_document`; the signed
   * agreements have their own card, and listing them in both places would
   * invite someone to delete a legal record from the wrong one.
   */
  async listForClient(
    session: Session,
    callbacks: RefreshCallbacks,
    clientId: string,
  ): Promise<ClientDocument[]> {
    const { items } = await authedRequest<{ items: ClientDocument[] }>(
      `/v1/admin/documents?clientId=${encodeURIComponent(clientId)}`,
      session,
      callbacks,
    );
    return items;
  },

  /**
   * Uploads one file for one client and records it. The document is
   * visible to the client the moment the API accepts it, which is the whole
   * point of the action — there is no second "share" step to forget.
   */
  async shareWithClient(
    session: Session,
    callbacks: RefreshCallbacks,
    clientId: string,
    file: File,
  ): Promise<ClientDocument> {
    const reason = documentRejectionReason(file);
    if (reason) {
      throw new ApiError(400, "validation_error", reason);
    }

    const signed = await authedRequest<SignedUpload>(
      "/v1/admin/documents/sign-upload",
      session,
      callbacks,
      { method: "POST", body: { kind: "client_document", clientId, filename: file.name } },
    );

    const form = new FormData();
    for (const [name, value] of Object.entries(signed.fields)) {
      form.append(name, value);
    }
    form.append("file", file);

    const response = await fetch(signed.url, { method: "POST", body: form });
    if (!response.ok) {
      // The store's error body is its own shape; the status is the only part
      // worth reporting, and retrying is the only move either way.
      throw new ApiError(
        response.status,
        "upload_failed",
        "That file could not be uploaded. Try again.",
      );
    }
    const uploaded = (await response.json()) as StoreUpload;

    const { document } = await authedRequest<{ document: ClientDocument }>(
      "/v1/admin/documents",
      session,
      callbacks,
      {
        method: "POST",
        body: {
          kind: "client_document",
          clientId,
          publicId: uploaded.public_id,
          filename: file.name,
          bytes: uploaded.bytes,
        },
      },
    );
    return document;
  },

  /** A short-lived signed URL — private documents have no public address. */
  async downloadUrl(
    session: Session,
    callbacks: RefreshCallbacks,
    documentId: string,
  ): Promise<string> {
    const { url } = await authedRequest<{ url: string }>(
      `/v1/admin/documents/${documentId}/url`,
      session,
      callbacks,
    );
    return url;
  },

  /** Hides a document from the client without deleting the practice's copy. */
  async setVisibility(
    session: Session,
    callbacks: RefreshCallbacks,
    documentId: string,
    visibleToClient: boolean,
  ): Promise<ClientDocument> {
    const { document } = await authedRequest<{ document: ClientDocument }>(
      `/v1/admin/documents/${documentId}`,
      session,
      callbacks,
      { method: "PATCH", body: { visibleToClient } },
    );
    return document;
  },

  async remove(
    session: Session,
    callbacks: RefreshCallbacks,
    documentId: string,
  ): Promise<void> {
    await authedRequest(`/v1/admin/documents/${documentId}`, session, callbacks, {
      method: "DELETE",
    });
  },
};
