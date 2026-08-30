"use client";

import { CircleAlert, ImagePlus, Images, Loader2, Trash2 } from "lucide-react";
import { useRef, useState } from "react";
import { Button } from "@/components/ui/Button";
import { useAuth } from "@/lib/auth";
import { ACCEPT_ATTRIBUTE, uploadCMSImage } from "@/lib/media";
import { describe } from "@/lib/use-resource";

const SITE_URL = (
  process.env.NEXT_PUBLIC_SITE_URL ?? "https://terioscoach.com"
).replace(/\/$/, "");

const bundledImages = [
  ["Theresa — clinical", "/images/brand/theresa-yirerong-clinical.webp"],
  ["Theresa — founder", "/images/brand/theresa-yirerong-about.webp"],
  ["Virtual care setting", "/images/marketing/services-care.webp"],
  ["Wellness consultation", "/images/marketing/home-hero.webp"],
  ["Practitioner portrait", "/images/marketing/about-practitioner.webp"],
  ...[
    "img-2201",
    "img-3762",
    "img-4169",
    "img-4293",
    "img-4562",
    "img-5569",
    "img-5973",
    "theresa-yirerong-by-jinnifer-douglass-002",
    "theresa-yirerong-by-jinnifer-douglass-007",
    "theresa-yirerong-by-jinnifer-douglass-008",
    "theresa-yirerong-by-jinnifer-douglass-009",
    "theresa-yirerong-by-jinnifer-douglass-010",
    "theresa-yirerong-by-jinnifer-douglass-020",
    "theresa-yirerong-by-jinnifer-douglass-035",
    "theresa-yirerong-by-jinnifer-douglass-036",
    "theresa-yirerong-by-jinnifer-douglass-037",
    "theresa-yirerong-by-jinnifer-douglass-038",
    "theresa-yirerong-by-jinnifer-douglass-039",
    "theresa-yirerong-by-jinnifer-douglass-050",
    "theresa-yirerong-by-jinnifer-douglass-053",
    "theresa-yirerong-by-jinnifer-douglass-060",
    "theresa-yirerong-by-jinnifer-douglass-062",
    "theresa-yirerong-by-jinnifer-douglass-092",
    "theresa-yirerong-by-jinnifer-douglass-093",
    "theresa-yirerong-by-jinnifer-douglass-095",
    "theresa-yirerong-by-jinnifer-douglass-096",
    "theresa-yirerong-by-jinnifer-douglass-101",
    "theresa-yirerong-by-jinnifer-douglass-103",
  ].map((name, index) => [
    `Theresa portrait ${index + 1}`,
    `/images/brand/portraits/${name}.webp`,
  ]),
  ...[
    "ai-generated-8686109_1280",
    "asparagus-2169305_1280",
    "businessman-8458550_1280",
    "drink-2471550_1280",
    "garlic-8227658_1280",
    "girl-9825347_1280",
    "grapes-3550733_1280",
    "lake-192979_1280",
    "lavender-3605688_1280",
    "marketing-online-1427787_1280",
    "meal-2834549_1280",
    "milkshake-8323288_1280",
    "people-8577400_1280",
    "swan-2077219_1280",
    "tray-2546077_1280",
    "waterfall-9684883_1280",
    "wine-3219850_640",
    "woman-7252445_1280",
    "woman-792162_1280",
    "woman-8656633_1280",
  ].map((name) => [
    name.replace(/[-_]\d+.*/, "").replaceAll("-", " "),
    `/images/blog/${name}.webp`,
  ]),
] as const;

function previewURL(value: string) {
  return value.startsWith("/") ? `${SITE_URL}${value}` : value;
}

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
            src={previewURL(value)}
            alt="Cover image preview"
            className="size-20 shrink-0 rounded-md object-cover"
          />
          <p className="min-w-0 flex-1 truncate text-[13px] text-ink-muted">
            {value}
          </p>
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
          <ImagePlus
            size={20}
            aria-hidden="true"
            className="shrink-0 text-ink-faint"
          />
          <p className="flex-1 text-[13px] leading-[1.5] text-ink-muted">
            A wide image reads best — around 1200×630. JPEG, PNG, WebP or AVIF,
            under 8MB.
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
              <Loader2
                size={15}
                aria-hidden="true"
                className="mr-1.5 animate-spin"
              />
              Uploading…
            </>
          ) : value ? (
            "Replace image"
          ) : (
            "Choose an image"
          )}
        </Button>
      </div>

      <details className="rounded-lg border border-border bg-surface-raised">
        <summary className="flex cursor-pointer list-none items-center gap-2 px-4 py-3 text-sm font-medium text-ink">
          <Images size={16} aria-hidden="true" /> Choose from the Terios library
        </summary>
        <div className="grid max-h-72 grid-cols-3 gap-2 overflow-y-auto border-t border-border p-3 sm:grid-cols-4">
          {bundledImages.map(([label, url]) => (
            <button
              key={url}
              type="button"
              disabled={disabled || uploading}
              aria-label={`Use ${label}`}
              aria-pressed={value === url}
              onClick={() => onChange(url)}
              className="group relative aspect-square overflow-hidden rounded-md border border-border bg-surface-sunken focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary aria-pressed:ring-2 aria-pressed:ring-primary"
            >
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src={previewURL(url)}
                alt=""
                className="h-full w-full object-cover transition-transform group-hover:scale-105"
              />
              <span className="absolute inset-x-0 bottom-0 bg-eucalyptus-950/80 px-1.5 py-1 text-left text-[9px] leading-tight text-sand-0">
                {label}
              </span>
            </button>
          ))}
        </div>
      </details>

      <input
        ref={inputRef}
        type="file"
        accept={ACCEPT_ATTRIBUTE}
        aria-label="Cover image file"
        className="sr-only"
        onChange={(event) => void handleFile(event.target.files?.[0])}
      />

      {error ? (
        <p
          role="alert"
          className="flex items-start gap-1.5 text-[13px] text-danger-ink"
        >
          <CircleAlert
            size={14}
            aria-hidden="true"
            className="mt-0.5 shrink-0"
          />
          {error}
        </p>
      ) : null}
    </div>
  );
}
