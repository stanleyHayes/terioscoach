import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { RecordingPlayer } from "./RecordingPlayer";

const props = { url: "https://api.cloudinary.com/video/download?signature=old", contentType: "video/mp4", fileName: "session.mp4" };
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });
describe("RecordingPlayer", () => {
  it("recovers playback when an expired URL is refreshed", () => {
    const { rerender } = render(<RecordingPlayer {...props} />);
    const player = screen.getByLabelText("Session recording (MP4)");
    expect(player.hasAttribute("playsinline")).toBe(true);
    fireEvent.error(player);
    expect(screen.getByRole("status").textContent).toContain("Refresh the page");
    rerender(<RecordingPlayer {...props} url="https://api.cloudinary.com/video/download?signature=fresh" />);
    expect(screen.getByLabelText("Session recording (MP4)")).toBeTruthy();
  });
  it("downloads a named blob without opening the Cloudinary URL", async () => {
    const fetcher = vi.fn().mockResolvedValue({ ok: true, blob: async () => new Blob(["video"], { type: "video/mp4" }) });
    vi.stubGlobal("fetch", fetcher);
    vi.stubGlobal("URL", Object.assign(URL, { createObjectURL: vi.fn().mockReturnValue("blob:recording"), revokeObjectURL: vi.fn() }));
    let clicked: { download: string; href: string; target: string } | undefined;
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(function (this: HTMLAnchorElement) { clicked = { download: this.download, href: this.href, target: this.target }; });
    render(<RecordingPlayer {...props} />);
    fireEvent.click(screen.getByRole("button", { name: "Download recording" }));
    await waitFor(() => expect(clicked?.download).toBe("session.mp4"));
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Download recording" }).hasAttribute("disabled")).toBe(false),
    );
    expect(clicked?.href).toBe("blob:recording");
    expect(clicked?.target).toBe("");
    expect(fetcher).toHaveBeenCalledWith(props.url, { credentials: "omit", cache: "no-store" });
  });
  it("shows a retryable error rather than navigating to a failed private link", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("expired", { status: 401 })));
    render(<RecordingPlayer {...props} />);
    fireEvent.click(screen.getByRole("button", { name: "Download recording" }));
    expect((await screen.findByRole("alert")).textContent).toContain("new private link");
    expect(screen.getByRole("button", { name: "Download recording" }).hasAttribute("disabled")).toBe(false);
  });
});
