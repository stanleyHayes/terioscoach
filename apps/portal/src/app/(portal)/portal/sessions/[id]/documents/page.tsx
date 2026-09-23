"use client";
import { use, useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useAuth } from "@/lib/auth";
import { authedRequest } from "@/lib/api";
import type { Booking, Participant } from "@/lib/bookings";
import type { AgreementStatus, StatementOfWork } from "@/lib/agreements";
import { ParticipantFields } from "@/components/booking/ParticipantFields";
import { AgreementStep } from "@/components/portal/AgreementStep";
import { StatementOfWorkStep } from "@/components/portal/StatementOfWorkStep";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";

type Status = AgreementStatus & {
  fee: string;
  guardianConsentReady: boolean;
  consentVersion?: string;
};
export default function SessionDocuments({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const { session, onTokensRefreshed } = useAuth();
  const [booking, setBooking] = useState<Booking | null>(null);
  const [participant, setParticipant] = useState<Participant>({
    name: "",
    under18: false,
    accurate: false,
  });
  const [statuses, setStatuses] = useState<Status[]>([]);
  const [readiness, setReadiness] = useState({
    ready: false,
    readinessMessage: "Checking session requirements…",
  });
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [editing, setEditing] = useState(false);
  const load = useCallback(async () => {
    if (!session) return;
    const [result, docs] = await Promise.all([
      authedRequest<{ booking: Booking }>(`/v1/bookings/${id}`, session, {
        onTokensRefreshed,
      }),
      authedRequest<{
        items: Status[];
        ready: boolean;
        readinessMessage: string;
      }>(`/v1/bookings/${id}/agreements`, session, { onTokensRefreshed }),
    ]);
    setBooking(result.booking);
    setParticipant(
      result.booking.participant ?? {
        name: "",
        under18: false,
        accurate: false,
      },
    );
    setStatuses(docs.items);
    setReadiness({
      ready: docs.ready,
      readinessMessage: docs.readinessMessage,
    });
  }, [id, session, onTokensRefreshed]);
  useEffect(() => {
    // load only sets state after the two network responses resolve.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load().catch((e) => setError(e.message));
  }, [load]);
  async function declare() {
    if (!session) return;
    setBusy(true);
    setError("");
    try {
      await authedRequest(
        `/v1/bookings/${id}/participant`,
        session,
        { onTokensRefreshed },
        { method: "PATCH", body: participant },
      );
      setEditing(false);
      await load();
    } catch (e) {
      setError(
        e instanceof Error ? e.message : "Could not save participant details.",
      );
    } finally {
      setBusy(false);
    }
  }
  const current = statuses.find((item) => !item.signed);
  async function sign(signedName: string, statementOfWork?: StatementOfWork) {
    if (!session || !current?.agreement) return;
    await authedRequest(
      `/v1/agreements/${current.agreement.id}/${statementOfWork ? "submit" : "sign"}`,
      session,
      { onTokensRefreshed },
      {
        method: "POST",
        body: {
          bookingId: id,
          agreementVersion: current.agreement.version,
          signerRole: booking?.participant?.under18 ? "guardian" : "client",
          acknowledged: true,
          consentVersion: current.consentVersion,
          signedName,
          statementOfWork,
        },
      },
    );
    await load();
  }
  const needsDeclaration =
    editing ||
    !booking?.participant ||
    Date.parse(booking.participant.appointmentAt ?? "") !==
      Date.parse(booking.startAt);
  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <h1 className="font-display text-3xl">Required session documents</h1>
      <p className="text-sm text-ink-muted">
        Checkout may proceed. Complete these documents before entering the
        session.
      </p>
      {error && (
        <p role="alert" className="text-danger-ink">
          {error}
        </p>
      )}
      {!booking ? (
        <p role="status">Loading appointment…</p>
      ) : needsDeclaration ? (
        <>
          <ParticipantFields value={participant} onChange={setParticipant} />
          <Button loading={busy} onClick={() => void declare()}>
            Confirm participant details
          </Button>
        </>
      ) : (
        <>
          <Card>
            <p>
              Participant: <strong>{booking.participant?.name}</strong> ·{" "}
              {booking.participant?.under18
                ? "Under 18 — guardian signature required"
                : "Adult"}
            </p>
            <Button variant="ghost" onClick={() => setEditing(true)}>
              Correct participant details
            </Button>
            <p className="text-xs text-ink-muted">
              Corrections require new signatures. Earlier signed evidence is
              retained.
            </p>
          </Card>
          {booking.participant?.under18 &&
          !current?.guardianConsentReady &&
          current ? (
            <Card>
              Guardian signing is awaiting the practice’s approved consent
              configuration. Contact the practice before your session.
            </Card>
          ) : current?.agreement ? (
            current.agreement.key.endsWith("_sow") ? (
              <StatementOfWorkStep
                key={`${current.agreement.id}:${current.agreement.version}:${current.consentVersion ?? ""}`}
                agreement={current.agreement}
                clientName={booking.participant?.name ?? ""}
                signerName={
                  booking.participant?.under18
                    ? booking.participant.guardianName
                    : booking.participant?.name
                }
                fee={current.fee}
                onSubmitted={(answers, name) => sign(name, answers)}
                onBack={() => window.history.back()}
              />
            ) : (
              <AgreementStep
                key={`${current.agreement.id}:${current.agreement.version}:${current.consentVersion ?? ""}`}
                agreement={current.agreement}
                clientName={
                  booking.participant?.under18
                    ? (booking.participant.guardianName ?? "")
                    : (booking.participant?.name ?? "")
                }
                onSigned={(name) => sign(name)}
                onBack={() => window.history.back()}
              />
            )
          ) : (
            <Card>
              <p>
                {booking.participant?.under18 && statuses.length === 0
                  ? "The practice must assign a consent document before this minor’s session can begin."
                  : "All assigned documents are signed."}
              </p>
              <p role="status" className="mt-2 text-sm">
                {readiness.readinessMessage}
              </p>
              {readiness.ready && (
                <Link
                  href={`/portal/sessions/${id}/room`}
                  className="text-primary underline"
                >
                  Continue to session
                </Link>
              )}
            </Card>
          )}
          <ul className="space-y-2">
            {statuses.map((item) => (
              <li key={item.agreement?.id}>
                {item.agreement?.title} — {item.signed ? "Signed" : "Required"}
              </li>
            ))}
          </ul>
        </>
      )}
      <Link href="/portal/forms" className="text-primary underline">
        Complete assigned forms
      </Link>
      <br />
      <Link href="/portal/sessions" className="text-primary underline">
        Back to sessions
      </Link>
    </div>
  );
}
