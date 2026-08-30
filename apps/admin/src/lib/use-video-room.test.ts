import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { useVideoRoom } from "@/lib/use-video-room";

/**
 * The video room's lifecycle (CX-05).
 *
 * The order of operations here is the whole design, and it is invisible
 * from the UI: entry is authorized *before* the camera is asked for, so a
 * client turned away at the door is never made to grant a permission they
 * did not need. And the camera is released on unmount, because a light
 * still on after a session is over is the kind of thing a client notices
 * and does not forget.
 */

const join = vi.hoisted(() => vi.fn());

vi.mock("@/lib/video", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/video")>();
  return { ...original, videoApi: { join } };
});

vi.mock("@/lib/auth", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/auth")>();
  const value = {
    status: "authenticated",
    user: { id: "client-1", email: "a@example.com", role: "client", name: "Ama" },
    session: { accessToken: "a1", refreshToken: "r1" },
    refreshCallbacks: { onTokensRefreshed: vi.fn() },
    logout: vi.fn(),
  };
  return { ...original, useAuth: () => value };
});

/** A track that records whether it was actually stopped. */
function fakeTrack(kind: "audio" | "video") {
  return {
    kind,
    enabled: true,
    readyState: "live" as string,
    stop: vi.fn(function (this: { readyState: string }) {
      this.readyState = "ended";
    }),
  };
}

let tracks: ReturnType<typeof fakeTrack>[];
let sockets: FakeSocket[];

class FakeSocket {
  static OPEN = 1;
  readyState = 1;
  sent: string[] = [];
  onmessage: ((event: { data: string }) => void) | null = null;
  onerror: (() => void) | null = null;
  onclose: (() => void) | null = null;
  close = vi.fn(() => {
    this.readyState = 3;
  });
  send = vi.fn((data: string) => this.sent.push(data));

  constructor(readonly url: string) {
    sockets.push(this);
  }
}

class FakePeerConnection {
  static instances: FakePeerConnection[] = [];
  localDescription: unknown = null;
  remoteDescription: unknown = null;
  onicecandidate: ((event: { candidate: unknown }) => void) | null = null;
  ontrack: ((event: { streams: MediaStream[] }) => void) | null = null;
  onconnectionstatechange: (() => void) | null = null;
  connectionState = "new";
  addTrack = vi.fn();
  close = vi.fn(() => {
    this.connectionState = "closed";
  });
  createOffer = vi.fn(async () => ({ type: "offer", sdp: "o" }));
  createAnswer = vi.fn(async () => ({ type: "answer", sdp: "a" }));
  setLocalDescription = vi.fn(async (d: unknown) => {
    this.localDescription = d;
  });
  setRemoteDescription = vi.fn(async (d: unknown) => {
    this.remoteDescription = d;
  });
  addIceCandidate = vi.fn(async () => {});
  getStats = vi.fn(async () => new Map());

  constructor() {
    FakePeerConnection.instances.push(this);
  }
}

const access = {
  bookingId: "booking-1",
  role: "client" as const,
  ticket: "tkt",
  ticketExpiresIn: 60,
  opensAt: "2026-08-12T09:00:00Z",
  closesAt: "2026-08-12T10:15:00Z",
  iceServers: [{ urls: ["stun:stun.example:3478"] }],
};

