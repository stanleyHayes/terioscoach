"use client";

import { CircleAlert, Eye, EyeOff, Send } from "lucide-react";
import { useEffect, useState } from "react";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { TextArea } from "@/components/ui/TextArea";
import { ApiError } from "@/lib/api";
import { cn } from "@/lib/cn";
import { notesApi, type SessionNote } from "@/lib/clients";
import { useAction } from "@/lib/use-resource";

/**
 * Session notes composer (ADM-04).
 *
 * The whole point of this screen is the line between the two halves. What
 * goes in "private" is never sent anywhere and never visible to the client;
 * what goes in "shared" is invisible too — until Share is pressed, once,
 * deliberately. The UI makes that boundary the loudest thing on the page,
 * because a note written into the wrong box is not a bug the client can
 * report, it is a confidence lost.
 *
 * Sharing is one-way by design: the API has no unshare, so the button says
 * so before it is pressed rather than after.
 */
export interface NotesComposerProps {
  bookingId: string;
  /** The stored note, or null when nothing has been written yet. */
  note: SessionNote | null;
  onSaved: (note: SessionNote) => void;
}

export function NotesComposer({ bookingId, note, onSaved }: NotesComposerProps) {
  const [privateNotes, setPrivateNotes] = useState(note?.privateNotes ?? "");
  const [sharedFeedback, setSharedFeedback] = useState(note?.sharedFeedback ?? "");
  const [resources, setResources] = useState((note?.sharedResources ?? []).join("\n"));
  const [confirmingShare, setConfirmingShare] = useState(false);
  const action = useAction();

  // A different booking's note replaces the fields wholesale; without this
  // the composer would keep showing the previous session's notes.
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setPrivateNotes(note?.privateNotes ?? "");
    setSharedFeedback(note?.sharedFeedback ?? "");
    setResources((note?.sharedResources ?? []).join("\n"));
    setConfirmingShare(false);
  }, [note, bookingId]);

  const shared = Boolean(note?.sharedAt);
  const dirty =
    privateNotes !== (note?.privateNotes ?? "") ||
    sharedFeedback !== (note?.sharedFeedback ?? "") ||
    resources !== (note?.sharedResources ?? []).join("\n");

  async function save() {
    const saved = await action.run("save", (session, callbacks) =>
      notesApi.save(session, callbacks, bookingId, {
        privateNotes,
        sharedFeedback,
        sharedResources: parseResources(resources),
      }),
    );
    if (saved) onSaved(saved);
  }

  async function share() {
    // Save first: sharing publishes what is stored, not what is on screen,
    // and a practitioner who typed then pressed Share means both.
    if (dirty) {
      const saved = await action.run("save", (session, callbacks) =>
        notesApi.save(session, callbacks, bookingId, {
          privateNotes,
          sharedFeedback,
          sharedResources: parseResources(resources),
        }),
      );
      if (!saved) return;
      onSaved(saved);
    }
    const result = await action.run("share", (session, callbacks) =>
      notesApi.share(session, callbacks, bookingId),
    );
    if (result) {
      onSaved(result);
      setConfirmingShare(false);
    }
  }

  return (
    <div className="flex flex-col gap-6">
      {action.error ? (
        <div
          role="alert"
          className="flex items-start gap-3 rounded-lg border border-danger-bg bg-danger-bg px-4 py-3"
        >
          <CircleAlert size={16} aria-hidden="true" className="mt-0.5 shrink-0 text-danger-ink" />
          <p className="text-sm text-danger-ink">{action.error}</p>
        </div>
      ) : null}

      {/* Private half. */}
      <section
        aria-labelledby="private-notes-heading"
        className="rounded-lg border border-border bg-surface-raised p-5"
      >
        <div className="flex flex-wrap items-center gap-3">
          <EyeOff size={16} aria-hidden="true" className="text-ink-muted" />
          <h3 id="private-notes-heading" className="text-base font-semibold text-ink">
            Private notes
          </h3>
          <Badge variant="neutral" dot={false}>
            Never shared
          </Badge>
        </div>
        <p className="mt-1.5 text-[13px] leading-[1.5] text-ink-muted">
          Your own record of the session. The client cannot see this, before
          or after sharing.
        </p>
        <TextArea
          label="Private notes"
          labelHidden
          rows={7}
          value={privateNotes}
          onChange={(event) => setPrivateNotes(event.target.value)}
          className="mt-3"
          placeholder="Observations, treatment detail, anything you need next time."
        />
      </section>

      {/* Shared half. */}
      <section
        aria-labelledby="shared-feedback-heading"
        className={cn(
          "rounded-lg border p-5",
          shared ? "border-success bg-success-bg/30" : "border-border bg-surface-raised",
        )}
      >
        <div className="flex flex-wrap items-center gap-3">
          <Eye size={16} aria-hidden="true" className="text-ink-muted" />
          <h3 id="shared-feedback-heading" className="text-base font-semibold text-ink">
            Feedback for the client
          </h3>
          {shared ? (
            <Badge variant="success">Shared</Badge>
          ) : (
            <Badge variant="warning">Not shared yet</Badge>
          )}
        </div>
        <p className="mt-1.5 text-[13px] leading-[1.5] text-ink-muted">
          {shared
            ? "This is visible in the client's portal. Edits appear there immediately."
            : "Invisible to the client until you share it."}
        </p>

        <TextArea
          label="Feedback"
          labelHidden
          rows={6}
          value={sharedFeedback}
          onChange={(event) => setSharedFeedback(event.target.value)}
          className="mt-3"
          placeholder="What went well, what to do between now and next time."
        />

        <div className="mt-4">
          <TextArea
            label="Resources — one link per line"
            rows={3}
            value={resources}
            onChange={(event) => setResources(event.target.value)}
            hint="Aftercare, exercises, reading. Optional."
          />
        </div>
      </section>

      <div className="flex flex-wrap items-center gap-3">
        <Button
          variant="secondary"
          disabled={!dirty || action.pending !== null}
          onClick={() => void save()}
        >
          Save
        </Button>

        {shared ? (
          <p className="text-[13px] text-ink-muted">
            Shared{" "}
            <time dateTime={note?.sharedAt}>
              {new Date(note!.sharedAt!).toLocaleDateString("en-GB", {
                day: "numeric",
                month: "short",
                year: "numeric",
              })}
            </time>
            . Sharing cannot be undone.
          </p>
        ) : confirmingShare ? (
          <div className="flex flex-wrap items-center gap-3">
            <p className="text-[13px] text-ink">
              Share this feedback with the client? This cannot be undone.
            </p>
            <Button disabled={action.pending !== null} onClick={() => void share()}>
              Yes, share it
            </Button>
            <Button
              variant="ghost"
              disabled={action.pending !== null}
              onClick={() => setConfirmingShare(false)}
            >
              Not yet
            </Button>
          </div>
        ) : (
          <Button
            disabled={sharedFeedback.trim() === "" || action.pending !== null}
            onClick={() => setConfirmingShare(true)}
          >
            <Send size={16} aria-hidden="true" className="mr-2" />
            Share with client
          </Button>
        )}
      </div>
    </div>
  );
}

/** One resource per line, blanks dropped. */
export function parseResources(value: string): string[] {
  return value
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);
}

/** True when a load failure means "no note yet" rather than a real problem —
 * a session with nothing written is the normal starting state. */
export function isMissingNote(error: unknown): boolean {
  return error instanceof ApiError && error.status === 404;
}
