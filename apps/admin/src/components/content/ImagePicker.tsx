"use client";

import { CircleAlert, ImagePlus, Loader2, Trash2 } from "lucide-react";
import { useRef, useState } from "react";
import { Button } from "@/components/ui/Button";
import { useAuth } from "@/lib/auth";
import { ACCEPT_ATTRIBUTE, uploadCMSImage } from "@/lib/media";
import { describe } from "@/lib/use-resource";

/**
 * Cover-image picker for the blog editor (ADM-07).
 *
 * The file input is visually hidden and driven by a real Button, because a
 * bare `<input type="file">` is a native control we can't style and the
 * design system has no place for one. It is still a genuine input — the
 * label association, keyboard activation and file dialog are the browser's,
 * not a reimplementation.
 */
export function ImagePicker({
  value,
  onChange,
  disabled = false,
}: {
  /** The current cover URL, or "" for none. */
  value: string;
  onChange: (url: string) => void;
  disabled?: boolean;
}) {
  const { session, refreshCallbacks } = useAuth();
  const inputRef = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleFile(file: File | undefined) {
    if (!file || !session) return;
    setError(null);
    setUploading(true);
    try {
      const uploaded = await uploadCMSImage(session, refreshCallbacks, file);
      onChange(uploaded.url);
    } catch (failure) {
      setError(describe(failure));
    } finally {
      setUploading(false);
      // Clearing lets the same file be chosen again after a failure —
      // without this the input's value is unchanged and no event fires.
      if (inputRef.current) inputRef.current.value = "";
    }
  }

  return (
    <div className="flex flex-col gap-2">
      <span id="cover-image-label" className="text-sm font-medium text-ink">
        Cover image
      </span>

      {value ? (
        <div className="flex items-center gap-4 rounded-lg border border-border bg-surface-sunken p-3">
          {/* Deliberately a plain img: the source is a Cloudinary URL known
              only at runtime, which next/image cannot pre-size, and this is
              an editor preview rather than a page the client loads. */}
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src={value}
            alt="Cover image preview"
            className="size-20 shrink-0 rounded-md object-cover"
          />
          <p className="min-w-0 flex-1 truncate text-[13px] text-ink-muted">{value}</p>
          <Button
            variant="ghost"
            size="sm"
            disabled={disabled || uploading}
            onClick={() => onChange("")}
          >
            <Trash2 size={15} aria-hidden="true" className="mr-1.5" />
            Remove
          </Button>
        </div>
      ) : (
        <div className="flex items-center gap-3 rounded-lg border border-dashed border-border-strong bg-surface-sunken px-4 py-6">
          <ImagePlus size={20} aria-hidden="true" className="shrink-0 text-ink-faint" />
          <p className="flex-1 text-[13px] leading-[1.5] text-ink-muted">
            A wide image reads best — around 1200×630. JPEG, PNG, WebP or AVIF, under 8MB.
          </p>
        </div>
      )}

      <div>
        <Button
          variant="secondary"
          size="sm"
          disabled={disabled || uploading}
          onClick={() => inputRef.current?.click()}
          aria-describedby="cover-image-label"
        >
          {uploading ? (
            <>
              <Loader2 size={15} aria-hidden="true" className="mr-1.5 animate-spin" />
              Uploading…
            </>
          ) : value ? (
            "Replace image"
          ) : (
            "Choose an image"
          )}
        </Button>
      </div>

      <input
        ref={inputRef}
        type="file"
        accept={ACCEPT_ATTRIBUTE}
        aria-label="Cover image file"
        className="sr-only"
        onChange={(event) => void handleFile(event.target.files?.[0])}
      />

      {error ? (
        <p role="alert" className="flex items-start gap-1.5 text-[13px] text-danger-ink">
          <CircleAlert size={14} aria-hidden="true" className="mt-0.5 shrink-0" />
          {error}
        </p>
      ) : null}
    </div>
  );
}
