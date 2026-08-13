"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { ArrowLeft } from "lucide-react";
import { PortalPage } from "@/components/portal/PortalPage";
import { VideoRoom } from "@/components/portal/VideoRoom";

/**
 * The client's video room for one session (CX-05).
 *
 * The page is thin on purpose: every rule about who may be here, and when,
 * lives in the API and is enforced before a socket exists. This screen's
 * job is to render whatever the room hook reports — including the refusals,
 * in words a client can act on.
 */
export default function SessionRoomPage() {
  const params = useParams<{ id: string }>();

  return (
    <PortalPage title="Your session">
      <Link
        href="/portal/sessions"
        className="inline-flex w-fit items-center gap-2 text-sm font-medium text-ink-muted transition-colors duration-instant ease-out hover:text-ink"
      >
        <ArrowLeft size={16} aria-hidden="true" />
        Back to your sessions
      </Link>

      <VideoRoom bookingId={params.id} peerLabel="your practitioner" />
    </PortalPage>
  );
}
