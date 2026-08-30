"use client";

import { CircleAlert, Mail, Trash2 } from "lucide-react";
import { useState } from "react";
import { Badge, type BadgeVariant } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { EmptyState } from "@/components/content/states";
import { IconButton } from "@/components/ui/IconButton";
import { cn } from "@/lib/cn";
import {
  ENQUIRY_STATUSES,
  enquiriesApi,
  type Enquiry,
  type EnquiryStatus,
} from "@/lib/inbox";
import { useAction, useResource } from "@/lib/use-resource";

/**
 * Enquiry inbox (ADM-09).
 *
 * A triage list, not a mailbox: every state is reachable from every other,
 * because "replied" set in error should be undoable and an archived message
 * that turns out to matter should come back. The unread count is derived
 * from the list on screen rather than fetched separately, so the badge and
 * the rows can never disagree.
 */

const statusTone: Record<EnquiryStatus, BadgeVariant> = {
  new: "info",
  read: "neutral",
  replied: "success",
  archived: "neutral",
};

const statusLabel: Record<EnquiryStatus, string> = {
  new: "New",
  read: "Read",
  replied: "Replied",
  archived: "Archived",
};

export default function EnquiriesPage() {
  const [filter, setFilter] = useState<EnquiryStatus | "all">("all");
  const [openId, setOpenId] = useState<string | null>(null);

  const enquiries = useResource<Enquiry[]>(
    (session, callbacks) => enquiriesApi.list(session, callbacks),
    [],
  );
  const action = useAction();

  const items = enquiries.data ?? [];
  const visible =
    filter === "all" ? items : items.filter((e) => e.status === filter);
  const unread = items.filter((e) => e.status === "new").length;

  /** Applies a triage change, replacing the row in place so the list does
   * not jump back to a skeleton for a one-field write. */
  async function setStatus(enquiry: Enquiry, status: EnquiryStatus) {
    const updated = await action.run(enquiry.id, (session, callbacks) =>
      enquiriesApi.setStatus(session, callbacks, enquiry.id, status),
    );
    if (updated) {
      // Functional update: a delete may have landed while this write was in
      // flight, and mapping over a stale snapshot would bring the row back.
      enquiries.set((current) =>
        (current ?? []).map((e) => (e.id === updated.id ? updated : e)),
      );
    }
  }

  async function remove(enquiry: Enquiry) {
    const done = await action.run(enquiry.id, (session, callbacks) =>
      enquiriesApi.remove(session, callbacks, enquiry.id).then(() => true),
    );
    if (done) {
      enquiries.set((current) =>
        (current ?? []).filter((e) => e.id !== enquiry.id),
      );
      setOpenId((current) => (current === enquiry.id ? null : current));
    }
  }

  /** Opening an enquiry marks it read — the practitioner has now seen it,
   * which is exactly what "read" records. */
  function open(enquiry: Enquiry) {
    const next = openId === enquiry.id ? null : enquiry.id;
    setOpenId(next);
    if (next && enquiry.status === "new") {
      void setStatus(enquiry, "read");
    }
  }

  return (
    <div data-admin-page="enquiries" className="flex flex-col gap-6">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="font-display text-[1.75rem] leading-[1.2] font-medium tracking-[-0.01em] text-ink">
            Enquiries
          </h1>
          <p className="mt-1.5 text-sm text-ink-muted">
            {unread > 0
              ? `${unread} waiting for a reply`
              : "Everything here has been seen"}
          </p>
        </div>

        <div
          className="flex flex-wrap gap-2"
          role="group"
          aria-label="Filter by status"
        >
          {(["all", ...ENQUIRY_STATUSES] as const).map((value) => (
            <button
              key={value}
              type="button"
              aria-pressed={filter === value}
              onClick={() => setFilter(value)}
              className={cn(
                "rounded-full px-3 py-1.5 text-[13px] font-medium transition-colors duration-instant ease-out",
                filter === value
                  ? "bg-primary text-on-primary"
                  : "bg-surface-sunken text-ink-muted hover:text-ink",
              )}
            >
              {value === "all" ? "All" : statusLabel[value]}
            </button>
          ))}
        </div>
      </header>

      {action.error ? (
        <div
          role="alert"
          className="flex items-start gap-3 rounded-lg border border-danger-bg bg-danger-bg px-4 py-3"
        >
          <CircleAlert
            size={16}
            aria-hidden="true"
            className="mt-0.5 shrink-0 text-danger-ink"
          />
          <p className="text-sm text-danger-ink">{action.error}</p>
        </div>
      ) : null}

      {enquiries.error ? (
        <div
          role="alert"
          className="rounded-lg border border-border bg-surface-raised p-8 text-center"
        >
          <p className="text-sm text-ink-muted">{enquiries.error}</p>
          <div className="mt-4">
            <Button variant="secondary" size="sm" onClick={enquiries.refresh}>
              Try again
            </Button>
          </div>
        </div>
      ) : enquiries.data === null ? (
        <div role="status" aria-busy="true" className="flex flex-col gap-3">
          <span className="sr-only">Loading enquiries…</span>
          {[0, 1, 2].map((i) => (
            <span
              key={i}
              aria-hidden="true"
              className="h-20 rounded-lg bg-surface-sunken"
            />
          ))}
        </div>
      ) : visible.length === 0 ? (
        <EmptyState
          icon={<Mail size={26} />}
          title={
            filter === "all"
              ? "No enquiries yet"
              : `Nothing marked ${statusLabel[filter].toLowerCase()}`
          }
          body={
            filter === "all"
              ? "Messages from the website's contact form arrive here, and you are emailed when one does."
              : "Try another filter to see the rest of the inbox."
          }
        />
      ) : (
        <ul className="flex flex-col gap-3">
          {visible.map((enquiry) => {
            const expanded = openId === enquiry.id;
            const busy = action.pending === enquiry.id;
            return (
              <li
                key={enquiry.id}
                className="rounded-lg border border-border bg-surface-raised"
              >
                <button
                  type="button"
                  aria-expanded={expanded}
                  aria-controls={`enquiry-${enquiry.id}`}
                  onClick={() => open(enquiry)}
                  className="flex w-full items-start justify-between gap-4 p-5 text-left"
                >
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-3">
                      <span className="text-base font-semibold text-ink">
                        {enquiry.name}
                      </span>
                      <Badge variant={statusTone[enquiry.status]}>
                        {statusLabel[enquiry.status]}
                      </Badge>
                    </div>
                    <p className="mt-1 truncate text-sm text-ink-muted">
                      {enquiry.subject || enquiry.message}
                    </p>
                  </div>
                  <time
                    dateTime={enquiry.createdAt}
                    className="shrink-0 text-[13px] tabular-nums text-ink-faint"
                  >
                    {new Date(enquiry.createdAt).toLocaleDateString("en-GB", {
                      day: "numeric",
                      month: "short",
                    })}
                  </time>
                </button>

                <div
                  id={`enquiry-${enquiry.id}`}
                  hidden={!expanded}
                  className="border-t border-border p-5"
                >
                  <dl className="grid gap-3 sm:grid-cols-2">
                    <div>
                      <dt className="text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-muted">
                        Email
                      </dt>
                      <dd className="mt-1 text-sm text-ink">
                        <a
                          href={`mailto:${enquiry.email}`}
                          className="text-primary transition-colors duration-instant ease-out hover:text-primary-hover"
                        >
                          {enquiry.email}
                        </a>
                      </dd>
                    </div>
                    {enquiry.phone ? (
                      <div>
                        <dt className="text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-muted">
                          Phone
                        </dt>
                        <dd className="mt-1 text-sm text-ink">
                          {enquiry.phone}
                        </dd>
                      </div>
                    ) : null}
                  </dl>

                  <p className="mt-4 max-w-[68ch] text-sm leading-[1.6] whitespace-pre-line text-ink">
                    {enquiry.message}
                  </p>

                  <div className="mt-6 flex flex-wrap items-center gap-2">
                    {ENQUIRY_STATUSES.filter(
                      (status) => status !== enquiry.status,
                    ).map((status) => (
                      <Button
                        key={status}
                        variant="secondary"
                        size="sm"
                        disabled={busy}
                        onClick={() => void setStatus(enquiry, status)}
                      >
                        Mark {statusLabel[status].toLowerCase()}
                      </Button>
                    ))}
                    <span className="flex-1" />
                    <IconButton
                      aria-label={`Delete the enquiry from ${enquiry.name}`}
                      disabled={busy}
                      onClick={() => void remove(enquiry)}
                    >
                      <Trash2 size={16} aria-hidden="true" />
                    </IconButton>
                  </div>
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
