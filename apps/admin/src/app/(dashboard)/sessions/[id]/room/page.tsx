"use client";

import Link from "next/link";
import { useParams, useSearchParams } from "next/navigation";
import { ArrowLeft } from "lucide-react";
import { VideoRoom } from "@/components/schedule/VideoRoom";

/**
 * The practitioner's video room for one session (CX-06).
 *
 * The API decides who may be here and when; this page renders the result.
 * A link back to the client's file is offered alongside, because notes are
 * usually written the moment a session ends and hunting for the client
 * afterwards is the friction that stops that happening.
 */
export default function SessionRoomPage() {
  const params = useParams<{ id: string }>();
  const search = useSearchParams();
  const clientId = search.get("client");
  const clientName = search.get("clientName") || "your client";

  return (
    <div data-admin-page="session-room" className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <Link
          href="/calendar"
          className="inline-flex items-center gap-2 text-sm font-medium text-ink-muted transition-colors duration-instant ease-out hover:text-ink"
        >
          <ArrowLeft size={16} aria-hidden="true" />
          Back to the calendar
        </Link>

        {clientId ? (
          <Link
            href={`/clients/${clientId}`}
            className="text-sm font-medium text-primary transition-colors duration-instant ease-out hover:text-primary-hover"
          >
            Open the client file
          </Link>
        ) : null}
      </div>

      <h1 className="font-display text-[1.75rem] leading-[1.2] font-medium tracking-[-0.01em] text-ink">
        Session room
      </h1>

      <VideoRoom bookingId={params.id} peerLabel={clientName} />
    </div>
  );
}
