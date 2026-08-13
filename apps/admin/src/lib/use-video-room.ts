"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError } from "./api";
import { useAuth } from "./auth";
import {
  CHAT_MAX_LEN,
  explainJoinFailure,
  hasSpeechRecognition,
  initialPeerState,
  reconnectDelays,
  scoreConnection,
  shouldOffer,
  signalUrl,
  videoApi,
  type CaptionPayload,
  type ChatMessage,
  type ChatPayload,
  type ConnectionQuality,
  type PeerState,
  type ReactionPayload,
  type SessionAccess,
  type SignalEnvelope,
} from "./video";

/**
 * The video room's whole lifecycle in one hook (CX-06; the portal has
 * its own copy — the two apps deploy separately and share no package).
 *
 * The sequence is: ask the API for permission and a ticket, take the
 * camera, open the socket, then negotiate. Permission comes first so a
 * client who is too early is told so before their camera light comes on —
 * asking for a camera and then refusing entry is a rude order to do things
 * in.
 *
 * Reconnection is automatic where recovery is possible: a dropped socket
 * re-runs the sequence with a fresh ticket (reusing the live camera stream,
 * so no new permission prompt) on a bounded backoff, and a failed ICE
 * transport is restarted in place first. The room treats a returning person
 * as a replacement rather than a second occupant. Manual rejoin remains as
 * the fallback once the backoff is exhausted.
 */

export type RoomState =
  | "idle"
  | "authorizing"
  | "requesting-media"
  | "connecting"
  | "waiting"
  | "connected"
  | "ended"
  | "failed";

export interface JoinOptions {
  /** false joins audio-only; the camera can be added later from devices. */
  video?: boolean;
}

export interface ActiveReaction {
  emoji: string;
  from: "me" | "peer";
  at: number;
}

export interface VideoRoom {
  state: RoomState;
  error: string | null;
  /** The caller's own camera feed. */
  localStream: MediaStream | null;
  /** The other participant's feed, once negotiated. */
  remoteStream: MediaStream | null;
  access: SessionAccess | null;
  /** True while an automatic rejoin is in progress. */
  reconnecting: boolean;
  micOn: boolean;
  cameraOn: boolean;
  toggleMic: () => void;
  toggleCamera: () => void;
  join: (options?: JoinOptions) => void;
  leave: () => void;
  /** Screen share — the camera track is parked and restored afterwards. */
  sharingScreen: boolean;
  toggleScreenShare: () => void;
  /** In-call chat. unreadCount resets via markChatRead (panel open). */
  messages: ChatMessage[];
  unreadCount: number;
  sendChat: (text: string) => void;
  markChatRead: () => void;
  /** Presence: the peer's last relayed state, and our own hand. */
  peerState: PeerState;
  handRaised: boolean;
  toggleHand: () => void;
  /** The most recent reaction, either side's; the UI fades it out. */
  activeReaction: ActiveReaction | null;
  sendReaction: (emoji: string) => void;
  /** Input devices, listed once permission has made labels available. */
  mics: MediaDeviceInfo[];
  cameras: MediaDeviceInfo[];
  selectedMicId: string | null;
  selectedCameraId: string | null;
  selectMic: (deviceId: string) => void;
  selectCamera: (deviceId: string) => void;
  /** Connection quality from getStats, refreshed while connected. */
  quality: ConnectionQuality | null;
  /** Local recording (MediaRecorder → .webm download on stop). */
  recordingSupported: boolean;
  recording: boolean;
  recordingSeconds: number;
  toggleRecording: () => void;
  /** Chrome-only captions: own mic transcribed and relayed to the peer. */
  captionsSupported: boolean;
  captionsEnabled: boolean;
  toggleCaptions: () => void;
  peerCaption: CaptionPayload | null;
}

const AUDIO_CONSTRAINTS = {
  echoCancellation: true,
  noiseSuppression: true,
  autoGainControl: true,
} as const;

/** How long an ICE transport may sit at "disconnected" before a restart. */
const ICE_DISCONNECTED_GRACE_MS = 5000;

/** How often connection quality is sampled while the call is live. */
const STATS_INTERVAL_MS = 3000;

