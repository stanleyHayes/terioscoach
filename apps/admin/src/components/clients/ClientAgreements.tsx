"use client";

import { useEffect, useState } from "react";
import { FileCheck2, FileDown } from "lucide-react";
import { API_BASE_URL } from "@/lib/api";
import {
  agreementsApi,
  signedAgreementPath,
  type AgreementSignature,
} from "@/lib/agreements";
import { useAuth } from "@/lib/auth";

/**
 * ClientAgreements — which service agreements this client has signed.
 *
 * The row that matters is "signed as": the name the client actually typed,
 * shown next to the name on the account, because the two are allowed to
 * differ and the difference is the kind of thing a practice needs to be
 * able to see rather than have normalised away.
 *
 * The PDF is rendered on demand from the signature, against the wording as
 * it stood when it was signed — so an agreement edited since still prints
 * what that client agreed to.
 */
export function ClientAgreements({ clientId }: { clientId: string }) {
  const { session, refreshCallbacks } = useAuth();
  const [signatures, setSignatures] = useState<AgreementSignature[] | null>(null);
  const [downloading, setDownloading] = useState<string | null>(null);

  useEffect(() => {
    if (!session) return;
    let cancelled = false;
    agreementsApi
      .signaturesForClient(session, refreshCallbacks, clientId)
      .then((items) => {
        if (!cancelled) setSignatures(items);
      })
      .catch(() => {
        if (!cancelled) setSignatures([]);
      });
    return () => {
      cancelled = true;
    };
  }, [clientId, refreshCallbacks, session]);

  // The PDF route is authenticated, so it is fetched with the access token
  // and handed to the browser as a blob; a plain link would arrive without
  // credentials and be refused.
  async function openPdf(signature: AgreementSignature) {
    if (!session) return;
    setDownloading(signature.id);
    try {
      const response = await fetch(
        `${API_BASE_URL}${signedAgreementPath(signature.id)}`,
        { headers: { Authorization: `Bearer ${session.accessToken}` } },
      );
      if (!response.ok) return;
      const url = URL.createObjectURL(await response.blob());
      window.open(url, "_blank", "noopener");
      // Revoked on a delay: revoking immediately can race the new tab.
      setTimeout(() => URL.revokeObjectURL(url), 60_000);
    } finally {
      setDownloading(null);
    }
  }

  if (signatures === null) {
    return <div className="h-10 animate-pulse rounded-lg bg-surface-sunken" />;
  }

  if (signatures.length === 0) {
    return (
      <p className="text-sm text-ink-muted">
        No agreements signed yet. The client is asked to sign when they book a
        service that needs one.
      </p>
    );
  }

  return (
    <ul className="flex flex-col gap-2">
      {signatures.map((signature) => (
        <li key={signature.id} className="rounded-lg bg-surface-sunken p-3">
          <div className="flex items-start gap-2">
            <FileCheck2
              size={16}
              aria-hidden="true"
              className="mt-0.5 shrink-0 text-primary"
            />
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium text-ink">
                {signature.agreementTitle}
              </p>
              <p className="mt-1 font-display text-base text-ink">
                {signature.signedName}
              </p>
              <p className="mt-0.5 text-[11px] text-ink-muted">
                Signed{" "}
                {new Date(signature.signedAt).toLocaleString("en-GB", {
                  day: "numeric",
                  month: "short",
                  year: "numeric",
                  hour: "2-digit",
                  minute: "2-digit",
                })}{" "}
                · version {signature.agreementVersion}
              </p>
              {signature.signedName.trim().toLowerCase() !==
              signature.clientName.trim().toLowerCase() ? (
                <p className="mt-0.5 text-[11px] text-ink-faint">
                  Account name: {signature.clientName}
                </p>
              ) : null}
              <button
                type="button"
                onClick={() => void openPdf(signature)}
                disabled={downloading === signature.id}
                className="mt-2 inline-flex items-center gap-1 text-[11px] font-semibold text-primary hover:text-primary-hover disabled:opacity-50"
              >
                <FileDown size={12} aria-hidden="true" />
                {downloading === signature.id ? "Opening…" : "Open signed PDF"}
              </button>
            </div>
          </div>
        </li>
      ))}
    </ul>
  );
}
