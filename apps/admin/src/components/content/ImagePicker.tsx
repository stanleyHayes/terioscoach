"use client";

import { CircleAlert, ImagePlus, Images, Loader2, Search, Trash2, Upload } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { useAuth } from "@/lib/auth";
import { ACCEPT_ATTRIBUTE, deleteCMSImage, listCMSImages, uploadCMSImage, type MediaLibraryItem } from "@/lib/media";
import { describe } from "@/lib/use-resource";

// Keep the bundled editorial library usable before custom-domain cutover.
// The stable Vercel alias serves the same public assets as terioscoach.com,
// while the relative path saved into the CMS remains domain-independent.
const ASSET_ORIGIN = (
  process.env.NEXT_PUBLIC_ASSET_ORIGIN || "https://terioscoach.com"
).replace(/\/$/, "");

const bundledImages = [
  ["Theresa — clinical", "/images/brand/theresa-yirerong-clinical.webp", "Theresa"],
  ["Theresa — founder", "/images/brand/theresa-yirerong-about.webp", "Theresa"],
  ["Virtual care setting", "/images/marketing/services-care.webp", "Wellness"],
  ["Wellness consultation", "/images/marketing/home-hero.webp", "Wellness"],
  ["Practitioner portrait", "/images/marketing/about-practitioner.webp", "Theresa"],
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
    `/images/brand/portraits/${name}.webp`, "Theresa",
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
    `/images/blog/${name}.webp`, "Wellness",
  ]),
] as const;

