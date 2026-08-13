"use client";

import { ChevronDown, MessageSquareText } from "lucide-react";
import { useState } from "react";
import { ApiError } from "@/lib/api";
import { cn } from "@/lib/cn";
import { notesApi, type SharedNote } from "@/lib/portal";
import { usePortalAction } from "@/lib/use-portal-data";

/**
 * Shared feedback and resources for one past session (CX-08).
 *
 * The API returns 404 when nothing has been shared — deliberately
 * indistinguishable from "no note exists", so a client can never infer that
 * their practitioner wrote something and kept it back. This component
 * treats that 404 as the ordinary "nothing yet" case and says so plainly,
 * which is the only honest reading available to it.
 *
 * Feedback is fetched when the row is opened rather than with the list:
 * most past sessions are never expanded, and a client's notes are not
 * something to prefetch in bulk.
 */
export interface SessionFeedbackProps {
  bookingId: string;
}

type State = "closed" | "loading" | "ready" | "none" | "error";

export function SessionFeedback({ bookingId }: SessionFeedbackProps) {
  const [state, setState] = useState<State>("closed");
  const [note, setNote] = useState<SharedNote | null>(null);
  const action = usePortalAction();

  async function toggle() {
    if (state !== "closed") {
      setState("closed");
      return;
    }
    setState("loading");

    const loaded = await action.run(bookingId, async (session, callbacks) => {
      try {
        return await notesApi.getShared(session, callbacks, bookingId);
      } catch (error) {
        // Nothing shared yet is the normal state, not a failure.
        if (error instanceof ApiError && error.status === 404) return null;
        throw error;
      }
    });

    if (loaded === undefined) {
      setState("error");
      return;
    }
    setNote(loaded);
    setState(loaded ? "ready" : "none");
  }

  const open = state !== "closed";

  return (
    <div className="border-t border-border">
      <button
        type="button"
        aria-expanded={open}
        aria-controls={`feedback-${bookingId}`}
        onClick={() => void toggle()}
        className="flex w-full items-center justify-between gap-3 px-5 py-3 text-left text-sm font-medium text-ink-muted transition-colors duration-instant ease-out hover:text-ink"
      >
        <span className="flex items-center gap-2">
          <MessageSquareText size={16} aria-hidden="true" />
          Feedback from this session
        </span>
        <ChevronDown
          size={16}
          aria-hidden="true"
          className={cn("transition-transform duration-fast ease-out", open && "rotate-180")}
        />
      </button>

      <div id={`feedback-${bookingId}`} hidden={!open} className="px-5 pb-5">
        {state === "loading" ? (
          <p role="status" className="text-sm text-ink-muted">
            Loading…
          </p>
        ) : state === "error" ? (
          <p role="alert" className="text-sm text-danger-ink">
            {action.error ?? "That didn't load. Try again in a moment."}
          </p>
        ) : state === "none" ? (
          <p className="text-sm leading-[1.55] text-ink-muted">
            Nothing has been shared for this session yet. If your practitioner
            adds feedback, it appears here.
          </p>
        ) : note ? (
          <div className="flex flex-col gap-4">
            {note.sharedFeedback ? (
              <p className="max-w-[68ch] text-base leading-[1.65] whitespace-pre-line text-ink">
                {note.sharedFeedback}
              </p>
            ) : null}

            {note.sharedResources.length > 0 ? (
              <div>
                <h3 className="text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-muted">
                  Resources
                </h3>
                <ul className="mt-2 flex flex-col gap-1.5">
                  {note.sharedResources.map((resource) => (
                    <li key={resource}>
                      <a
                        href={resource}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-sm text-primary transition-colors duration-instant ease-out hover:text-primary-hover"
                      >
                        {resource}
                      </a>
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}

            <p className="text-[13px] text-ink-faint">
              Shared{" "}
              <time dateTime={note.sharedAt}>
                {new Date(note.sharedAt).toLocaleDateString("en-GB", {
                  day: "numeric",
                  month: "short",
                  year: "numeric",
                })}
              </time>
            </p>
          </div>
        ) : null}
      </div>
    </div>
  );
}
