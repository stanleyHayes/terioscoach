"use client";

import {
  Captions,
  Circle,
  Hand,
  Maximize,
  MessageSquare,
  Mic,
  MicOff,
  MonitorUp,
  PhoneOff,
  PictureInPicture2,
  Settings2,
  Smile,
  Square,
  Video,
  VideoOff,
} from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/Button";
import { cn } from "@/lib/cn";
import { useVideoRoom } from "@/lib/use-video-room";

/**
 * The video room (CX-06; the portal has its own copy — the two apps
 * deploy separately and share no package).
 *
 * Every piece of chrome here is ours: the tiles, the controls, the states.
 * No browser call UI, per the platform rule — and because a wellness
 * session should not look like a conference call.
 *
 * Streams are attached to the video elements through refs rather than being
 * held in JSX, because a MediaStream is not a value React can diff; the
 * element's srcObject is the only place it belongs.
 */
export interface VideoRoomProps {
  bookingId: string;
  /** Shown on the remote tile before the other person arrives. */
  peerLabel: string;
  onLeave?: () => void;
}

const REACTIONS = ["👍", "❤️", "😂", "👏", "🎉"];

/** How long a reaction floats over a tile before fading. */
const REACTION_MS = 2500;

function formatClock(totalSeconds: number): string {
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${String(seconds).padStart(2, "0")}`;
}

/** One round control in the call bar. */
function ControlButton({
  label,
  pressed,
  disabled,
  danger,
  title,
  onClick,
  children,
}: {
  label: string;
  pressed?: boolean;
  disabled?: boolean;
  /** Danger styling for "off"/ending states. */
  danger?: boolean;
  title?: string;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      aria-label={label}
      aria-pressed={pressed}
      title={title ?? label}
      onClick={onClick}
      disabled={disabled}
      className={cn(
        "relative flex size-12 items-center justify-center rounded-full transition-colors duration-instant ease-out disabled:opacity-40",
        danger ? "bg-danger text-on-primary" : "bg-surface-sunken text-ink hover:bg-border",
      )}
    >
      {children}
    </button>
  );
}

export function VideoRoom({ bookingId, peerLabel, onLeave }: VideoRoomProps) {
  const room = useVideoRoom(bookingId);
  const localVideo = useRef<HTMLVideoElement | null>(null);
  const remoteVideo = useRef<HTMLVideoElement | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);

  const [chatOpen, setChatOpen] = useState(false);
  const [chatDraft, setChatDraft] = useState("");
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [reactionsOpen, setReactionsOpen] = useState(false);
  const [fullscreen, setFullscreen] = useState(false);
  // A reaction floats briefly, then fades. Keying the timeout on the
  // reaction itself means back-to-back reactions each get their moment.
  const [dismissedAt, setDismissedAt] = useState(0);
  useEffect(() => {
    if (!room.activeReaction) return;
    const at = room.activeReaction.at;
    const fade = setTimeout(() => setDismissedAt(at), REACTION_MS);
    return () => clearTimeout(fade);
  }, [room.activeReaction]);
  const visibleReaction =
    room.activeReaction && room.activeReaction.at > dismissedAt ? room.activeReaction : null;

  useEffect(() => {
    if (localVideo.current) localVideo.current.srcObject = room.localStream;
  }, [room.localStream]);

  useEffect(() => {
    if (remoteVideo.current) remoteVideo.current.srcObject = room.remoteStream;
  }, [room.remoteStream]);

  // Opening the panel acknowledges everything in it.
  useEffect(() => {
    if (chatOpen) room.markChatRead();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [chatOpen, room.messages.length]);

  useEffect(() => {
    const onChange = () => setFullscreen(document.fullscreenElement !== null);
    document.addEventListener("fullscreenchange", onChange);
    return () => document.removeEventListener("fullscreenchange", onChange);
  }, []);

  const live = room.state === "connected" || room.state === "waiting";
  const pipSupported =
    typeof document !== "undefined" && "pictureInPictureEnabled" in document;

  const toggleFullscreen = () => {
    if (document.fullscreenElement) {
      void document.exitFullscreen();
    } else {
      void containerRef.current?.requestFullscreen?.();
    }
  };

  const togglePictureInPicture = () => {
    void (async () => {
      if (document.pictureInPictureElement) {
        await document.exitPictureInPicture();
      } else {
        await remoteVideo.current?.requestPictureInPicture?.();
      }
    })().catch(() => {});
  };

  const submitChat = () => {
    room.sendChat(chatDraft);
    setChatDraft("");
  };

  if (room.state === "idle") {
    return (
      <div className="flex flex-col items-center gap-5 rounded-xl border border-border bg-surface-raised px-6 py-16 text-center">
        <h2 className="font-display text-[1.5rem] font-medium text-ink">Ready when you are</h2>
        <p className="max-w-[46ch] text-sm leading-[1.55] text-ink-muted">
          Joining will ask for your camera and microphone. You can turn either
          off once you are in.
        </p>
        <div className="flex flex-wrap items-center justify-center gap-3">
          <Button onClick={() => room.join()}>Join the session</Button>
          <Button variant="secondary" onClick={() => room.join({ video: false })}>
            Join without camera
          </Button>
        </div>
      </div>
    );
  }

  if (room.state === "failed") {
    return (
      <div
        role="alert"
        className="flex flex-col items-center gap-5 rounded-xl border border-border bg-surface-raised px-6 py-16 text-center"
      >
        <h2 className="font-display text-[1.5rem] font-medium text-ink">
          The session couldn&rsquo;t start
        </h2>
        <p className="max-w-[48ch] text-sm leading-[1.55] text-ink-muted">{room.error}</p>
        <Button variant="secondary" onClick={() => room.join()}>
          Try again
        </Button>
      </div>
    );
  }

  if (room.state === "ended") {
    return (
      <div className="flex flex-col items-center gap-5 rounded-xl border border-border bg-surface-raised px-6 py-16 text-center">
        <h2 className="font-display text-[1.5rem] font-medium text-ink">Session ended</h2>
        <p className="max-w-[46ch] text-sm leading-[1.55] text-ink-muted">
          Your camera and microphone have been released.
        </p>
        <Button variant="secondary" onClick={() => room.join()}>
          Rejoin
        </Button>
      </div>
    );
  }

  return (
    <div ref={containerRef} className="flex flex-col gap-4 bg-surface">
      <div className="relative overflow-hidden rounded-xl border border-border bg-ink">
        {/* Remote tile. */}
        <video
          ref={remoteVideo}
          autoPlay
          playsInline
          aria-label={`${peerLabel}'s camera`}
          className={cn(
            "aspect-video w-full bg-ink object-cover",
            !room.remoteStream && "opacity-0",
          )}
        />

        {!room.remoteStream ? (
          <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 text-center">
            <p role="status" className="text-base font-medium text-surface">
              {room.reconnecting
                ? "Reconnecting…"
                : room.state === "connecting" || room.state === "requesting-media"
                  ? "Connecting…"
                  : `Waiting for ${peerLabel} to join`}
            </p>
            <p className="max-w-[40ch] text-sm text-surface/70">
              {room.reconnecting
                ? "The connection dropped. Getting it back — no need to rejoin."
                : "They will appear here as soon as they arrive."}
            </p>
          </div>
        ) : null}

        {/* The peer's presence, top-left: mute, raised hand, their recording. */}
        <div className="absolute left-4 top-4 flex items-center gap-2">
          {!room.peerState.micOn && room.remoteStream ? (
            <span className="flex items-center gap-1.5 rounded-full bg-ink/70 px-3 py-1 text-xs text-surface">
              <MicOff size={13} aria-hidden="true" /> Muted
            </span>
          ) : null}
          {room.peerState.handRaised ? (
            <span className="flex items-center gap-1.5 rounded-full bg-ink/70 px-3 py-1 text-xs text-surface">
              <Hand size={13} aria-hidden="true" /> Hand raised
            </span>
          ) : null}
          {room.peerState.recording ? (
            <span className="flex items-center gap-1.5 rounded-full bg-danger px-3 py-1 text-xs font-medium text-on-primary">
              ● Rec
            </span>
          ) : null}
        </div>

        {/* Connection quality, top-right. */}
        {room.quality ? (
          <span
            role="img"
            aria-label={`Connection quality: ${room.quality}`}
            title={`Connection: ${room.quality}`}
            className="absolute right-4 top-4 flex items-end gap-0.5 rounded-full bg-ink/70 px-2.5 py-1.5"
          >
            {[1, 2, 3].map((bar) => (
              <span
                key={bar}
                className={cn(
                  "w-1 rounded-sm",
                  bar === 1 && "h-1.5",
                  bar === 2 && "h-2.5",
                  bar === 3 && "h-3.5",
                  (room.quality === "good" ? 3 : room.quality === "fair" ? 2 : 1) >= bar
                    ? room.quality === "poor"
                      ? "bg-danger"
                      : room.quality === "fair"
                        ? "bg-amber-400"
                        : "bg-emerald-400"
                    : "bg-surface/30",
                )}
              />
            ))}
          </span>
        ) : null}

        {/* Own recording pill, bottom-left, with elapsed time. */}
        {room.recording ? (
          <span className="absolute bottom-4 left-4 flex items-center gap-1.5 rounded-full bg-danger px-3 py-1 text-xs font-medium text-on-primary">
            ● Rec {formatClock(room.recordingSeconds)}
          </span>
        ) : null}

        {/* Own screen-share pill. */}
        {room.sharingScreen ? (
          <span className="absolute bottom-4 left-1/2 -translate-x-1/2 rounded-full bg-ink/70 px-3 py-1 text-xs text-surface">
            You are sharing your screen
          </span>
        ) : null}

        {/* Reactions float over the relevant tile. */}
        {visibleReaction ? (
          <span
            key={visibleReaction.at}
            aria-hidden="true"
            className={cn(
              "absolute z-10 animate-bounce text-4xl",
              visibleReaction.from === "peer" ? "left-6 top-1/3" : "bottom-20 right-6",
            )}
          >
            {visibleReaction.emoji}
          </span>
        ) : null}

        {/* Own tile, inset. Muted so a client does not hear themselves. */}
        <video
          ref={localVideo}
          autoPlay
          playsInline
          muted
          aria-label="Your camera"
          className={cn(
            "absolute bottom-4 right-4 aspect-video w-32 rounded-lg border border-surface/20 bg-ink object-cover sm:w-44",
            !room.cameraOn && "opacity-40",
          )}
        />

        {/* The peer's captions, along the bottom of their tile. */}
        {room.peerCaption ? (
          <p
            aria-live="polite"
            className={cn(
              "absolute inset-x-4 bottom-4 line-clamp-2 rounded-lg bg-ink/70 px-3 py-1.5 text-center text-sm text-surface",
              !room.peerCaption.final && "opacity-60",
            )}
          >
            {room.peerCaption.text}
          </p>
        ) : null}
      </div>

      {room.error ? (
        <p role="alert" className="text-sm text-danger-ink">
          {room.error}
        </p>
      ) : null}

      {/* Device settings. */}
      {settingsOpen ? (
        <div className="flex flex-col gap-3 rounded-xl border border-border bg-surface-raised p-4 sm:flex-row">
          <label className="flex flex-1 flex-col gap-1 text-xs text-ink-muted">
            Microphone
            <select
              aria-label="Microphone"
              value={room.selectedMicId ?? ""}
              onChange={(event) => room.selectMic(event.target.value)}
              className="rounded-lg border border-border bg-surface px-2 py-1.5 text-sm text-ink"
            >
              {room.mics.length === 0 ? <option value="">Default microphone</option> : null}
              {room.mics.map((device) => (
                <option key={device.deviceId} value={device.deviceId}>
                  {device.label || "Microphone"}
                </option>
              ))}
            </select>
          </label>
          <label className="flex flex-1 flex-col gap-1 text-xs text-ink-muted">
            Camera
            <select
              aria-label="Camera"
              value={room.selectedCameraId ?? ""}
              onChange={(event) => room.selectCamera(event.target.value)}
              className="rounded-lg border border-border bg-surface px-2 py-1.5 text-sm text-ink"
            >
              {room.cameras.length === 0 ? <option value="">Default camera</option> : null}
              {room.cameras.map((device) => (
                <option key={device.deviceId} value={device.deviceId}>
                  {device.label || "Camera"}
                </option>
              ))}
            </select>
          </label>
        </div>
      ) : null}

      {/* Call controls — built, not borrowed. */}
      <div className="relative flex flex-wrap items-center justify-center gap-3">
        <ControlButton
          label={room.micOn ? "Mute your microphone" : "Unmute your microphone"}
          pressed={!room.micOn}
          danger={!room.micOn}
          disabled={!live}
          onClick={room.toggleMic}
        >
          {room.micOn ? <Mic size={20} aria-hidden="true" /> : <MicOff size={20} aria-hidden="true" />}
        </ControlButton>

        <ControlButton
          label={room.cameraOn ? "Turn your camera off" : "Turn your camera on"}
          pressed={!room.cameraOn}
          danger={!room.cameraOn}
          disabled={!live}
          onClick={room.toggleCamera}
        >
          {room.cameraOn ? (
            <Video size={20} aria-hidden="true" />
          ) : (
            <VideoOff size={20} aria-hidden="true" />
          )}
        </ControlButton>

        <ControlButton
          label={room.sharingScreen ? "Stop sharing your screen" : "Share your screen"}
          pressed={room.sharingScreen}
          disabled={!live}
          onClick={room.toggleScreenShare}
        >
          <MonitorUp size={20} aria-hidden="true" />
        </ControlButton>

        <ControlButton
          label={room.recording ? "Stop recording" : "Record this session"}
          pressed={room.recording}
          danger={room.recording}
          disabled={!live || !room.recordingSupported}
          title={
            room.recordingSupported
              ? room.recording
                ? "Stop and download the recording"
                : "Record to a file on this device"
              : "Recording isn't supported in this browser"
          }
          onClick={room.toggleRecording}
        >
          {room.recording ? (
            <Square size={18} aria-hidden="true" />
          ) : (
            <Circle size={18} aria-hidden="true" />
          )}
        </ControlButton>

        <ControlButton
          label={room.handRaised ? "Lower your hand" : "Raise your hand"}
          pressed={room.handRaised}
          disabled={!live}
          onClick={room.toggleHand}
        >
          <Hand size={20} aria-hidden="true" />
        </ControlButton>

        <ControlButton
          label="Reactions"
          pressed={reactionsOpen}
          disabled={!live}
          onClick={() => setReactionsOpen((open) => !open)}
        >
          <Smile size={20} aria-hidden="true" />
        </ControlButton>

        <ControlButton
          label={chatOpen ? "Hide chat" : "Show chat"}
          pressed={chatOpen}
          disabled={!live}
          onClick={() => setChatOpen((open) => !open)}
        >
          <MessageSquare size={20} aria-hidden="true" />
          {room.unreadCount > 0 && !chatOpen ? (
            <span className="absolute -right-1 -top-1 flex size-5 items-center justify-center rounded-full bg-danger text-[0.65rem] font-medium text-on-primary">
              {room.unreadCount > 9 ? "9+" : room.unreadCount}
            </span>
          ) : null}
        </ControlButton>

        <ControlButton
          label={room.captionsSupported ? "Toggle captions" : "Captions (Chrome only)"}
          pressed={room.captionsEnabled}
          disabled={!live || !room.captionsSupported}
          title={
            room.captionsSupported
              ? "Caption this call — your side is transcribed and shared (Chrome only)"
              : "Captions need Chrome's speech recognition"
          }
          onClick={room.toggleCaptions}
        >
          <Captions size={20} aria-hidden="true" />
        </ControlButton>

        <ControlButton
          label="Camera and microphone devices"
          pressed={settingsOpen}
          disabled={!live}
          onClick={() => setSettingsOpen((open) => !open)}
        >
          <Settings2 size={20} aria-hidden="true" />
        </ControlButton>

        <ControlButton
          label={fullscreen ? "Exit fullscreen" : "Fullscreen"}
          onClick={toggleFullscreen}
        >
          <Maximize size={20} aria-hidden="true" />
        </ControlButton>

        {pipSupported ? (
          <ControlButton label="Picture in picture" disabled={!room.remoteStream} onClick={togglePictureInPicture}>
            <PictureInPicture2 size={20} aria-hidden="true" />
          </ControlButton>
        ) : null}

        <ControlButton
          label="Leave the session"
          danger
          onClick={() => {
            room.leave();
            onLeave?.();
          }}
        >
          <PhoneOff size={20} aria-hidden="true" />
        </ControlButton>

        {/* The reaction picker pops above the control bar. */}
        {reactionsOpen ? (
          <div className="absolute -top-14 left-1/2 flex -translate-x-1/2 gap-1 rounded-full border border-border bg-surface-raised px-2 py-1.5 shadow-sm">
            {REACTIONS.map((emoji) => (
              <button
                key={emoji}
                type="button"
                aria-label={`React with ${emoji}`}
                onClick={() => {
                  room.sendReaction(emoji);
                  setReactionsOpen(false);
                }}
                className="rounded-full px-1.5 text-xl transition-transform duration-instant hover:scale-125"
              >
                {emoji}
              </button>
            ))}
          </div>
        ) : null}
      </div>

      {/* Chat panel. */}
      {chatOpen ? (
        <div className="flex flex-col gap-2 rounded-xl border border-border bg-surface-raised p-3">
          <div
            aria-live="polite"
            className="flex max-h-56 flex-col gap-1.5 overflow-y-auto px-1"
          >
            {room.messages.length === 0 ? (
              <p className="py-4 text-center text-sm text-ink-muted">
                Messages here stay between the two of you and disappear when the session ends.
              </p>
            ) : (
              room.messages.map((message) => (
                <div
                  key={message.id}
                  className={cn(
                    "max-w-[80%] rounded-2xl px-3 py-1.5 text-sm leading-[1.45]",
                    message.from === "me"
                      ? "self-end bg-primary text-on-primary"
                      : "self-start bg-surface-sunken text-ink",
                  )}
                >
                  {message.text}
                </div>
              ))
            )}
          </div>
          <form
            className="flex gap-2"
            onSubmit={(event) => {
              event.preventDefault();
              submitChat();
            }}
          >
            <input
              type="text"
              value={chatDraft}
              onChange={(event) => setChatDraft(event.target.value)}
              placeholder="Write a message…"
              aria-label="Write a message"
              maxLength={500}
              className="flex-1 rounded-lg border border-border bg-surface px-3 py-2 text-sm text-ink placeholder:text-ink-muted"
            />
            <Button type="submit" disabled={!chatDraft.trim()}>
              Send
            </Button>
          </form>
        </div>
      ) : null}
    </div>
  );
}
