/**
 * Typed client for service agreements (design/api-contract.md §Agreements BE-14).
 *
 *   GET  /v1/services/{id}/agreement            → {required, signed, agreement?, signature?}
 *   POST /v1/agreements/{id}/sign               → {signature}
 *   GET  /v1/agreements/mine                    → {items}
 *   GET  /v1/agreements/signatures/{id}/pdf     → the signed agreement as a PDF
 *
 * One signature covers every service the agreement is attached to, and
 * every later booking of them — the client is asked once per agreement, not
 * once per booking.
 */

import { authedRequest, type RefreshCallbacks, type Session } from "@/lib/api";

export interface Agreement {
  id: string;
  key: string;
  title: string;
  /** The full text, as plain paragraphs with "## " headings. */
  body: string;
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
  /** Exactly what the client typed, kept verbatim. */
  signedName: string;
  bookingId?: string;
  signedAt: string;
}

export interface AgreementStatus {
  required: boolean;
  signed: boolean;
  agreement?: Agreement;
  signature?: AgreementSignature;
}

export const agreementsApi = {
  /** What stands between this client and booking this service, if anything. */
  async forService(
    session: Session,
    callbacks: RefreshCallbacks,
    serviceId: string,
  ): Promise<AgreementStatus> {
    return authedRequest<AgreementStatus>(
      `/v1/services/${serviceId}/agreement`,
      session,
      callbacks,
    );
  },

  /** Records acceptance. Signing an agreement already on file is a no-op. */
  async sign(
    session: Session,
    callbacks: RefreshCallbacks,
    agreementId: string,
    signedName: string,
    bookingId?: string,
  ): Promise<AgreementSignature> {
    const { signature } = await authedRequest<{ signature: AgreementSignature }>(
      `/v1/agreements/${agreementId}/sign`,
      session,
      callbacks,
      { method: "POST", body: { signedName, bookingId } },
    );
    return signature;
  },

  async listMine(
    session: Session,
    callbacks: RefreshCallbacks,
  ): Promise<AgreementSignature[]> {
    const { items } = await authedRequest<{ items: AgreementSignature[] }>(
      "/v1/agreements/mine",
      session,
      callbacks,
    );
    return items;
  },
};

/**
 * Splits agreement text into the blocks a reader renders.
 *
 * The server sends plain text on purpose — never HTML — so nothing in a
 * contract can carry markup into the page. The shape it does carry is
 * minimal: "## " opens a heading, a leading clause number or bullet marks
 * an indented item, and everything else is a paragraph.
 */
export type AgreementBlock =
  | { kind: "heading"; text: string }
  | { kind: "clause"; text: string }
  | { kind: "paragraph"; text: string };

export function parseAgreement(body: string): AgreementBlock[] {
  return body
    .split("\n\n")
    .map((chunk) => chunk.trim())
    .filter(Boolean)
    .map((chunk) => {
      if (chunk.startsWith("## ")) {
        return { kind: "heading", text: chunk.slice(3) } as const;
      }
      if (chunk.startsWith("# ")) {
        return { kind: "heading", text: chunk.slice(2) } as const;
      }
      if (/^\d+\./.test(chunk) || /^[-*]\s/.test(chunk)) {
        return { kind: "clause", text: chunk } as const;
      }
      return { kind: "paragraph", text: chunk } as const;
    });
}