export function useVideoRoom(bookingId: string): VideoRoom {
  const { session, refreshCallbacks } = useAuth();

  const [state, setState] = useState<RoomState>("idle");
  const [error, setError] = useState<string | null>(null);
  const [access, setAccess] = useState<SessionAccess | null>(null);
  const [localStream, setLocalStream] = useState<MediaStream | null>(null);
  const [remoteStream, setRemoteStream] = useState<MediaStream | null>(null);
  const [reconnecting, setReconnecting] = useState(false);
  const [micOn, setMicOn] = useState(true);
  const [cameraOn, setCameraOn] = useState(true);
  const [sharingScreen, setSharingScreen] = useState(false);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [peerState, setPeerState] = useState<PeerState>(initialPeerState);
  const [handRaised, setHandRaised] = useState(false);
  const [activeReaction, setActiveReaction] = useState<ActiveReaction | null>(null);
  const [mics, setMics] = useState<MediaDeviceInfo[]>([]);
  const [cameras, setCameras] = useState<MediaDeviceInfo[]>([]);
  const [selectedMicId, setSelectedMicId] = useState<string | null>(null);
  const [selectedCameraId, setSelectedCameraId] = useState<string | null>(null);
  const [quality, setQuality] = useState<ConnectionQuality | null>(null);
  const [recording, setRecording] = useState(false);
  const [recordingSeconds, setRecordingSeconds] = useState(0);
  const [captionsEnabled, setCaptionsEnabled] = useState(false);
  const [peerCaption, setPeerCaption] = useState<CaptionPayload | null>(null);

  const socketRef = useRef<WebSocket | null>(null);
  const peerRef = useRef<RTCPeerConnection | null>(null);
  const localRef = useRef<MediaStream | null>(null);
  const remoteRef = useRef<MediaStream | null>(null);
  // Candidates can arrive before the remote description is set; holding
  // them is simpler and more reliable than hoping they never do.
  const pendingCandidates = useRef<RTCIceCandidateInit[]>([]);

  const sessionRef = useRef(session);
  const stateRef = useRef<RoomState>(state);
  const accessRef = useRef<SessionAccess | null>(null);
  const intentionalLeaveRef = useRef(false);
  // A rejoin supersedes the old socket; the generation lets stale handlers
  // tell they belong to a dead connection.
  const generationRef = useRef(0);
  const attemptsRef = useRef(0);
  const offererRef = useRef(false);
  const iceRestartAttemptedRef = useRef(false);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const iceGraceTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const statsTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const screenTrackRef = useRef<MediaStreamTrack | null>(null);
  const parkedCameraTrackRef = useRef<MediaStreamTrack | null>(null);
  const recorderRef = useRef<MediaRecorder | null>(null);
  const recordChunksRef = useRef<Blob[]>([]);
  const recordContextRef = useRef<AudioContext | null>(null);
  const recordTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const recognitionRef = useRef<SpeechRecognitionLike | null>(null);

  // Refs mirrored from state, so stable callbacks read current values
  // without re-subscribing every handler on every toggle.
  const micOnRef = useRef(true);
  const cameraOnRef = useRef(true);
  const handRaisedRef = useRef(false);
  const recordingRef = useRef(false);
  const captionsEnabledRef = useRef(false);

  useEffect(() => {
    sessionRef.current = session;
  }, [session]);
  useEffect(() => {
    stateRef.current = state;
  }, [state]);

  const send = useCallback((envelope: SignalEnvelope) => {
    const socket = socketRef.current;
    if (socket?.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify(envelope));
    }
  }, []);

  /** Relays the caller's presence so the peer's tile can show it. */
  const emitState = useCallback(() => {
    send({
      type: "state",
      payload: {
        micOn: micOnRef.current,
        cameraOn: cameraOnRef.current,
        handRaised: handRaisedRef.current,
        recording: recordingRef.current,
      } satisfies PeerState,
    });
  }, [send]);

  /** Test-only surface for the e2e suite: a DOM element proves nothing
   * about a WebRTC transport, so the suite reads these instead. */
  const exposeDebug = useCallback(() => {
    if (typeof window === "undefined") return;
    const w = window as unknown as Record<string, unknown>;
    w.__peerConnection = peerRef.current ?? undefined;
    w.__peerConnectionState = peerRef.current?.connectionState ?? "none";
    w.__localStream = localRef.current ?? undefined;
    w.__iceServers = accessRef.current?.iceServers ?? undefined;
  }, []);

  const clearTimers = useCallback(() => {
    if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current);
    if (iceGraceTimerRef.current) clearTimeout(iceGraceTimerRef.current);
    if (statsTimerRef.current) clearInterval(statsTimerRef.current);
    if (recordTimerRef.current) clearInterval(recordTimerRef.current);
    reconnectTimerRef.current = null;
    iceGraceTimerRef.current = null;
    statsTimerRef.current = null;
    recordTimerRef.current = null;
  }, []);

  const stopRecordingInternals = useCallback(() => {
    // Stopping the recorder finalizes the download in its onstop handler.
    if (recorderRef.current && recorderRef.current.state !== "inactive") {
      recorderRef.current.stop();
    }
    recorderRef.current = null;
    void recordContextRef.current?.close().catch(() => {});
    recordContextRef.current = null;
    if (recordTimerRef.current) clearInterval(recordTimerRef.current);
    recordTimerRef.current = null;
  }, []);

  const stopCaptionsInternals = useCallback(() => {
    captionsEnabledRef.current = false;
    try {
      recognitionRef.current?.stop();
    } catch {
      // A recognizer that never started throws on stop — harmless.
    }
    recognitionRef.current = null;
  }, []);

  const teardown = useCallback(() => {
    intentionalLeaveRef.current = true;
    generationRef.current += 1; // stale socket handlers now no-op
    clearTimers();

    socketRef.current?.close();
    socketRef.current = null;

    peerRef.current?.close();
    peerRef.current = null;

    screenTrackRef.current?.stop();
    screenTrackRef.current = null;
    parkedCameraTrackRef.current = null;

    stopRecordingInternals();
    stopCaptionsInternals();

    localRef.current?.getTracks().forEach((track) => track.stop());
    localRef.current = null;
    remoteRef.current = null;

    pendingCandidates.current = [];
    exposeDebug();
    setLocalStream(null);
    setRemoteStream(null);
    setQuality(null);
    setSharingScreen(false);
    setRecording(false);
    setRecordingSeconds(0);
    setPeerCaption(null);
  }, [clearTimers, exposeDebug, stopCaptionsInternals, stopRecordingInternals]);

  // Leaving the page must release the camera. Without this the indicator
  // light stays on after a client navigates away, which is alarming and
  // entirely our fault.
  useEffect(() => teardown, [teardown]);

  // Candidates that arrived before the remote description was set. They
  // are held rather than dropped: ICE sends them as it finds them, and a
  // candidate discarded for arriving early is a connection path lost.
  const drainCandidates = useCallback(async (peer: RTCPeerConnection) => {
    const queued = pendingCandidates.current;
    pendingCandidates.current = [];
    for (const candidate of queued) {
      await peer.addIceCandidate(candidate).catch(() => {
        // A candidate the browser rejects is not fatal: ICE tries the rest.
      });
    }
  }, []);

  const makeOffer = useCallback(
    async (peer: RTCPeerConnection) => {
      const offer = await peer.createOffer();
      await peer.setLocalDescription(offer);
      send({ type: "offer", payload: offer });
    },
    [send],
  );

  /** Restarts a failed ICE transport in place. Only the offering side
   * re-offers, so both sides never offer at once (the glare rule). */
  const attemptIceRecovery = useCallback(
    (peer: RTCPeerConnection) => {
      if (typeof peer.restartIce !== "function" || iceRestartAttemptedRef.current) {
        setError("The connection dropped and could not recover. Rejoining usually fixes it.");
        setState("failed");
        return;
      }
      iceRestartAttemptedRef.current = true;
      peer.restartIce();
      if (offererRef.current) void makeOffer(peer);
    },
    [makeOffer],
  );

  /** Samples the selected pair's RTT and inbound loss into a quality score. */
  function sampleStats(peer: RTCPeerConnection) {
    void peer
      .getStats()
      .then((report) => {
        let rttMs: number | null = null;
        let lost = 0;
        let received = 0;
        report.forEach((stat) => {
          const s = stat as {
            type?: string;
            state?: string;
            currentRoundTripTime?: number;
            packetsLost?: number;
            packetsReceived?: number;
          };
          if (s.type === "candidate-pair" && s.state === "succeeded" && s.currentRoundTripTime) {
            rttMs = s.currentRoundTripTime * 1000;
          }
          if (s.type === "inbound-rtp") {
            lost += s.packetsLost ?? 0;
            received += s.packetsReceived ?? 0;
          }
        });
        if (rttMs === null) return;
        const total = lost + received;
        setQuality(scoreConnection(rttMs, total > 0 ? lost / total : 0));
      })
      .catch(() => {
        // A stats failure says nothing useful; keep the last reading.
      });
  }

  const buildPeer = useCallback(
    (granted: SessionAccess, stream: MediaStream) => {
      const peer = new RTCPeerConnection({
        iceServers: granted.iceServers.map((server) => ({
          urls: server.urls,
          username: server.username,
          credential: server.credential,
        })),
      });

      stream.getTracks().forEach((track) => peer.addTrack(track, stream));

      peer.ontrack = (event) => {
        remoteRef.current = event.streams[0] ?? null;
        setRemoteStream(event.streams[0] ?? null);
        setState("connected");
      };

      peer.onicecandidate = (event) => {
        if (event.candidate) {
          send({ type: "candidate", payload: event.candidate.toJSON() });
        }
      };

      peer.onconnectionstatechange = () => {
        exposeDebug();
        if (peer.connectionState === "connected") {
          // A working transport resets both recovery budgets and confirms
          // the peer our presence (they may have joined mid-toggle).
          attemptsRef.current = 0;
          iceRestartAttemptedRef.current = false;
          setReconnecting(false);
          emitState();
        } else if (peer.connectionState === "failed") {
          attemptIceRecovery(peer);
        }
      };

      peer.oniceconnectionstatechange = () => {
        if (peer.iceConnectionState === "disconnected") {
          // Brief drops recover on their own; a lasting one gets a restart.
          if (iceGraceTimerRef.current) clearTimeout(iceGraceTimerRef.current);
          iceGraceTimerRef.current = setTimeout(() => {
            if (peer.iceConnectionState === "disconnected") attemptIceRecovery(peer);
          }, ICE_DISCONNECTED_GRACE_MS);
        } else if (iceGraceTimerRef.current) {
          clearTimeout(iceGraceTimerRef.current);
          iceGraceTimerRef.current = null;
        }
      };

      if (statsTimerRef.current) clearInterval(statsTimerRef.current);
      statsTimerRef.current = setInterval(() => {
        if (peerRef.current === peer && peer.connectionState === "connected") {
          sampleStats(peer);
        }
      }, STATS_INTERVAL_MS);

      peerRef.current = peer;
      exposeDebug();
      return peer;
    },
    [attemptIceRecovery, emitState, exposeDebug, send],
  );

  const scheduleRejoin = useCallback(() => {
    if (attemptsRef.current >= reconnectDelays.length) {
      setReconnecting(false);
      setError("The connection dropped and could not recover. Rejoining usually fixes it.");
      setState("failed");
      return;
    }
    const delay = reconnectDelays[attemptsRef.current];
    attemptsRef.current += 1;
    setReconnecting(true);
    reconnectTimerRef.current = setTimeout(() => {
      void (async () => {
        const currentSession = sessionRef.current;
        const stream = localRef.current;
        if (!currentSession || !stream || intentionalLeaveRef.current) return;
        try {
          // A fresh ticket for the same camera stream: the room treats a
          // returning person as a replacement, not a second occupant.
          const granted = await videoApi.join(
            currentSession,
            refreshCallbacks,
            bookingId,
          );
          accessRef.current = granted;
          setAccess(granted);
          connect(granted, stream);
        } catch {
          scheduleRejoin();
        }
      })();
    }, delay);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [bookingId, refreshCallbacks]);

  /** Opens the signaling socket and its peer connection for one ticket. */
  function connect(granted: SessionAccess, stream: MediaStream) {
    const generation = generationRef.current + 1;
    generationRef.current = generation;
    const peer = buildPeer(granted, stream);
    const socket = new WebSocket(signalUrl(bookingId, granted.ticket));
    socketRef.current = socket;

    socket.onmessage = (event) => {
      let envelope: SignalEnvelope;
      try {
        envelope = JSON.parse(event.data as string) as SignalEnvelope;
      } catch {
        return;
      }
      void handleSignal(envelope);
    };
    socket.onerror = () => {
      if (generation !== generationRef.current) return;
      setError("The connection to the session was interrupted.");
      setState("failed");
    };
    socket.onclose = () => {
      if (generation !== generationRef.current || intentionalLeaveRef.current) return;
      peer.close();
      const live = ["connected", "waiting", "connecting"].includes(stateRef.current);
      if (!live) return;
      // A close past the window is the session ending; anything earlier is
      // a drop worth recovering from.
      const closesAt = accessRef.current?.closesAt;
      if (closesAt && Date.now() > new Date(closesAt).getTime()) {
        setState("ended");
        return;
      }
      setState("connecting");
      scheduleRejoin();
    };
  }

  const handleSignal = useCallback(
    async (envelope: SignalEnvelope) => {
      const peer = peerRef.current;

      switch (envelope.type) {
        case "joined": {
          if (!peer) return;
          const payload = envelope.payload as { peers?: unknown[] } | undefined;
          const already = payload?.peers?.length ?? 0;
          if (shouldOffer(already)) {
            offererRef.current = true;
            await makeOffer(peer);
          } else {
            setState("waiting");
          }
          break;
        }

        case "peer-joined":
          // The other side arrived after us, so they will offer. Nothing to
          // do but wait for it — and tell them where our toggles stand.
          setState("waiting");
          emitState();
          break;

        case "offer": {
          if (!peer) return;
          await peer.setRemoteDescription(
            new RTCSessionDescription(envelope.payload as RTCSessionDescriptionInit),
          );
          await drainCandidates(peer);
          const answer = await peer.createAnswer();
          await peer.setLocalDescription(answer);
          send({ type: "answer", payload: answer });
          break;
        }

        case "answer":
          if (!peer) return;
          await peer.setRemoteDescription(
            new RTCSessionDescription(envelope.payload as RTCSessionDescriptionInit),
          );
          await drainCandidates(peer);
          break;

        case "candidate": {
          if (!peer) return;
          const candidate = envelope.payload as RTCIceCandidateInit;
          if (peer.remoteDescription) {
            await peer.addIceCandidate(candidate);
          } else {
            pendingCandidates.current.push(candidate);
          }
          break;
        }

        case "chat": {
          const payload = envelope.payload as ChatPayload;
          if (typeof payload?.text !== "string") break;
          setMessages((current) => [
            ...current,
            {
              id: `${Date.now()}-${current.length}`,
              from: "peer",
              text: payload.text.slice(0, CHAT_MAX_LEN),
              at: Date.now(),
            },
          ]);
          setUnreadCount((count) => count + 1);
          break;
        }

        case "state":
          setPeerState({ ...initialPeerState, ...(envelope.payload as Partial<PeerState>) });
          break;

        case "reaction": {
          const payload = envelope.payload as ReactionPayload;
          if (typeof payload?.emoji === "string" && payload.emoji) {
            setActiveReaction({ emoji: payload.emoji, from: "peer", at: Date.now() });
          }
          break;
        }

        case "caption": {
          const payload = envelope.payload as CaptionPayload;
          if (typeof payload?.text === "string") {
            setPeerCaption({ text: payload.text, final: payload.final === true });
          }
          break;
        }

        case "peer-left":
          remoteRef.current = null;
          setRemoteStream(null);
          setPeerState(initialPeerState);
          setPeerCaption(null);
          setQuality(null);
          setState("waiting");
          break;

        case "error":
          setError(envelope.reason ?? "The session reported a problem.");
          break;

        default:
          break;
      }
    },
    [drainCandidates, emitState, makeOffer, send],
  );

  const refreshDevices = useCallback(async () => {
    const devices =
      (await navigator.mediaDevices?.enumerateDevices?.().catch(() => [])) ?? [];
    setMics(devices.filter((device) => device.kind === "audioinput"));
    setCameras(devices.filter((device) => device.kind === "videoinput"));
  }, []);

  const join = useCallback(
    (options?: JoinOptions) => {
      if (!session) return;
      const withVideo = options?.video !== false;
      intentionalLeaveRef.current = false;
      setError(null);
      setState("authorizing");

      void (async () => {
        let granted: SessionAccess;
        try {
          granted = await videoApi.join(session, refreshCallbacks, bookingId);
        } catch (failure) {
          setError(
            failure instanceof ApiError
              ? explainJoinFailure(failure.code, failure.message)
              : "We couldn't reach the session. Check your connection and try again.",
          );
          setState("failed");
          return;
        }
        accessRef.current = granted;
        setAccess(granted);

        // The camera is asked for only after entry is granted.
        setState("requesting-media");
        let stream: MediaStream;
        try {
          stream = await navigator.mediaDevices.getUserMedia({
            video: withVideo,
            audio: { ...AUDIO_CONSTRAINTS },
          });
        } catch {
          setError(
            withVideo
              ? "We couldn't reach your camera or microphone. Check your browser's permissions and try again."
              : "We couldn't reach your microphone. Check your browser's permissions and try again.",
          );
          setState("failed");
          return;
        }
        localRef.current = stream;
        setLocalStream(stream);
        micOnRef.current = true;
        cameraOnRef.current = withVideo;
        setMicOn(true);
        setCameraOn(withVideo);
        void refreshDevices();

        setState("connecting");
        connect(granted, stream);
      })();
    },
    // connect is a plain function declaration — stable across renders.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [session, refreshCallbacks, bookingId, refreshDevices],
  );

  const leave = useCallback(() => {
    teardown();
    setState("ended");
  }, [teardown]);

  const toggleMic = useCallback(() => {
    const track = localRef.current?.getAudioTracks()[0];
    if (!track) return;
    track.enabled = !track.enabled;
    micOnRef.current = track.enabled;
    setMicOn(track.enabled);
    emitState();
  }, [emitState]);

  const toggleCamera = useCallback(() => {
    const track = localRef.current?.getVideoTracks()[0];
    if (!track) return;
    track.enabled = !track.enabled;
    cameraOnRef.current = track.enabled;
    setCameraOn(track.enabled);
    emitState();
  }, [emitState]);

  const toggleScreenShare = useCallback(() => {
    if (screenTrackRef.current) {
      // Stop: put the parked camera track back on the wire.
      const cameraTrack = parkedCameraTrackRef.current;
      const sender = peerRef.current
        ?.getSenders?.()
        .find((candidate) => candidate.track?.kind === "video");
      if (cameraTrack && sender) void sender.replaceTrack(cameraTrack);
      screenTrackRef.current.onended = null;
      screenTrackRef.current.stop();
      screenTrackRef.current = null;
      parkedCameraTrackRef.current = null;
      setSharingScreen(false);
      return;
    }

    void (async () => {
      let display: MediaStream;
      try {
        display = await navigator.mediaDevices.getDisplayMedia({ video: true });
      } catch {
        return; // dismissed the picker — nothing to explain
      }
      const screenTrack = display.getVideoTracks()[0];
      if (!screenTrack) return;
      const sender = peerRef.current
        ?.getSenders?.()
        .find((candidate) => candidate.track?.kind === "video");
      if (!sender) {
        screenTrack.stop();
        return;
      }
      // The camera track is parked, not stopped: sharing ends by putting it
      // back, and the self-view keeps showing the camera throughout.
      parkedCameraTrackRef.current = localRef.current?.getVideoTracks()[0] ?? null;
      await sender.replaceTrack(screenTrack);
      // The browser's own "stop sharing" control ends the track.
      screenTrack.onended = () => {
        const cameraTrack = parkedCameraTrackRef.current;
        if (cameraTrack) void sender.replaceTrack(cameraTrack);
        parkedCameraTrackRef.current = null;
        screenTrackRef.current = null;
        setSharingScreen(false);
      };
      screenTrackRef.current = screenTrack;
      setSharingScreen(true);
    })();
  }, []);

  const sendChat = useCallback(
    (text: string) => {
      const trimmed = text.trim().slice(0, CHAT_MAX_LEN);
      if (!trimmed) return;
      setMessages((current) => [
        ...current,
        { id: `${Date.now()}-${current.length}`, from: "me", text: trimmed, at: Date.now() },
      ]);
      send({ type: "chat", payload: { text: trimmed } satisfies ChatPayload });
    },
    [send],
  );

  const markChatRead = useCallback(() => setUnreadCount(0), []);

  const toggleHand = useCallback(() => {
    handRaisedRef.current = !handRaisedRef.current;
    setHandRaised(handRaisedRef.current);
    emitState();
  }, [emitState]);

  const sendReaction = useCallback(
    (emoji: string) => {
      if (!emoji) return;
      setActiveReaction({ emoji, from: "me", at: Date.now() });
      send({ type: "reaction", payload: { emoji } satisfies ReactionPayload });
    },
    [send],
  );

  /** Swaps one input device mid-call: replaceTrack renegotiates in-band,
   * so there is no permission re-prompt and no peer-visible glitch. */
  const swapTrack = useCallback(
    async (kind: "audio" | "video", deviceId: string) => {
      const stream = await navigator.mediaDevices.getUserMedia(
        kind === "audio"
          ? { audio: { deviceId: { exact: deviceId }, ...AUDIO_CONSTRAINTS } }
          : { video: { deviceId: { exact: deviceId } } },
      );
      const newTrack = kind === "audio" ? stream.getAudioTracks()[0] : stream.getVideoTracks()[0];
      if (!newTrack) return;
      newTrack.enabled = kind === "audio" ? micOnRef.current : cameraOnRef.current;

      // While screen sharing the video sender carries the screen; the new
      // camera track becomes the one sharing reverts to.
      const sharing = kind === "video" && screenTrackRef.current !== null;
      const sender = peerRef.current
        ?.getSenders?.()
        .find((candidate) => candidate.track?.kind === kind);
      if (sender && !sharing) await sender.replaceTrack(newTrack);

      const current = localRef.current;
      const oldTrack = current?.getTracks().find((track) => track.kind === kind);
      oldTrack?.stop();
      const remaining = (current?.getTracks() ?? []).filter((track) => track.kind !== kind);
      const next = new MediaStream([...remaining, newTrack]);
      localRef.current = next;
      setLocalStream(next);
      if (sharing) parkedCameraTrackRef.current = newTrack;
      exposeDebug();
    },
    [exposeDebug],
  );

  const selectMic = useCallback(
    (deviceId: string) => {
      setSelectedMicId(deviceId);
      void swapTrack("audio", deviceId).catch(() => {
        setError("We couldn't switch to that microphone.");
      });
    },
    [swapTrack],
  );

  const selectCamera = useCallback(
    (deviceId: string) => {
      setSelectedCameraId(deviceId);
      void swapTrack("video", deviceId).catch(() => {
        setError("We couldn't switch to that camera.");
      });
    },
    [swapTrack],
  );

  const recordingSupported = typeof MediaRecorder !== "undefined";

  const toggleRecording = useCallback(() => {
    if (recordingRef.current) {
      stopRecordingInternals();
      recordingRef.current = false;
      setRecording(false);
      setRecordingSeconds(0);
      emitState();
      return;
    }
    if (!recordingSupported) {
      setError("Recording isn't supported in this browser.");
      return;
    }

    try {
      // Both voices on one track: local mic and remote audio are mixed
      // through an AudioContext; the picture is the remote tile.
      const context = new AudioContext();
      const destination = context.createMediaStreamDestination();
      for (const source of [localRef.current, remoteRef.current]) {
        if (source && source.getAudioTracks().length > 0) {
          context.createMediaStreamSource(source).connect(destination);
        }
      }
      const combined = new MediaStream([
        ...(remoteRef.current?.getVideoTracks() ?? []),
        ...destination.stream.getAudioTracks(),
      ]);
      const mimeType = ["video/webm;codecs=vp9,opus", "video/webm"].find((candidate) =>
        MediaRecorder.isTypeSupported(candidate),
      );
      const recorder = new MediaRecorder(combined, mimeType ? { mimeType } : undefined);
      recordChunksRef.current = [];
      recorder.ondataavailable = (event) => {
        if (event.data.size > 0) recordChunksRef.current.push(event.data);
      };
      recorder.onstop = () => {
        const chunks = recordChunksRef.current;
        recordChunksRef.current = [];
        if (chunks.length === 0) return;
        const blob = new Blob(chunks, { type: recorder.mimeType || "video/webm" });
        const url = URL.createObjectURL(blob);
        const anchor = document.createElement("a");
        anchor.href = url;
        anchor.download = `terios-session-${bookingId}-${new Date()
          .toISOString()
          .replace(/[:.]/g, "-")}.webm`;
        anchor.click();
        URL.revokeObjectURL(url);
      };
      recorder.start(1000);
      recorderRef.current = recorder;
      recordContextRef.current = context;
      recordingRef.current = true;
      setRecording(true);
      setRecordingSeconds(0);
      recordTimerRef.current = setInterval(
        () => setRecordingSeconds((seconds) => seconds + 1),
        1000,
      );
      emitState();
    } catch {
      setError("Recording couldn't start in this browser.");
    }
  }, [bookingId, emitState, recordingSupported, stopRecordingInternals]);

  const captionsSupported = hasSpeechRecognition();

  const toggleCaptions = useCallback(() => {
    if (captionsEnabledRef.current) {
      stopCaptionsInternals();
      setCaptionsEnabled(false);
      return;
    }
    if (!captionsSupported) return;

    // The Web Speech API hears the microphone, not the peer — so each side
    // transcribes its own voice and relays the text, which is also how the
    // captions arrive already labelled by speaker.
    const w = window as unknown as Record<string, unknown>;
    const Ctor = (w.SpeechRecognition ?? w.webkitSpeechRecognition) as
      | (new () => SpeechRecognitionLike)
      | undefined;
    if (!Ctor) return;
    const recognition = new Ctor();
    recognition.continuous = true;
    recognition.interimResults = true;
    recognition.lang = "en-US";
    recognition.onresult = (event) => {
      for (let index = event.resultIndex; index < event.results.length; index += 1) {
        const result = event.results[index];
        const text = result?.[0]?.transcript.trim();
        if (text) {
          send({
            type: "caption",
            payload: { text, final: result.isFinal } satisfies CaptionPayload,
          });
        }
      }
    };
    recognition.onerror = () => {
      // Recognizer failures never touch the call; captions are a courtesy.
    };
    recognition.onend = () => {
      // Recognition stops on its own after pauses; restart while enabled.
      if (captionsEnabledRef.current && !intentionalLeaveRef.current) {
        try {
          recognition.start();
        } catch {
          // Already started — nothing to do.
        }
      }
    };
    try {
      recognition.start();
      recognitionRef.current = recognition;
      captionsEnabledRef.current = true;
      setCaptionsEnabled(true);
    } catch {
      // A recognizer that refuses to start disables the feature quietly.
    }
  }, [captionsSupported, send, stopCaptionsInternals]);

  return {
    state,
    error,
    localStream,
    remoteStream,
    access,
    reconnecting,
    micOn,
    cameraOn,
    toggleMic,
    toggleCamera,
    join,
    leave,
    sharingScreen,
    toggleScreenShare,
    messages,
    unreadCount,
    sendChat,
    markChatRead,
    peerState,
    handRaised,
    toggleHand,
    activeReaction,
    sendReaction,
    mics,
    cameras,
    selectedMicId,
    selectedCameraId,
    selectMic,
    selectCamera,
    quality,
    recordingSupported,
    recording,
    recordingSeconds,
    toggleRecording,
    captionsSupported,
    captionsEnabled,
    toggleCaptions,
    peerCaption,
  };
}

/** The slice of the Web Speech API this hook uses, typed locally so the
 * lib dom types carry no vendor prefix. */
interface SpeechRecognitionLike {
  continuous: boolean;
  interimResults: boolean;
  lang: string;
  onresult: ((event: SpeechRecognitionResultEventLike) => void) | null;
  onerror: (() => void) | null;
  onend: (() => void) | null;
  start: () => void;
  stop: () => void;
}

interface SpeechRecognitionResultEventLike {
  resultIndex: number;
  results: ArrayLike<{ isFinal: boolean; 0?: { transcript: string } }>;
}
