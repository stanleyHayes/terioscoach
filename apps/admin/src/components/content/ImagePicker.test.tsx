import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { ImagePicker } from "./ImagePicker";

const { uploadCMSImage, listCMSImages } = vi.hoisted(() => ({
  uploadCMSImage: vi.fn(),
  listCMSImages: vi.fn(),
}));

vi.mock("@/lib/media", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/media")>();
  return { ...original, uploadCMSImage, listCMSImages };
});

vi.mock("@/lib/auth", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/auth")>();
  return {
    ...original,
    useAuth: () => ({
      status: "authenticated",
      user: { id: "prac-1", email: "t@example.com", role: "practitioner", name: "Terios" },
      session: { accessToken: "a1", refreshToken: "r1" },
      refreshCallbacks: { onTokensRefreshed: vi.fn() },
      logout: vi.fn(),
    }),
  };
});

function pngFile(name = "cover.png"): File {
  return new File(["x"], name, { type: "image/png" });
}

/** The input is visually hidden and driven by a button, so tests reach it
 * by its label rather than by role. */
function fileInput(): HTMLInputElement {
  return screen.getByLabelText(/cover image file/i) as HTMLInputElement;
}

describe("ImagePicker", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    uploadCMSImage.mockResolvedValue({
      url: "https://res.cloudinary.com/demo/cover.png",
      publicId: "cms/cover",
      filename: "cover.png",
      bytes: 1024,
    });
    listCMSImages.mockResolvedValue([]);
  });

  it("uses no visible native file control", () => {
    const { container } = render(<ImagePicker value="" onChange={vi.fn()} />);
    // A bare file input is a native control we cannot style; it is present
    // for the browser's sake but never the thing the practitioner clicks.
    expect(container.querySelector('input[type="file"]')!.className).toContain("sr-only");
    expect(screen.getByRole("button", { name: /upload a new image/i })).toBeTruthy();
  });

  it("hands back the uploaded URL", async () => {
    const onChange = vi.fn();
    render(<ImagePicker value="" onChange={onChange} />);

    fireEvent.change(fileInput(), { target: { files: [pngFile()] } });

    await waitFor(() =>
      expect(onChange).toHaveBeenCalledWith("https://res.cloudinary.com/demo/cover.png"),
    );
  });

  it("previews the current image and offers to replace it", () => {
    render(<ImagePicker value="https://res.cloudinary.com/demo/cover.png" onChange={vi.fn()} />);

    const preview = screen.getByAltText(/cover image preview/i) as HTMLImageElement;
    expect(preview.src).toBe("https://res.cloudinary.com/demo/cover.png");
    expect(screen.getByRole("button", { name: /upload a new image/i })).toBeTruthy();
  });

  it("loads bundled library thumbnails from the canonical public asset origin", async () => {
    render(<ImagePicker value="" onChange={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: /choose from media library/i }));
    const thumbnail = (await screen.findByRole("button", { name: /use theresa — clinical/i }))
      .querySelector("img") as HTMLImageElement;
    expect(thumbnail.src).toBe(
      "https://terioscoach.com/images/brand/theresa-yirerong-clinical.webp",
    );
  });

  it("reuses a previously uploaded image from the media library", async () => {
    listCMSImages.mockResolvedValue([{ id: "doc-1", url: "https://res.cloudinary.com/demo/upload/cms/about", title: "About portrait", filename: "about.webp", bytes: 2048, createdAt: "2026-09-03T00:00:00Z" }]);
    const onChange = vi.fn();
    render(<ImagePicker value="" onChange={onChange} />);

    fireEvent.click(screen.getByRole("button", { name: /choose from media library/i }));
    fireEvent.click(await screen.findByRole("button", { name: /use about portrait/i }));

    expect(onChange).toHaveBeenCalledWith("https://res.cloudinary.com/demo/upload/cms/about");
  });

  it("clears the image without touching the upload path", () => {
    const onChange = vi.fn();
    render(<ImagePicker value="https://res.cloudinary.com/demo/cover.png" onChange={onChange} />);

    fireEvent.click(screen.getByRole("button", { name: /remove/i }));

    expect(onChange).toHaveBeenCalledWith("");
    expect(uploadCMSImage).not.toHaveBeenCalled();
  });

  it("reports a rejected file and leaves the choice open", async () => {
    uploadCMSImage.mockRejectedValue(
      new ApiError(400, "validation_error", "Choose a JPEG, PNG, WebP or AVIF image."),
    );
    const onChange = vi.fn();
    render(<ImagePicker value="" onChange={onChange} />);

    fireEvent.change(fileInput(), { target: { files: [pngFile("logo.svg")] } });

    expect((await screen.findByRole("alert")).textContent).toMatch(/JPEG, PNG, WebP/);
    expect(onChange).not.toHaveBeenCalled();
    // Cleared, so choosing the same file again fires a fresh change event
    // rather than silently doing nothing.
    expect(fileInput().value).toBe("");
  });

  it("does not start a second upload while one is in flight", async () => {
    let release: (value: unknown) => void = () => {};
    uploadCMSImage.mockReturnValue(new Promise((resolve) => (release = resolve)));
    render(<ImagePicker value="" onChange={vi.fn()} />);

    fireEvent.change(fileInput(), { target: { files: [pngFile()] } });

    const button = await screen.findByRole("button", { name: /uploading/i });
    expect(button.hasAttribute("disabled")).toBe(true);

    release({ url: "https://res.cloudinary.com/demo/cover.png", publicId: "p", filename: "f", bytes: 1 });
    await waitFor(() => expect(screen.getByRole("button", { name: /upload a new image/i })).toBeTruthy());
  });
});
