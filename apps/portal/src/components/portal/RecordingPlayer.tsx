"use client";

import { useState } from "react";
import { Download } from "lucide-react";

/** Plays the delivered rendition and offers a download without leaving the session. */
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
  const [failedURL, setFailedURL] = useState<string | null>(null);
  const failed = failedURL === url;

  return (
    <div className="space-y-3">
      {failed ? (
        <div role="status" className="rounded-lg border border-border bg-surface-sunken p-4 text-sm leading-relaxed text-ink-muted">
          This recording could not be loaded. Refresh the page to get a new private link, or try downloading it.
        </div>
      ) : (
        <video
          key={url}
          src={url}
          controls
          playsInline
          preload="metadata"
          className={className}
          onError={() => setFailedURL(url)}
          aria-label={`Session recording (${contentType.includes("mp4") ? "MP4" : "WebM"})`}
        />
      )}
      <a
        href={url}
        download={fileName}
        target="_blank"
        rel="noopener noreferrer"
        className="inline-flex min-h-10 items-center gap-2 rounded-full border border-border-strong px-4 text-sm font-semibold text-primary transition-colors hover:bg-eucalyptus-100"
      >
        <Download size={16} aria-hidden="true" />
        Download recording
      </a>
    </div>
  );
}
