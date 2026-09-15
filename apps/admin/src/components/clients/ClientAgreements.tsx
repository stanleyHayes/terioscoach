"use client";

import { useEffect, useState } from "react";
import { FileCheck2, FileDown, PenTool } from "lucide-react";
import { API_BASE_URL } from "@/lib/api";
import {
  agreementsApi,
  signedAgreementPath,
  type AgreementSignature,
} from "@/lib/agreements";
import { useAuth } from "@/lib/auth";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { TextInput } from "@/components/ui/TextInput";

/**
 * ClientAgreements — which service agreements this client has signed.
 *
 * The row that matters is "signed as": the name the client actually typed,
 * shown next to the name on the account, because the two are allowed to
 * differ and the difference is the kind of thing a practice needs to be
 * able to see rather than have normalised away.
 *
 * Supports practitioner countersignatures when required.
 *
 * The PDF is rendered on demand from the signature, against the wording as
 * it stood when it was signed — so an agreement edited since still prints
 * what that client agreed to.
 */
export function ClientAgreements({ clientId }: { clientId: string }) {
  const { session, user, refreshCallbacks } = useAuth();
  const [signatures, setSignatures] = useState<AgreementSignature[] | null>(null);
  const [downloading, setDownloading] = useState<string | null>(null);
  const [countersignTarget, setCountersignTarget] = useState<AgreementSignature | null>(null);
  const [practitionerName, setPractitionerName] = useState("");
  const [countersigning, setCountersigning] = useState(false);
  const [countersignError, setCountersignError] = useState<string | null>(null);

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

  function startCountersigning(sig: AgreementSignature) {
    setCountersignTarget(sig);
    setPractitionerName(user?.name || "Stanley Hayes");
    setCountersignError(null);
  }

  async function handleCountersignSubmit() {
    if (!session || !countersignTarget || !practitionerName.trim()) return;
    setCountersigning(true);
    setCountersignError(null);
    try {
      const updated = await agreementsApi.countersign(
        session,
        refreshCallbacks,
        countersignTarget.id,
        practitionerName.trim(),
      );
      setSignatures((prev) =>
        prev
          ? prev.map((item) => (item.id === updated.id ? updated : item))
          : [updated],
      );
      setCountersignTarget(null);
    } catch (err: unknown) {
      setCountersignError(
        err instanceof Error ? err.message : "Failed to countersign agreement.",
      );
    } finally {
      setCountersigning(false);
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
    <>
      <ul className="flex flex-col gap-3">
        {signatures.map((signature) => {
          const isPendingCountersign =
            signature.requiresCountersignature && !signature.practitionerSignedAt;
          const isCountersigned = Boolean(
            signature.requiresCountersignature && signature.practitionerSignedAt,
          );

          return (
            <li key={signature.id} className="rounded-lg bg-surface-sunken p-3.5 border border-border/60">
              <div className="flex items-start gap-2.5">
                <FileCheck2
                  size={16}
                  aria-hidden="true"
                  className="mt-0.5 shrink-0 text-primary"
                />
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <p className="text-sm font-semibold text-ink">
                      {signature.agreementTitle}
                    </p>
                    {isPendingCountersign ? (
                      <Badge variant="warning">Countersignature pending</Badge>
                    ) : isCountersigned ? (
                      <Badge variant="success">Countersigned</Badge>
                    ) : (
                      <Badge variant="neutral">Signed</Badge>
                    )}
                  </div>

                  <div className="mt-2 text-xs text-ink-muted">
                    <span className="font-medium text-ink">Client signature:</span>{" "}
                    <span className="font-display italic text-ink">{signature.signedName}</span>
                    <span className="ml-1 text-[11px] text-ink-faint">
                      ({new Date(signature.signedAt).toLocaleString("en-US", {
                        day: "numeric",
                        month: "short",
                        year: "numeric",
                        hour: "2-digit",
                        minute: "2-digit",
                      })})
                    </span>
                  </div>

                  {signature.signedName.trim().toLowerCase() !==
                  signature.clientName.trim().toLowerCase() ? (
                    <p className="mt-0.5 text-[11px] text-ink-faint">
                      Account name: {signature.clientName}
                    </p>
                  ) : null}

                  {isCountersigned ? (
                    <div className="mt-1 text-xs text-ink-muted">
                      <span className="font-medium text-ink">Practitioner countersignature:</span>{" "}
                      <span className="font-display italic text-ink">{signature.practitionerSignedName}</span>
                      {signature.practitionerSignedAt ? (
                        <span className="ml-1 text-[11px] text-ink-faint">
                          ({new Date(signature.practitionerSignedAt).toLocaleString("en-US", {
                            day: "numeric",
                            month: "short",
                            year: "numeric",
                            hour: "2-digit",
                            minute: "2-digit",
                          })})
                        </span>
                      ) : null}
                    </div>
                  ) : null}

                  <div className="mt-3 flex flex-wrap items-center gap-2">
                    <button
                      type="button"
                      onClick={() => void openPdf(signature)}
                      disabled={downloading === signature.id}
                      className="inline-flex items-center gap-1 rounded-md bg-surface px-2.5 py-1 text-xs font-semibold text-primary border border-border hover:text-primary-hover disabled:opacity-50"
                    >
                      <FileDown size={13} aria-hidden="true" />
                      {downloading === signature.id ? "Opening…" : "Open PDF"}
                    </button>

                    {isPendingCountersign ? (
                      <Button
                        size="sm"
                        variant="primary"
                        onClick={() => startCountersigning(signature)}
                        className="text-xs h-7 px-3"
                      >
                        <PenTool size={12} className="mr-1" aria-hidden="true" />
                        Countersign
                      </Button>
                    ) : null}
                  </div>
                </div>
              </div>
            </li>
          );
        })}
      </ul>

      {countersignTarget ? (
        <Modal
          open
          onClose={() => setCountersignTarget(null)}
          title="Countersign Agreement"
          description={`Add your practitioner signature to ${countersignTarget.agreementTitle} for ${countersignTarget.clientName}.`}
          footer={
            <div className="flex justify-end gap-2">
              <Button
                variant="secondary"
                disabled={countersigning}
                onClick={() => setCountersignTarget(null)}
              >
                Cancel
              </Button>
              <Button
                variant="primary"
                loading={countersigning}
                disabled={!practitionerName.trim()}
                onClick={() => void handleCountersignSubmit()}
              >
                Sign and complete
              </Button>
            </div>
          }
        >
          <div className="flex flex-col gap-4 py-2">
            {countersignError ? (
              <p role="alert" className="text-sm text-danger-ink">
                {countersignError}
              </p>
            ) : null}
            <TextInput
              label="Practitioner signature (Full Legal Name)"
              required
              value={practitionerName}
              onChange={(e) => setPractitionerName(e.target.value)}
              placeholder="Stanley Hayes, BSN, RN"
            />
            <p className="text-xs text-ink-muted">
              By typing your name, you execute the countersignature block on this document on behalf of Terios Wellness Spa.
            </p>
          </div>
        </Modal>
      ) : null}
    </>
  );
}
