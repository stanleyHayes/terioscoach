"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { ArrowLeft, CircleAlert, FileText, FolderOpen } from "lucide-react";
import { useState } from "react";
import { NotesComposer, isMissingNote } from "@/components/clients/NotesComposer";
import { Badge, type BadgeVariant } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { TextArea } from "@/components/ui/TextArea";
import { TextInput } from "@/components/ui/TextInput";
import { cn } from "@/lib/cn";
import {
  clientsApi,
  notesApi,
  splitClientBookings,
  type ClientBooking,
  type ClientRecord,
  type SessionNote,
} from "@/lib/clients";
import { formatMoney } from "@/lib/format";
import { useAction, useResource } from "@/lib/use-resource";

/**
 * Client file (ADM-03) with the session-notes composer (ADM-04).
 *
 * One screen holds everything the practice knows about a person: who they
 * are, what they have booked, what has been paid, and the notes for each
 * session. The notes composer opens against a chosen session rather than
 * the client, because a note belongs to a visit — which is also how the API
 * stores it.
 */

const statusVariant: Record<ClientBooking["status"], BadgeVariant> = {
  confirmed: "info",
  completed: "success",
  cancelled: "neutral",
  no_show: "warning",
};

const statusLabel: Record<ClientBooking["status"], string> = {
  confirmed: "Confirmed",
  completed: "Completed",
  cancelled: "Cancelled",
  no_show: "No show",
};

