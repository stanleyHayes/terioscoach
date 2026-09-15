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
  const [downloading, setDownloading] = useState(false);
  const [downloadError, setDownloadError] = useState<string | null>(null);

  async function download() {
    if (downloading) return;
    setDownloading(true);
    setDownloadError(null);
    try {
      // Cross-origin anchors can ignore "download". Fetch the private rendition
      // and download a same-origin blob instead of navigating to the storage API.
      const response = await fetch(url, { credentials: "omit", cache: "no-store" });
      if (!response.ok) throw new Error("Download failed");
      const blob = await response.blob();
      if (!blob.size || !blob.type.startsWith("video/")) throw new Error("Invalid recording");
      const objectURL = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = objectURL;
      anchor.download = fileName;
      document.body.appendChild(anchor);
      anchor.click();
      anchor.remove();
      // Allow browsers time to begin consuming the blob before releasing it.
      setTimeout(() => URL.revokeObjectURL(objectURL), 60_000);
    } catch {
      setDownloadError("The recording could not be downloaded. Refresh the page for a new private link, then try again.");
    } finally {
      setDownloading(false);
    }
  }

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
      <button
        type="button"
        onClick={() => void download()}
        disabled={downloading}
        aria-busy={downloading}
        className="inline-flex min-h-10 items-center gap-2 rounded-full border border-border-strong px-4 text-sm font-semibold text-primary transition-colors hover:bg-eucalyptus-100"
      >
        <Download size={16} aria-hidden="true" />
        {downloading ? "Downloading recording…" : "Download recording"}
      </button>
      {downloadError ? <p role="alert" className="text-sm text-danger-ink">{downloadError}</p> : null}
    </div>
  );
}
