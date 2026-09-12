import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { RecordingPlayer } from "./RecordingPlayer";

describe("RecordingPlayer", () => {
  it("offers download during playback and recovers when an expired URL is refreshed", () => {
    const props = { url: "https://api.cloudinary.com/video/download?signature=old", contentType: "video/mp4", fileName: "session.mp4" };
    const { rerender } = render(<RecordingPlayer {...props} />);
    const player = screen.getByLabelText("Session recording (MP4)");
    expect(player.hasAttribute("playsinline")).toBe(true);
    expect(screen.getByRole("link", { name: "Download recording" }).getAttribute("href")).toBe(props.url);
    fireEvent.error(player);
    expect(screen.getByRole("status").textContent).toContain("Refresh the page");
    expect(screen.queryByLabelText("Session recording (MP4)")).toBeNull();
    rerender(<RecordingPlayer {...props} url="https://api.cloudinary.com/video/download?signature=fresh" />);
    expect(screen.getByLabelText("Session recording (MP4)")).toBeTruthy();
  });
});