function previewURL(value: string) {
  return value.startsWith("/") ? `${ASSET_ORIGIN}${value}` : value;
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
  label = "Cover image",
}: {
  /** The current cover URL, or "" for none. */
  value: string;
  onChange: (url: string) => void;
  disabled?: boolean;
  label?: string;
}) {
  const { session, refreshCallbacks } = useAuth();
  const confirmationRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [libraryOpen, setLibraryOpen] = useState(false);
  const [uploadedImages, setUploadedImages] = useState<MediaLibraryItem[]>([]);
  const [libraryLoading, setLibraryLoading] = useState(false);
  const [confirming, setConfirming] = useState<MediaLibraryItem | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState("All");

  useEffect(() => {
    if (!libraryOpen || !session) return;
    let cancelled = false;
    listCMSImages(session, refreshCallbacks)
      .then((items) => { if (!cancelled) setUploadedImages(items); })
      .catch((failure) => { if (!cancelled) setError(describe(failure)); })
      .finally(() => { if (!cancelled) setLibraryLoading(false); });
    return () => { cancelled = true; };
  }, [libraryOpen, refreshCallbacks, session]);

  useEffect(() => {
    if (confirming) confirmationRef.current?.closest('[role="dialog"]')?.scrollTo?.({ top: 0 });
  }, [confirming]);

  function openLibrary() {
    setError(null);
    setNotice(null);
    setConfirming(null);
    setLibraryLoading(true);
    setLibraryOpen(true);
  }

  const library = useMemo(() => {
    const uploaded = uploadedImages.map((item) => ({ label: item.title || item.filename, url: item.url, category: "Uploads", item }));
    const bundled = bundledImages.map(([label, url, group]) => ({ label, url, category: group, item: undefined }));
    const needle = query.trim().toLowerCase();
    return [...uploaded, ...bundled].filter((item) =>
      (category === "All" || item.category === category) &&
      (!needle || item.label.toLowerCase().includes(needle)),
    );
  }, [category, query, uploadedImages]);

  async function handleFile(file: File | undefined) {
    if (!file || !session) return;
    setError(null);
    setUploading(true);
    try {
      const uploaded = await uploadCMSImage(session, refreshCallbacks, file);
      onChange(uploaded.url);
      if (uploaded.documentId) setUploadedImages((items) => [{ id: uploaded.documentId!, url: uploaded.url, title: uploaded.filename, filename: uploaded.filename, bytes: uploaded.bytes, createdAt: new Date().toISOString() }, ...items]);
    } catch (failure) {
      setError(describe(failure));
    } finally {
      setUploading(false);
      // Clearing lets the same file be chosen again after a failure —
      // without this the input's value is unchanged and no event fires.
      if (inputRef.current) inputRef.current.value = "";
    }
  }

  async function removeImage() {
    if (!session || !confirming || deleting) return;
    const image = confirming;
    setDeleting(true);
    setError(null);
    try {
      await deleteCMSImage(session, refreshCallbacks, image.id);
      setUploadedImages((items) => items.filter((item) => item.id !== image.id));
      if (value === image.url) onChange("");
      setConfirming(null);
      setNotice(`${image.title || image.filename} deleted from the media library.`);
    } catch (failure) {
      setError(describe(failure));
    } finally {
      setDeleting(false);
    }
  }

  return (
    <div className="flex flex-col gap-2">
      <span id="cover-image-label" className="text-sm font-medium text-ink">
        {label}
      </span>

      {value ? (
        <div className="group relative aspect-[16/7] min-h-48 overflow-hidden rounded-2xl border border-border bg-surface-sunken">
          {/* Deliberately a plain img: the source is a Cloudinary URL known
              only at runtime, which next/image cannot pre-size, and this is
              an editor preview rather than a page the client loads. */}
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src={previewURL(value)}
            alt="Cover image preview"
            className="size-full object-cover"
          />
          <div className="absolute inset-x-3 bottom-3 flex items-center gap-3 rounded-xl border border-white/15 bg-eucalyptus-900 p-3 text-sand-0 backdrop-blur-md">
            <p className="min-w-0 flex-1 truncate text-xs">{value}</p>
            <Button variant="ghost" size="sm" disabled={disabled || uploading} onClick={() => onChange("")} className="text-sand-0 hover:bg-white/10 hover:text-sand-0">
              <Trash2 size={15} aria-hidden="true" className="mr-1.5" /> Remove
            </Button>
          </div>
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

      <div className="flex flex-wrap gap-2">
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
          ) : (
            <><Upload size={15} aria-hidden="true" className="mr-1.5" />Upload a new image</>
          )}
        </Button>
        <Button variant="secondary" size="sm" disabled={disabled || uploading} onClick={openLibrary}>
          <Images size={15} aria-hidden="true" className="mr-1.5" /> Choose from media library
        </Button>
      </div>

      <Modal open={libraryOpen} onClose={() => { if (!deleting) setLibraryOpen(false); }} title="Media library" description="Reuse an uploaded image or choose from the Terios brand collection." size="wide">
        <div className="sticky top-0 z-10 -mx-1 mb-5 grid gap-3 bg-surface-raised pb-3 sm:grid-cols-[1fr_auto]">
          <label className="relative block">
            <span className="sr-only">Search media</span>
            <Search aria-hidden="true" className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-ink-faint" />
            <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search images" className="h-10 w-full rounded-lg border border-border bg-surface pl-10 pr-3 text-sm text-ink outline-none focus:border-primary focus:ring-2 focus:ring-primary/15" />
          </label>
          <div className="flex flex-wrap gap-1" aria-label="Filter media">
            {["All", "Uploads", "Theresa", "Wellness"].map((item) => <button key={item} type="button" aria-pressed={category === item} onClick={() => setCategory(item)} className="rounded-full border border-border px-3 py-2 text-xs font-medium text-ink-muted transition-colors hover:text-ink aria-pressed:border-primary aria-pressed:bg-eucalyptus-50 aria-pressed:text-primary">{item}</button>)}
          </div>
        </div>
        {error ? <p role="alert" className="mb-4 rounded-lg bg-danger-bg p-3 text-sm text-danger-ink">{error}</p> : null}
        {notice ? <p role="status" className="mb-4 rounded-lg bg-eucalyptus-100 p-3 text-sm text-eucalyptus-800">{notice}</p> : null}
        {confirming ? (
          <div ref={confirmationRef} role="group" aria-label="Delete image confirmation" className="mb-5 rounded-xl border border-danger/30 bg-surface-sunken p-4">
            <p className="font-semibold text-ink">Delete {confirming.title || confirming.filename}?</p>
            <p className="mt-2 text-sm leading-relaxed text-ink-muted">This permanently removes the uploaded file. Any published pages or posts using it will lose their image. Replace it there before deleting.</p>
            <div className="mt-4 flex flex-wrap gap-2">
              <Button variant="secondary" disabled={deleting} onClick={() => { setConfirming(null); setError(null); }}>Keep image</Button>
              <Button variant="danger" loading={deleting} onClick={() => void removeImage()}>Delete image</Button>
            </div>
          </div>
        ) : null}
        <p className="mb-4 text-xs text-ink-muted">Uploaded images can be deleted. The bundled Terios brand collection is part of the website.</p>
        {libraryLoading ? <div className="flex min-h-64 items-center justify-center text-sm text-ink-muted"><Loader2 className="mr-2 size-5 animate-spin" />Loading your uploads…</div> : library.length ? (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
          {library.map(({ label, url, category: group, item }) => (
            <div key={url} className="overflow-hidden rounded-xl border border-border bg-surface-sunken">
            <button
              key={url}
              type="button"
              disabled={disabled || uploading || deleting}
              aria-label={`Use ${label}`}
              aria-pressed={value === url}
              onClick={() => { onChange(url); setLibraryOpen(false); }}
              className="group w-full overflow-hidden text-left focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary aria-pressed:ring-2 aria-pressed:ring-primary"
            >
              <span className="relative block aspect-[4/3] overflow-hidden">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src={previewURL(url)}
                alt=""
                className="h-full w-full object-cover transition-transform group-hover:scale-105"
              />
              </span>
              <span className="block px-3 pb-1 pt-2 text-[10px] font-semibold uppercase tracking-[.08em] text-primary">{group}</span>
              <span className="line-clamp-2 block min-h-11 px-3 pb-3 text-xs font-medium leading-4 text-ink">{label}</span>
            </button>
            {item ? <button type="button" disabled={disabled || deleting} aria-label={`Delete ${label}`} onClick={() => { setConfirming(item); setError(null); setNotice(null); }} className="flex min-h-10 w-full items-center justify-center gap-2 border-t border-border px-3 py-2 text-xs font-semibold text-danger-ink hover:bg-danger-bg disabled:opacity-50"><Trash2 size={14} aria-hidden="true" />Delete image</button> : null}
            </div>
          ))}
        </div>
        ) : <div className="grid min-h-64 place-items-center rounded-xl border border-dashed border-border text-sm text-ink-muted">No images match that search.</div>}
      </Modal>

      <input
        ref={inputRef}
        type="file"
        accept={ACCEPT_ATTRIBUTE}
        aria-label="Cover image file"
        className="sr-only"
        onChange={(event) => void handleFile(event.target.files?.[0])}
      />

      {error && !libraryOpen ? (
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
