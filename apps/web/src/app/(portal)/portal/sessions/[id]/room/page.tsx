"use client";

import { ArrowLeft } from "lucide-react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { VideoRoom } from "@/components/portal/VideoRoom";

/**
 * The client's video room for one session (CX-05).
 *
 * The mirror of the practitioner's room in apps/admin. The API decides who
 * may be here and when — ownership, and the ten-minutes-before to
 * fifteen-after window — so this page only renders the result. A client who
 * arrives too early, too late, or at somebody else's session is told so by
 * `VideoRoom` from the API's own answer rather than by a guess made here.
 *
 * Leaving returns them to their sessions list rather than dropping them on
 * a dead page: the session is over, and what they want next is their
 * upcoming ones.
 */
export default function ClientSessionRoomPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();

  return (
    <div data-portal-page="session-room" className="flex flex-col gap-8">
      <Link
        href="/portal/sessions"
        className="inline-flex items-center gap-2 self-start text-sm font-medium text-ink-muted transition-colors duration-instant ease-out hover:text-ink"
      >
        <ArrowLeft size={16} aria-hidden="true" />
        Back to your sessions
      </Link>

      <h1 className="font-display text-[1.75rem] leading-[1.2] font-medium tracking-[-0.01em] text-ink">
        Your session
      </h1>

      <VideoRoom
        bookingId={params.id}
        peerLabel="your practitioner"
        onLeave={() => router.push("/portal/sessions")}
      />
    </div>
  );
}
