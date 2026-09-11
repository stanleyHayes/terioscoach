"use client";

import { useState } from "react";
import { Download } from "lucide-react";

/**
 * RecordingPlayer — a saved session recording, with an honest fallback.
 *
 * Recordings are muxed by whichever browser the practitioner ran the call
 * in. A Chrome recording is WebM/VP9, which Safari — every iPhone — cannot
 * decode: the client used to get a black player with a struck-through play
 * button and no explanation.
 *
 * The element is asked rather than guessed at: it is rendered, and its own
 * `error` event swaps in the download. That catches every reason a file
 * will not play, not just a MIME type the browser declines up front.
 */
export function RecordingPlayer({
  url,
  contentType,
  fileName,
  className,
}: {
  url: string;
  contentType: string;
  fileName: string;
  className?: string;
}) {
  const [failed, setFailed] = useState(false);

  if (!failed) {
    return (
      <video
        controls
        preload="metadata"
        src={url}
        className={className}
        onError={() => setFailed(true)}
      />
    );
  }

  return (
    <div className="flex flex-col items-start gap-3 rounded-lg border border-border bg-surface-sunken p-4">
      <p className="text-sm leading-relaxed text-ink-muted">
        This recording was saved in a format this browser cannot play
        {contentType.includes("webm") ? ", which Safari does not support" : ""}.
        Download it and open it in another player.
      </p>
      <a
        href={url}
        download={fileName}
        className="inline-flex h-10 items-center gap-2 rounded-full bg-primary px-4 text-sm font-semibold text-on-primary transition-colors hover:bg-primary-hover"
      >
        <Download size={16} aria-hidden="true" />
        Download recording
      </a>
    </div>
  );
}