describe("useVideoRoom", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    tracks = [fakeTrack("audio"), fakeTrack("video")];
    sockets = [];
    FakePeerConnection.instances = [];

    const stream = {
      getTracks: () => tracks,
      getAudioTracks: () => tracks.filter((t) => t.kind === "audio"),
      getVideoTracks: () => tracks.filter((t) => t.kind === "video"),
    };

    join.mockResolvedValue(access);
    vi.stubGlobal("WebSocket", FakeSocket);
    vi.stubGlobal("RTCPeerConnection", FakePeerConnection);
    vi.stubGlobal("navigator", {
      ...globalThis.navigator,
      mediaDevices: { getUserMedia: vi.fn().mockResolvedValue(stream) },
    });
  });

  afterEach(() => vi.unstubAllGlobals());

  it("starts idle and asks for nothing", () => {
    const { result } = renderHook(() => useVideoRoom("booking-1"));

    expect(result.current.state).toBe("idle");
    expect(navigator.mediaDevices.getUserMedia).not.toHaveBeenCalled();
    expect(join).not.toHaveBeenCalled();
  });

  it("authorizes before touching the camera", async () => {
    const { result } = renderHook(() => useVideoRoom("booking-1"));
    act(() => result.current.join());

    await waitFor(() => expect(result.current.state).toBe("connecting"));

    // The order is the point: a client turned away at the door is never
    // asked for a permission they did not need.
    const authorizedAt = join.mock.invocationCallOrder[0]!;
    const cameraAt = (navigator.mediaDevices.getUserMedia as ReturnType<typeof vi.fn>).mock
      .invocationCallOrder[0]!;
    expect(authorizedAt).toBeLessThan(cameraAt);
  });

  it("never opens the camera when entry is refused", async () => {
    join.mockRejectedValue(new ApiError(403, "room_not_open", "the video room is not open yet"));
    const { result } = renderHook(() => useVideoRoom("booking-1"));

    act(() => result.current.join());

    await waitFor(() => expect(result.current.state).toBe("failed"));
    expect(navigator.mediaDevices.getUserMedia).not.toHaveBeenCalled();
    expect(result.current.error).toMatch(/room isn't open yet/i);
  });

  it("explains a refused camera without blaming the session", async () => {
    (navigator.mediaDevices.getUserMedia as ReturnType<typeof vi.fn>).mockRejectedValue(
      new DOMException("Permission denied", "NotAllowedError"),
    );
    const { result } = renderHook(() => useVideoRoom("booking-1"));

    act(() => result.current.join());

    await waitFor(() => expect(result.current.state).toBe("failed"));
    expect(result.current.error).toMatch(/camera or microphone/i);
    expect(sockets).toHaveLength(0);
  });

  it("carries the ticket on the socket URL, not a header", async () => {
    const { result } = renderHook(() => useVideoRoom("booking-1"));
    act(() => result.current.join());

    await waitFor(() => expect(sockets).toHaveLength(1));
    // A browser cannot put a bearer token on a WebSocket handshake, which
    // is the entire reason single-use tickets exist.
    expect(sockets[0]!.url).toContain("tkt");
  });

  it("toggles the microphone on the track, not by re-asking for media", async () => {
    const { result } = renderHook(() => useVideoRoom("booking-1"));
    act(() => result.current.join());
    await waitFor(() => expect(result.current.localStream).not.toBeNull());

    act(() => result.current.toggleMic());

    expect(result.current.micOn).toBe(false);
    expect(tracks[0]!.enabled).toBe(false);
    // Re-prompting mid-call would put a permission dialog over the session.
    expect(navigator.mediaDevices.getUserMedia).toHaveBeenCalledTimes(1);
  });

  it("toggles the camera the same way", async () => {
    const { result } = renderHook(() => useVideoRoom("booking-1"));
    act(() => result.current.join());
    await waitFor(() => expect(result.current.localStream).not.toBeNull());

    act(() => result.current.toggleCamera());

    expect(result.current.cameraOn).toBe(false);
    expect(tracks[1]!.enabled).toBe(false);
  });

  it("releases the camera when the client leaves", async () => {
    const { result } = renderHook(() => useVideoRoom("booking-1"));
    act(() => result.current.join());
    await waitFor(() => expect(result.current.localStream).not.toBeNull());

    act(() => result.current.leave());

    expect(result.current.state).toBe("ended");
    for (const track of tracks) {
      expect(track.readyState).toBe("ended");
    }
    expect(sockets[0]!.close).toHaveBeenCalled();
    expect(FakePeerConnection.instances[0]!.close).toHaveBeenCalled();
  });

  it("releases the camera when the page navigates away mid-call", async () => {
    const { result, unmount } = renderHook(() => useVideoRoom("booking-1"));
    act(() => result.current.join());
    await waitFor(() => expect(result.current.localStream).not.toBeNull());

    unmount();

    // Closing the tab is not the only way out of a call. A camera light
    // still on after navigating away is the failure worth guarding.
    for (const track of tracks) {
      expect(track.stop).toHaveBeenCalled();
    }
  });

  it("treats a socket close after a working call as the session ending", async () => {
    const { result } = renderHook(() => useVideoRoom("booking-1"));
    act(() => result.current.join());
    await waitFor(() => expect(sockets).toHaveLength(1));

    // "Connected" means the other person's media arrived, not that a
    // transport opened. A room that negotiated but shows nothing is not a
    // session anyone would call connected.
    const peer = FakePeerConnection.instances[0]!;
    const remote = { getTracks: () => [] } as unknown as MediaStream;
    act(() => peer.ontrack?.({ streams: [remote] }));
    await waitFor(() => expect(result.current.state).toBe("connected"));

    act(() => sockets[0]!.onclose?.());

    expect(result.current.state).toBe("ended");
    expect(result.current.error).toBeNull();
  });

  it("treats a socket close before a call as a failure worth reporting", async () => {
    const { result } = renderHook(() => useVideoRoom("booking-1"));
    act(() => result.current.join());
    await waitFor(() => expect(sockets).toHaveLength(1));

    act(() => sockets[0]!.onerror?.());

    expect(result.current.state).toBe("failed");
    expect(result.current.error).toMatch(/interrupted/i);
  });

  it("reports a dropped connection and says rejoining usually fixes it", async () => {
    const { result } = renderHook(() => useVideoRoom("booking-1"));
    act(() => result.current.join());
    await waitFor(() => expect(FakePeerConnection.instances).toHaveLength(1));

    const peer = FakePeerConnection.instances[0]!;
    act(() => {
      peer.connectionState = "failed";
      peer.onconnectionstatechange?.();
    });

    expect(result.current.state).toBe("failed");
    expect(result.current.error).toMatch(/rejoining/i);
  });

  it("ignores a malformed signal rather than tearing the room down", async () => {
    const { result } = renderHook(() => useVideoRoom("booking-1"));
    act(() => result.current.join());
    await waitFor(() => expect(sockets).toHaveLength(1));

    act(() => sockets[0]!.onmessage?.({ data: "not json" }));

    expect(result.current.state).toBe("connecting");
    expect(result.current.error).toBeNull();
  });
});

