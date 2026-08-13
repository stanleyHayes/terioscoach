import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { MAX_IMAGE_BYTES, rejectionReason, uploadCMSImage } from "@/lib/media";

const authedRequest = vi.hoisted(() => vi.fn());

vi.mock("@/lib/api", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/api")>();
  return { ...original, authedRequest };
});

const session = { accessToken: "a1", refreshToken: "r1" };
const callbacks = { onTokensRefreshed: vi.fn() };

function imageFile(name = "cover.png", type = "image/png", size = 1024): File {
  const file = new File(["x"], name, { type });
  Object.defineProperty(file, "size", { value: size });
  return file;
}

describe("rejectionReason", () => {
  it("accepts the formats the site serves", () => {
    for (const type of ["image/jpeg", "image/png", "image/webp", "image/avif"]) {
      expect(rejectionReason(imageFile("c", type))).toBeNull();
    }
  });

  it("refuses anything that is not one of them", () => {
    // SVG is the one worth naming: it can carry script, and it is the
    // format people reach for when they want a logo on a page.
    expect(rejectionReason(imageFile("logo.svg", "image/svg+xml"))).toMatch(/JPEG, PNG, WebP/);
    expect(rejectionReason(imageFile("notes.pdf", "application/pdf"))).toMatch(/JPEG/);
  });

  it("refuses an oversized file before the upload starts", () => {
    expect(rejectionReason(imageFile("big.png", "image/png", MAX_IMAGE_BYTES + 1))).toMatch(/8MB/);
    expect(rejectionReason(imageFile("ok.png", "image/png", MAX_IMAGE_BYTES))).toBeNull();
  });
});

describe("uploadCMSImage", () => {
  const signed = {
    url: "https://api.cloudinary.com/v1_1/demo/image/upload",
    fields: { api_key: "k", timestamp: "1", folder: "cms", signature: "sig" },
    expiresAt: "2026-08-11T10:00:00Z",
  };

  beforeEach(() => {
    vi.clearAllMocks();
    authedRequest.mockResolvedValue(signed);
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          secure_url: "https://res.cloudinary.com/demo/image/upload/cms/cover.png",
          public_id: "cms/cover",
          bytes: 2048,
        }),
      }),
    );
  });

  afterEach(() => vi.unstubAllGlobals());

  it("rejects an unusable file without asking the API for a signature", async () => {
    await expect(
      uploadCMSImage(session, callbacks, imageFile("logo.svg", "image/svg+xml")),
    ).rejects.toBeInstanceOf(ApiError);
    expect(authedRequest).not.toHaveBeenCalled();
    expect(fetch).not.toHaveBeenCalled();
  });

  it("posts the signed fields and the file straight to Cloudinary", async () => {
    const result = await uploadCMSImage(session, callbacks, imageFile());

    expect(authedRequest.mock.calls[0]![0]).toBe("/v1/admin/documents/sign-upload");
    expect(authedRequest.mock.calls[0]![3]).toMatchObject({
      method: "POST",
      body: { kind: "cms_image", filename: "cover.png" },
    });

    const [url, init] = (fetch as unknown as ReturnType<typeof vi.fn>).mock.calls[0]!;
    expect(url).toBe(signed.url);
    const form = init.body as FormData;
    for (const [key, value] of Object.entries(signed.fields)) {
      expect(form.get(key)).toBe(value);
    }
    expect(form.get("file")).toBeInstanceOf(File);

    expect(result.url).toBe("https://res.cloudinary.com/demo/image/upload/cms/cover.png");
    expect(result.publicId).toBe("cms/cover");
  });

  it("records the upload with the API afterwards", async () => {
    await uploadCMSImage(session, callbacks, imageFile());

    const record = authedRequest.mock.calls.find(([path]) => path === "/v1/admin/documents");
    expect(record).toBeDefined();
    expect(record![3].body).toEqual({
      kind: "cms_image",
      publicId: "cms/cover",
      filename: "cover.png",
      bytes: 2048,
    });
  });

  it("still returns the image when only the bookkeeping call fails", async () => {
    // Cloudinary has the bytes and the editor has a usable URL. Failing the
    // whole upload over a missing document row would throw that away.
    authedRequest.mockImplementation((path: string) =>
      path === "/v1/admin/documents"
        ? Promise.reject(new ApiError(500, "server_error", "nope"))
        : Promise.resolve(signed),
    );

    const result = await uploadCMSImage(session, callbacks, imageFile());
    expect(result.url).toContain("res.cloudinary.com");
  });

  it("reports a failed upload in our own words, not Cloudinary's", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false, status: 400 }));

    await expect(uploadCMSImage(session, callbacks, imageFile())).rejects.toThrow(
      /could not be uploaded/i,
    );
  });
});