export default function ClientRecordPage() {
  const params = useParams<{ id: string }>();
  const clientId = params.id;

  const [selectedBooking, setSelectedBooking] = useState<string | null>(null);
  const [note, setNote] = useState<SessionNote | null>(null);
  const [noteState, setNoteState] = useState<"idle" | "loading" | "ready" | "error">("idle");
  const [noteError, setNoteError] = useState<string | null>(null);

  const record = useResource<ClientRecord>(
    (session, callbacks) => clientsApi.get(session, callbacks, clientId),
    [clientId],
  );
  const action = useAction();

  const [phone, setPhone] = useState<string | null>(null);
  const [practiceNotes, setPracticeNotes] = useState<string | null>(null);
  const [tags, setTags] = useState<string | null>(null);

  const data = record.data;
  const { upcoming, past } = splitClientBookings(data?.recentBookings ?? []);

  /** Opens a session's notes. A 404 is the normal "nothing written yet"
   * state, not an error worth showing. */
  async function openNotes(bookingId: string) {
    if (selectedBooking === bookingId) {
      setSelectedBooking(null);
      return;
    }
    setSelectedBooking(bookingId);
    setNote(null);
    setNoteError(null);
    setNoteState("loading");

    const loaded = await action.run(`note:${bookingId}`, async (session, callbacks) => {
      try {
        return await notesApi.get(session, callbacks, bookingId);
      } catch (error) {
        if (isMissingNote(error)) return null;
        throw error;
      }
    });

    if (loaded === undefined) {
      setNoteState("error");
      setNoteError("Those notes didn't load. Try again.");
      return;
    }
    setNote(loaded);
    setNoteState("ready");
  }

  async function saveProfile() {
    if (!data) return;
    const updated = await action.run("profile", (session, callbacks) =>
      clientsApi.updateProfile(session, callbacks, clientId, {
        phone: phone ?? data.phone ?? "",
        practiceNotes: practiceNotes ?? data.practiceNotes,
        tags: (tags ?? data.tags.join(", "))
          .split(",")
          .map((tag) => tag.trim())
          .filter(Boolean),
      }),
    );
    if (updated) {
      record.set((current) =>
        current
          ? {
              ...current,
              phone: updated.phone,
              practiceNotes: updated.practiceNotes,
              tags: updated.tags,
            }
          : current,
      );
      setPhone(null);
      setPracticeNotes(null);
      setTags(null);
    }
  }

  const profileDirty = phone !== null || practiceNotes !== null || tags !== null;

  return (
    <div data-admin-page="client-record" className="flex flex-col gap-6">
      <Link
        href="/clients"
        className="inline-flex w-fit items-center gap-2 text-sm font-medium text-ink-muted transition-colors duration-instant ease-out hover:text-ink"
      >
        <ArrowLeft size={16} aria-hidden="true" />
        All clients
      </Link>

      {record.error ? (
        <div role="alert" className="rounded-lg border border-border bg-surface-raised p-8 text-center">
          <p className="text-sm text-ink-muted">{record.error}</p>
          <div className="mt-4">
            <Button variant="secondary" size="sm" onClick={record.refresh}>
              Try again
            </Button>
          </div>
        </div>
      ) : data === null ? (
        <div role="status" aria-busy="true" className="flex flex-col gap-3">
          <span className="sr-only">Loading the client file…</span>
          {[0, 1, 2].map((i) => (
            <span key={i} aria-hidden="true" className="h-24 rounded-lg bg-surface-sunken" />
          ))}
        </div>
      ) : (
        <>
          <header>
            <h1 className="font-display text-[1.75rem] leading-[1.2] font-medium tracking-[-0.01em] text-ink">
              {data.name}
            </h1>
            <p className="mt-1.5 text-sm text-ink-muted">{data.email}</p>
          </header>

          {/* Rollups. */}
          <dl className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <Stat label="Sessions" value={String(data.recentBookings.length)} />
            <Stat
              label="Paid"
              value={formatMoney(data.payments.totalPaidKobo, data.payments.currency)}
            />
            <Stat
              label="Refunded"
              value={formatMoney(data.payments.totalRefundedKobo, data.payments.currency)}
            />
            <Stat
              label="Files & forms"
              value={`${data.documentCount} · ${data.formSubmissionCount}`}
            />
          </dl>

          {action.error ? (
            <div
              role="alert"
              className="flex items-start gap-3 rounded-lg border border-danger-bg bg-danger-bg px-4 py-3"
            >
              <CircleAlert size={16} aria-hidden="true" className="mt-0.5 shrink-0 text-danger-ink" />
              <p className="text-sm text-danger-ink">{action.error}</p>
            </div>
          ) : null}

          <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
            {/* Sessions and their notes. */}
            <section aria-labelledby="sessions-heading" className="flex flex-col gap-4">
              <h2 id="sessions-heading" className="font-display text-xl font-medium text-ink">
                Sessions
              </h2>

              {data.recentBookings.length === 0 ? (
                <p className="rounded-lg border border-border bg-surface-raised p-6 text-sm text-ink-muted">
                  No sessions on file yet.
                </p>
              ) : (
                <ul className="flex flex-col gap-3">
                  {[...upcoming, ...past].map((booking) => {
                    const open = selectedBooking === booking.id;
                    return (
                      <li key={booking.id} className="rounded-lg border border-border bg-surface-raised">
                        <button
                          type="button"
                          aria-expanded={open}
                          aria-controls={`notes-${booking.id}`}
                          onClick={() => void openNotes(booking.id)}
                          className="flex w-full items-center justify-between gap-4 p-4 text-left"
                        >
                          <span className="flex flex-wrap items-center gap-3">
                            <span className="text-sm font-medium tabular-nums text-ink">
                              {new Date(booking.startAt).toLocaleString("en-GB", {
                                day: "numeric",
                                month: "short",
                                year: "numeric",
                                hour: "2-digit",
                                minute: "2-digit",
                              })}
                            </span>
                            <Badge variant={statusVariant[booking.status]}>
                              {statusLabel[booking.status]}
                            </Badge>
                            {booking.paymentStatus === "paid" ? (
                              <Badge variant="success" dot={false}>
                                Paid
                              </Badge>
                            ) : null}
                          </span>
                          <span className="text-[13px] text-ink-muted">
                            {open ? "Hide notes" : "Notes"}
                          </span>
                        </button>

                        <div id={`notes-${booking.id}`} hidden={!open} className="border-t border-border p-5">
                          {noteState === "loading" ? (
                            <p role="status" className="text-sm text-ink-muted">
                              Loading notes…
                            </p>
                          ) : noteState === "error" ? (
                            <p role="alert" className="text-sm text-danger-ink">
                              {noteError}
                            </p>
                          ) : (
                            <NotesComposer
                              bookingId={booking.id}
                              note={note}
                              onSaved={(saved) => setNote(saved)}
                            />
                          )}
                        </div>
                      </li>
                    );
                  })}
                </ul>
              )}
            </section>

            {/* Practice-side detail. */}
            <aside className="flex flex-col gap-6">
              <section
                aria-labelledby="practice-detail-heading"
                className="rounded-lg border border-border bg-surface-raised p-5"
              >
                <h2 id="practice-detail-heading" className="text-base font-semibold text-ink">
                  Practice detail
                </h2>
                <p className="mt-1 text-[13px] leading-[1.5] text-ink-muted">
                  Yours alone — the client never sees any of this.
                </p>

                <div className="mt-4 flex flex-col gap-4">
                  <TextInput
                    label="Phone"
                    value={phone ?? data.phone ?? ""}
                    onChange={(event) => setPhone(event.target.value)}
                  />
                  <TextInput
                    label="Tags"
                    hint="Comma separated"
                    value={tags ?? data.tags.join(", ")}
                    onChange={(event) => setTags(event.target.value)}
                  />
                  <TextArea
                    label="Private summary"
                    rows={6}
                    value={practiceNotes ?? data.practiceNotes}
                    onChange={(event) => setPracticeNotes(event.target.value)}
                  />
                  <Button
                    variant="secondary"
                    size="sm"
                    disabled={!profileDirty || action.pending !== null}
                    onClick={() => void saveProfile()}
                  >
                    Save detail
                  </Button>
                </div>
              </section>

              <section className="rounded-lg border border-border bg-surface-raised p-5">
                <h2 className="text-base font-semibold text-ink">Files and forms</h2>
                <ul className="mt-3 flex flex-col gap-2 text-sm text-ink-muted">
                  <li className="flex items-center gap-2">
                    <FolderOpen size={16} aria-hidden="true" className="text-ink-faint" />
                    {data.documentCount} {data.documentCount === 1 ? "document" : "documents"}
                  </li>
                  <li className="flex items-center gap-2">
                    <FileText size={16} aria-hidden="true" className="text-ink-faint" />
                    {data.formSubmissionCount}{" "}
                    {data.formSubmissionCount === 1 ? "form" : "forms"} on file
                  </li>
                </ul>
              </section>
            </aside>
          </div>
        </>
      )}
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className={cn("rounded-lg border border-border bg-surface-raised p-4")}>
      <dt className="text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-muted">
        {label}
      </dt>
      <dd className="mt-1.5 font-display text-xl font-medium tabular-nums text-ink">{value}</dd>
    </div>
  );
}
