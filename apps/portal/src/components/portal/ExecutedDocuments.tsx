"use client";
import { useEffect, useState } from "react";
import { useAuth } from "@/lib/auth";
import { API_BASE_URL } from "@/lib/api";
import { agreementsApi, type AgreementSignature } from "@/lib/agreements";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
export function ExecutedDocuments() {
  const { session, onTokensRefreshed, user } = useAuth();
  const [items, setItems] = useState<AgreementSignature[] | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState("");
  useEffect(() => {
    if (!session) return;
    let cancelled = false;
    agreementsApi
      .listMine(session, { onTokensRefreshed })
      .then((items) => {
        if (!cancelled) setItems(items);
      })
      .catch((e) => {
        if (!cancelled) setError(e.message);
      });
    return () => {
      cancelled = true;
    };
  }, [session, onTokensRefreshed]);
  async function download(item: AgreementSignature) {
    if (!session) return;
    setBusy(item.id);
    setError("");
    try {
      const response = await fetch(
        `${API_BASE_URL}/v1/agreements/signatures/${item.id}/pdf`,
        { headers: { Authorization: `Bearer ${session.accessToken}` } },
      );
      if (!response.ok)
        throw new Error(
          "This document could not be downloaded. Refresh and try again.",
        );
      const url = URL.createObjectURL(await response.blob());
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = `terios-document-${item.id}.pdf`;
      anchor.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Download failed.");
    } finally {
      setBusy("");
    }
  }
  return (
    <section className="mb-8 space-y-4">
      <h2 className="font-display text-2xl">
        Completed forms & signed documents
      </h2>
      {error && (
        <p role="alert" className="text-danger-ink">
          {error}
        </p>
      )}
      {!items && !error ? (
        <p role="status">Loading signed documents…</p>
      ) : items?.length === 0 ? (
        <p className="text-sm text-ink-muted">
          Completed documents will appear here.
        </p>
      ) : (
        items?.map((item) => (
          <Card key={item.id} id={`execution-${item.id}`}>
            <h3 className="font-semibold">{item.agreementTitle}</h3>
            <p className="mt-1 text-sm">
              {item.signedName
                ? `Signed by ${item.signedName} (${item.signerRole ?? "client"})`
                : "Submitted — unsigned (legacy)"}{" "}
              · Version {item.agreementVersion}
            </p>
            {item.participantName && (
              <p className="text-sm">Participant: {item.participantName}</p>
            )}
            {item.signedName && (
              <p className="text-xs text-ink-muted">
                {new Date(item.signedAt).toLocaleString("en-US", {
                  timeZone: user?.timezone ?? "UTC",
                  timeZoneName: "short",
                })}
                {item.consentVersion ? ` · ${item.consentVersion}` : ""}
              </p>
            )}
            {item.statementOfWork && (
              <dl className="mt-3 grid grid-cols-2 gap-2 text-sm">
                <dt>Client name</dt>
                <dd>{item.statementOfWork.clientName}</dd>
                <dt>Start date</dt>
                <dd>{item.statementOfWork.effectiveDate}</dd>
                <dt>Term / package</dt>
                <dd>
                  {item.statementOfWork.package ||
                    `${item.statementOfWork.initialTermMonths} months`}
                </dd>
                <dt>Monthly fee</dt>
                <dd>{item.statementOfWork.monthlyFee}</dd>
              </dl>
            )}
            <Button
              className="mt-4"
              variant="secondary"
              loading={busy === item.id}
              onClick={() => void download(item)}
            >
              Download PDF
            </Button>
            <p className="mt-2 text-xs text-ink-muted">
              {item.archiveStatus === "pending"
                ? "Signed evidence saved. Archive copy pending; your PDF is available now."
                : "Generated from your saved document evidence."}
            </p>
          </Card>
        ))
      )}
    </section>
  );
}