describe("useVideoRoom collaboration and recovery", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    tracks = [fakeTrack("audio"), fakeTrack("video")];
    sockets = [];
    FakePeerConnection.instances = [];

    const stream = {
      getTracks: () => tracks,
      getAudioTracks: () => tracks.filter((t) => t.kind === "audio"),
      getVideoTracks: () => tracks.filter((t) => t.kind === "video"),
    };

    join.mockResolvedValue(access);
    vi.stubGlobal("WebSocket", FakeSocket);
    vi.stubGlobal("RTCPeerConnection", FakePeerConnection);
    vi.stubGlobal("navigator", {
      ...globalThis.navigator,
      mediaDevices: { getUserMedia: vi.fn().mockResolvedValue(stream) },
    });
  });

  afterEach(() => vi.unstubAllGlobals());

  it("rejoins automatically when the socket drops mid-call", async () => {
    // A room still inside its window: the close is a drop, not the ending.
    const future = new Date(Date.now() + 30 * 60 * 1000).toISOString();
    join.mockResolvedValue({ ...access, closesAt: future });

    const { result } = renderHook(() => useVideoRoom("booking-1"));
    act(() => result.current.join());
    await waitFor(() => expect(sockets).toHaveLength(1));

    const peer = FakePeerConnection.instances[0]!;
    const remote = { getTracks: () => [] } as unknown as MediaStream;
    act(() => peer.ontrack?.({ streams: [remote] }));
    await waitFor(() => expect(result.current.state).toBe("connected"));

    act(() => sockets[0]!.onclose?.());

    expect(result.current.reconnecting).toBe(true);
    // The first backoff delay is one second; a fresh ticket is requested
    // and a new socket opened, without touching the camera again.
    await waitFor(() => expect(join.mock.calls.length).toBe(2), { timeout: 4000 });
    await waitFor(() => expect(sockets.length).toBe(2), { timeout: 4000 });
    expect(navigator.mediaDevices.getUserMedia).toHaveBeenCalledTimes(1);
  });

  it("treats a socket close past the window as the session ending", async () => {
    const { result } = renderHook(() => useVideoRoom("booking-1"));
    act(() => result.current.join());
    await waitFor(() => expect(sockets).toHaveLength(1));

    const peer = FakePeerConnection.instances[0]!;
    const remote = { getTracks: () => [] } as unknown as MediaStream;
    act(() => peer.ontrack?.({ streams: [remote] }));
    await waitFor(() => expect(result.current.state).toBe("connected"));

    // The shared fixture's window is in the past.
    act(() => sockets[0]!.onclose?.());

    expect(result.current.state).toBe("ended");
    expect(result.current.reconnecting).toBe(false);
    expect(join).toHaveBeenCalledTimes(1);
  });

  it("sends chat as a relayed envelope and receives the peer's", async () => {
    const { result } = renderHook(() => useVideoRoom("booking-1"));
    act(() => result.current.join());
    await waitFor(() => expect(sockets).toHaveLength(1));

    act(() => result.current.sendChat("  see you soon  "));
    expect(result.current.messages).toHaveLength(1);
    expect(result.current.messages[0]).toMatchObject({ from: "me", text: "see you soon" });
    const sent = JSON.parse(sockets[0]!.sent[0]!);
    expect(sent).toMatchObject({ type: "chat", payload: { text: "see you soon" } });

    act(() =>
      sockets[0]!.onmessage?.({
        data: JSON.stringify({ type: "chat", payload: { text: "hello" } }),
      }),
    );
    expect(result.current.messages).toHaveLength(2);
    expect(result.current.messages[1]).toMatchObject({ from: "peer", text: "hello" });
    expect(result.current.unreadCount).toBe(1);

    act(() => result.current.markChatRead());
    expect(result.current.unreadCount).toBe(0);
  });

  it("relays presence on toggle and adopts the peer's", async () => {
    const { result } = renderHook(() => useVideoRoom("booking-1"));
    act(() => result.current.join());
    await waitFor(() => expect(sockets).toHaveLength(1));

    act(() => result.current.toggleMic());
    const sent = JSON.parse(sockets[0]!.sent[0]!);
    expect(sent).toMatchObject({ type: "state", payload: { micOn: false } });

    act(() =>
      sockets[0]!.onmessage?.({
        data: JSON.stringify({ type: "state", payload: { micOn: false, handRaised: true } }),
      }),
    );
    expect(result.current.peerState.micOn).toBe(false);
    expect(result.current.peerState.handRaised).toBe(true);
    // Unmentioned fields fall back to the defaults, never to undefined.
    expect(result.current.peerState.recording).toBe(false);
  });

  it("keeps a client in the waiting room until the practitioner admits them", async () => {
    const { result } = renderHook(() => useVideoRoom("booking-1"));
    act(() => result.current.join());
    await waitFor(() => expect(sockets).toHaveLength(1));

    act(() =>
      sockets[0]!.onmessage?.({
        data: JSON.stringify({
          type: "joined",
          payload: { peers: [{ peerId: "practitioner", role: "practitioner" }] },
        }),
      }),
    );

    expect(result.current.waitingForAdmission).toBe(true);
    expect(JSON.parse(sockets[0]!.sent.at(-1)!)).toMatchObject({ type: "admission-request" });
    expect(FakePeerConnection.instances[0]!.createOffer).not.toHaveBeenCalled();

    act(() => sockets[0]!.onmessage?.({ data: JSON.stringify({ type: "admission-granted" }) }));
    await waitFor(() => expect(FakePeerConnection.instances[0]!.createOffer).toHaveBeenCalled());
    expect(result.current.waitingForAdmission).toBe(false);
  });

  it("lets only the practitioner actively admit the waiting client", async () => {
    join.mockResolvedValue({ ...access, role: "practitioner" });
    const { result } = renderHook(() => useVideoRoom("booking-1"));
    act(() => result.current.join());
    await waitFor(() => expect(sockets).toHaveLength(1));

    act(() =>
      sockets[0]!.onmessage?.({
        data: JSON.stringify({
          type: "joined",
          payload: { peers: [{ peerId: "client", role: "client" }] },
        }),
      }),
    );
    expect(result.current.admissionRequested).toBe(true);

    act(() => result.current.admitClient());
    expect(JSON.parse(sockets[0]!.sent.at(-1)!)).toMatchObject({ type: "admission-granted" });
    await waitFor(() => expect(FakePeerConnection.instances[0]!.createOffer).toHaveBeenCalled());
  });

  it("requests recording consent and handles a decline without recording", async () => {
    vi.stubGlobal("MediaRecorder", class {});
    const { result } = renderHook(() => useVideoRoom("booking-1"));
    act(() => result.current.join());
    await waitFor(() => expect(sockets).toHaveLength(1));
    const remote = { getTracks: () => [], getAudioTracks: () => [], getVideoTracks: () => [] } as unknown as MediaStream;
    act(() => FakePeerConnection.instances[0]!.ontrack?.({ streams: [remote] }));

    act(() => result.current.toggleRecording());
    expect(result.current.recordingConsentPending).toBe(true);
    expect(result.current.recording).toBe(false);
    expect(JSON.parse(sockets[0]!.sent.at(-1)!)).toMatchObject({ type: "recording-request" });

    act(() => sockets[0]!.onmessage?.({ data: JSON.stringify({ type: "recording-consent", payload: { approved: false } }) }));
    expect(result.current.recordingConsentPending).toBe(false);
    expect(result.current.error).toMatch(/declined/i);
  });

  it("ignores unsolicited recording approval", async () => {
    vi.stubGlobal("MediaRecorder", class {});
    const { result } = renderHook(() => useVideoRoom("booking-1"));
    act(() => result.current.join());
    await waitFor(() => expect(sockets).toHaveLength(1));

    act(() => sockets[0]!.onmessage?.({ data: JSON.stringify({ type: "recording-consent", payload: { approved: true } }) }));
    expect(result.current.recording).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it("releases local media when admission is denied", async () => {
    const { result } = renderHook(() => useVideoRoom("booking-1"));
    act(() => result.current.join());
    await waitFor(() => expect(sockets).toHaveLength(1));

    act(() => sockets[0]!.onmessage?.({ data: JSON.stringify({ type: "admission-denied" }) }));
    expect(result.current.state).toBe("failed");
    expect(result.current.localStream).toBeNull();
    expect(tracks.every((track) => track.stop.mock.calls.length > 0)).toBe(true);
  });

  it("answers recording requests and handles peer departure", async () => {
    const { result } = renderHook(() => useVideoRoom("booking-1"));
    act(() => result.current.join());
    await waitFor(() => expect(sockets).toHaveLength(1));

    act(() => sockets[0]!.onmessage?.({ data: JSON.stringify({ type: "recording-request" }) }));
    expect(result.current.recordingConsentRequested).toBe(true);
    act(() => result.current.respondToRecordingRequest(true));
    expect(JSON.parse(sockets[0]!.sent.at(-1)!)).toMatchObject({ type: "recording-consent", payload: { approved: true } });

    act(() => sockets[0]!.onmessage?.({ data: JSON.stringify({ type: "peer-left" }) }));
    expect(result.current.state).toBe("waiting");
    expect(result.current.remoteStream).toBeNull();
  });

  it("honors solicited recording approval and handles an ended session", async () => {
    vi.stubGlobal("MediaRecorder", class {});
    const { result } = renderHook(() => useVideoRoom("booking-1"));
    act(() => result.current.join());
    await waitFor(() => expect(sockets).toHaveLength(1));
    const remote = { getTracks: () => [], getAudioTracks: () => [], getVideoTracks: () => [] } as unknown as MediaStream;
    act(() => FakePeerConnection.instances[0]!.ontrack?.({ streams: [remote] }));

    act(() => result.current.toggleRecording());
    act(() => sockets[0]!.onmessage?.({ data: JSON.stringify({ type: "recording-consent", payload: { approved: true } }) }));
    expect(result.current.recordingConsentPending).toBe(false);
    expect(result.current.error).toMatch(/couldn't start/i);

    act(() => sockets[0]!.onmessage?.({ data: JSON.stringify({ type: "session-ended" }) }));
    expect(result.current.state).toBe("ended");
  });

  it("allows the practitioner to end the call for both participants", async () => {
    const { result } = renderHook(() => useVideoRoom("booking-1"));
    act(() => result.current.join());
    await waitFor(() => expect(sockets).toHaveLength(1));

    act(() => result.current.endForAll());
    expect(JSON.parse(sockets[0]!.sent.at(-1)!)).toMatchObject({ type: "session-ended" });
    expect(result.current.state).toBe("ended");
  });

  it("exposes the e2e debug surface after joining", async () => {
    const { result } = renderHook(() => useVideoRoom("booking-1"));
    act(() => result.current.join());
    await waitFor(() => expect(sockets).toHaveLength(1));

    const w = window as unknown as Record<string, unknown>;
    expect(w.__peerConnection).toBe(FakePeerConnection.instances[0]);
    expect(w.__localStream).toBe(result.current.localStream);
    expect(w.__iceServers).toEqual(access.iceServers);
  });
});
