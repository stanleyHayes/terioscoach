import { afterEach, expect, it, vi } from "vitest";
import { createRecordingComposite } from "./recording-compositor";

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); vi.useRealTimers(); });
it("draws both participants, follows replacement tracks, and cleans up only its own stream", () => {
  vi.useFakeTimers();
  const paint = { fillStyle: "", font: "", textAlign: "", fillRect: vi.fn(), fillText: vi.fn(), drawImage: vi.fn() };
  vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue(paint as unknown as CanvasRenderingContext2D);
  const stop = vi.fn();
  Object.defineProperty(HTMLCanvasElement.prototype, "captureStream", { configurable: true, value: () => ({ getTracks: () => [{ stop }] }) });
  vi.spyOn(HTMLMediaElement.prototype, "play").mockResolvedValue();
  vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => {});
  vi.spyOn(HTMLMediaElement.prototype, "readyState", "get").mockReturnValue(4);
  vi.spyOn(HTMLVideoElement.prototype, "videoWidth", "get").mockReturnValue(640);
  vi.spyOn(HTMLVideoElement.prototype, "videoHeight", "get").mockReturnValue(480);
  class Stream {
    constructor(public tracks: unknown[]) {}
    getVideoTracks() { return this.tracks; }
  }
  vi.stubGlobal("MediaStream", Stream);
  const local = { enabled: true, muted: false, readyState: "live", stop: vi.fn() };
  const remote = { ...local, stop: vi.fn() };
  const sources = [new Stream([local]), new Stream([remote])];
  const composite = createRecordingComposite(() => sources as unknown as [MediaStream, MediaStream]);
  expect(paint.drawImage).toHaveBeenCalledTimes(2);
  expect(paint.fillText).toHaveBeenCalledWith("Practitioner", 320, 690);
  expect(paint.fillText).toHaveBeenCalledWith("Client", 960, 690);
  const replacement = { ...local, enabled: false };
  sources[0].tracks = [replacement];
  vi.advanceTimersByTime(40);
  expect(paint.fillText).toHaveBeenCalledWith("Camera unavailable", 320, 332);
  const ownVideo = paint.drawImage.mock.calls[0][0] as HTMLVideoElement;
  expect((ownVideo.srcObject as unknown as Stream).tracks[0]).toBe(replacement);
  composite.stop();
  const draws = paint.drawImage.mock.calls.length;
  vi.advanceTimersByTime(80);
  expect(paint.drawImage).toHaveBeenCalledTimes(draws);
  expect(stop).toHaveBeenCalledOnce();
  expect(local.stop).not.toHaveBeenCalled();
  expect(remote.stop).not.toHaveBeenCalled();
});
