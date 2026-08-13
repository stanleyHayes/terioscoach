/**
 * Direct-to-Cloudinary image upload for CMS imagery (BE-11 / ADM-07).
 *
 * The file never passes through our API. The API mints a signature for one
 * upload into one folder, the browser POSTs the bytes straight to
 * Cloudinary, and the API is then told what landed. That keeps large files
 * off the API's request path and out of its memory, and it means an upload
 * cannot be redirected somewhere else — the folder and delivery type are
 * inside the signature.
 */

import { ApiError, authedRequest, type RefreshCallbacks, type Session } from "@/lib/api";

/** What the API hands back: where to POST, and the fields to send with it. */
interface SignedUpload {
  url: string;
  fields: Record<string, string>;
  expiresAt: string;
}

/** The subset of Cloudinary's upload response we rely on. */
interface CloudinaryUpload {
  secure_url: string;
  public_id: string;
  bytes: number;
}

export interface UploadedImage {
  /** The public delivery URL, which is what a post's coverImage stores. */
  url: string;
  publicId: string;
  filename: string;
  bytes: number;
}

/** Cloudinary's free tier tops out well above this; the limit is here so a
 * mistaken 40MB original fails immediately rather than after a long upload. */
export const MAX_IMAGE_BYTES = 8 * 1024 * 1024;

const ACCEPTED = ["image/jpeg", "image/png", "image/webp", "image/avif"];

/** Stated on the picker and enforced here, so the two cannot drift. */
export const ACCEPT_ATTRIBUTE = ACCEPTED.join(",");

export function rejectionReason(file: File): string | null {
  if (!ACCEPTED.includes(file.type)) {
    return "Choose a JPEG, PNG, WebP or AVIF image.";
  }
  if (file.size > MAX_IMAGE_BYTES) {
    return `That image is ${Math.round(file.size / 1024 / 1024)}MB. Keep it under 8MB.`;
  }
  return null;
}

/**
 * Signs, uploads, and records one CMS image.
 *
 * The recording step is deliberately last and deliberately non-fatal to the
 * returned URL: if Cloudinary has the bytes, the editor has a usable image,
 * and failing the whole operation over the bookkeeping call would lose it.
 */
export async function uploadCMSImage(
  session: Session,
  callbacks: RefreshCallbacks,
  file: File,
): Promise<UploadedImage> {
  const reason = rejectionReason(file);
  if (reason) {
    throw new ApiError(400, "validation_error", reason);
  }

  const signed = await authedRequest<SignedUpload>(
    "/v1/admin/documents/sign-upload",
    session,
    callbacks,
    { method: "POST", body: { kind: "cms_image", filename: file.name } },
  );

  const form = new FormData();
  for (const [name, value] of Object.entries(signed.fields)) {
    form.append(name, value);
  }
  form.append("file", file);

  const response = await fetch(signed.url, { method: "POST", body: form });
  if (!response.ok) {
    // Cloudinary's error body is its own shape, not ours; the status is the
    // only part worth reporting and the practitioner can only retry anyway.
    throw new ApiError(response.status, "upload_failed", "The image could not be uploaded. Try again.");
  }
  const uploaded = (await response.json()) as CloudinaryUpload;

  await authedRequest("/v1/admin/documents", session, callbacks, {
    method: "POST",
    body: {
      kind: "cms_image",
      publicId: uploaded.public_id,
      filename: file.name,
      bytes: uploaded.bytes,
    },
  }).catch(() => {
    // Recorded or not, the image is live and usable. The gap is a document
    // row, not a broken page.
  });

  return {
    url: uploaded.secure_url,
    publicId: uploaded.public_id,
    filename: file.name,
    bytes: uploaded.bytes,
  };
}
