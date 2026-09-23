"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ExternalLink } from "lucide-react";
import { useAuth } from "@/lib/auth";
import { clientsApi, type ClientRecord } from "@/lib/clients";
import { cn } from "@/lib/cn";

/**
 * ClientFilePanel — the client's record, read-only, inside the session room.
 *
 * Opening the full file used to mean navigating away from /sessions/[id]/room,
 * which tears down the peer connection and drops the call. The practitioner
 * needs the history in front of them *during* the consultation, so the facts
 * that get referred to mid-session are fetched and shown here instead, and the
 * editable file stays one click away in a new tab.
 */

const STATUS_LABEL: Record<string, string> = {
  pending_payment: "Payment required",
  confirmed: "Confirmed",
  cancelled: "Cancelled",
  completed: "Completed",
  no_show: "No-show",
};

function formatWhen(iso: string, timeZone: string): string {
  return new Date(iso).toLocaleString("en-GB", {
    timeZone,
    timeZoneName: "short",
    day: "numeric",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function ClientFilePanel({ clientId }: { clientId: string }) {
  const { session, refreshCallbacks, user } = useAuth();
  const [record, setRecord] = useState<ClientRecord | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    if (!session) return;
    let cancelled = false;
    clientsApi
      .get(session, refreshCallbacks, clientId)
      .then((value) => {
        if (cancelled) return;
        setRecord(value);
        setFailed(false);
      })
      .catch(() => {
        if (!cancelled) setFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [clientId, refreshCallbacks, session]);

  if (failed) {
    return (
      <div className="px-1 py-6 text-center">
        <p className="text-sm text-ink-muted">
          The client file could not be loaded. The call is unaffected.
        </p>
      </div>
    );
  }

  if (!record) {
    return (
      <div
        className="flex flex-col gap-2 py-2"
        aria-label="Loading the client file"
      >
        {[0, 1, 2, 3].map((row) => (
          <div
            key={row}
            className="h-12 animate-pulse rounded-lg bg-surface-sunken"
          />
        ))}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <div>
        <p className="text-sm font-semibold text-ink">{record.name}</p>
        <p className="mt-0.5 text-xs break-all text-ink-muted">
          {record.email}
        </p>
        {record.phone ? (
          <p className="mt-0.5 text-xs text-ink-muted">{record.phone}</p>
        ) : null}
      </div>

      {record.tags.length > 0 ? (
        <div className="flex flex-wrap gap-1.5">
          {record.tags.map((tag) => (
            <span
              key={tag}
              className="rounded-full bg-surface-sunken px-2.5 py-1 text-[11px] font-medium text-ink-muted"
            >
              {tag}
            </span>
          ))}
        </div>
      ) : null}

      <dl className="grid grid-cols-2 gap-2">
        {[
          ["Sessions", String(record.recentBookings.length)],
          ["Documents", String(record.documentCount)],
          ["Forms", String(record.formSubmissionCount)],
          ["Payments", String(record.payments.paymentCount)],
        ].map(([label, value]) => (
          <div key={label} className="rounded-lg bg-surface-sunken px-3 py-2">
            <dt className="text-[11px] text-ink-muted">{label}</dt>
            <dd className="text-sm font-semibold text-ink">{value}</dd>
          </div>
        ))}
      </dl>

      {record.practiceNotes ? (
        <div>
          <h3 className="text-xs font-semibold tracking-[0.04em] text-ink-muted uppercase">
            Private summary
          </h3>
          <p className="mt-1.5 text-sm leading-relaxed whitespace-pre-wrap text-ink">
            {record.practiceNotes}
          </p>
        </div>
      ) : null}

      <div>
        <h3 className="text-xs font-semibold tracking-[0.04em] text-ink-muted uppercase">
          Recent sessions
        </h3>
        {record.recentBookings.length === 0 ? (
          <p className="mt-1.5 text-sm text-ink-muted">
            No sessions on file yet.
          </p>
        ) : (
          <ul className="mt-1.5 flex flex-col gap-1.5">
            {record.recentBookings.slice(0, 8).map((booking) => (
              <li
                key={booking.id}
                className="flex items-center justify-between gap-2 rounded-lg bg-surface-sunken px-3 py-2"
              >
                <span className="text-xs text-ink">
                  {formatWhen(booking.startAt, user?.timezone ?? "UTC")}
                </span>
                <span
                  className={cn(
                    "shrink-0 text-[11px] font-medium",
                    booking.status === "cancelled" ||
                      booking.status === "no_show"
                      ? "text-danger-ink"
                      : "text-ink-muted",
                  )}
                >
                  {STATUS_LABEL[booking.status] ?? booking.status}
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>

      {/* New tab, deliberately: this is the one link here that must never
          unmount the room and end the call. */}
      <Link
        href={`/clients/${clientId}`}
        target="_blank"
        rel="noopener"
        className="inline-flex items-center gap-2 text-sm font-semibold text-primary hover:text-primary-hover"
      >
        Open the full file in a new tab
        <ExternalLink size={14} aria-hidden="true" />
      </Link>
    </div>
  );
}
