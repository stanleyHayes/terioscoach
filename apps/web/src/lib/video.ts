/**
 * Video session client (CX-05/CX-06) — the API half.
 *
 *   POST /v1/sessions/{bookingId}/join            → ticket + ICE servers
 *   GET  /v1/sessions/{bookingId}/signal?ticket=  → the signaling socket
 *
 * Authorization happens on the POST, where a real status code can explain
 * why (too early, cancelled, not yours). The socket carries a single-use
 * ticket instead of a bearer token, because a browser cannot set headers on
 * a WebSocket handshake.
 */

import { API_BASE_URL, authedRequest, type RefreshCallbacks, type Session } from "./api";

export interface IceServer {
  urls: string[];
  username?: string;
  credential?: string;
}

export interface SessionAccess {
  bookingId: string;
  role: "client" | "practitioner";
  ticket: string;
  ticketExpiresIn: number;
  opensAt: string;
  closesAt: string;
  iceServers: IceServer[];
}

export const videoApi = {
  /** Throws ApiError: 403 room_not_open / room_closed, 409 invalid_status,
   * 404 booking_not_found — each of which the room UI explains in words. */
  join(
    session: Session,
    callbacks: RefreshCallbacks,
    bookingId: string,
  ): Promise<SessionAccess> {
    return authedRequest<SessionAccess>(
      `/v1/sessions/${bookingId}/join`,
      session,
      callbacks,
      { method: "POST" },
    );
  },
};

/** The signaling socket URL for a redeemed ticket. */
export function signalUrl(bookingId: string, ticket: string): string {
  const base = API_BASE_URL.replace(/^http/, "ws");
  return `${base}/v1/sessions/${bookingId}/signal?ticket=${encodeURIComponent(ticket)}`;
}

export type SignalType =
  | "offer"
  | "answer"
  | "candidate"
  | "chat"
  | "state"
  | "reaction"
  | "caption"
  | "joined"
  | "peer-joined"
  | "peer-left"
  | "error"
  | "ping"
  | "pong";

export interface SignalEnvelope {
  type: SignalType;
  from?: string;
  role?: "client" | "practitioner";
  payload?: unknown;
  reason?: string;
}

/** Payload of a "chat" envelope. */
export interface ChatPayload {
  text: string;
}

/** Payload of a "state" envelope — a participant's presence, relayed so the
 * other tile can show mute/hand/recording indicators. */
export interface PeerState {
  micOn: boolean;
  cameraOn: boolean;
  handRaised: boolean;
  recording: boolean;
}

export const initialPeerState: PeerState = {
  micOn: true,
  cameraOn: true,
  handRaised: false,
  recording: false,
};

/** Payload of a "reaction" envelope. */
export interface ReactionPayload {
  emoji: string;
}

/** Payload of a "caption" envelope — own-mic transcription, relayed. */
export interface CaptionPayload {
  text: string;
  final: boolean;
}

/** One line in the room's chat panel. */
export interface ChatMessage {
  id: string;
  from: "me" | "peer";
  text: string;
  at: number;
}

/** Chat lines are capped client-side; the socket's frame cap bounds the rest. */
export const CHAT_MAX_LEN = 500;

/** Backoff for automatic rejoins after a dropped socket: three attempts,
 * then the manual-rejoin UI takes over. */
export const reconnectDelays = [1000, 2000, 4000];

export type ConnectionQuality = "good" | "fair" | "poor";

/**
 * Scores the call's connection from the selected pair's round-trip time and
 * the inbound packet-loss ratio. Thresholds are WebRTC rules of thumb for
 * a call that still feels conversational.
 */
export function scoreConnection(rttMs: number, lossRatio: number): ConnectionQuality {
  if (rttMs > 400 || lossRatio > 0.08) return "poor";
  if (rttMs > 150 || lossRatio > 0.02) return "fair";
  return "good";
}

/** Whether this browser has the Web Speech API (Chrome/Edge, as
 * `SpeechRecognition` or the prefixed variant) — the captions gate. */
export function hasSpeechRecognition(): boolean {
  if (typeof window === "undefined") return false;
  const w = window as unknown as Record<string, unknown>;
  return "SpeechRecognition" in w || "webkitSpeechRecognition" in w;
}

/**
 * Who makes the offer.
 *
 * Both peers cannot offer at once — that is "glare", and WebRTC's recovery
 * from it is more machinery than this needs. The rule here is simple and
 * symmetric: whoever arrives second offers, because they are the one who
 * knows somebody is already there.
 */
export function shouldOffer(peersAlreadyPresent: number): boolean {
  return peersAlreadyPresent > 0;
}

/** Turns a join failure into something a client should read. */
export function explainJoinFailure(code: string, message: string): string {
  switch (code) {
    case "room_not_open":
      return "The room isn't open yet. You can join from ten minutes before your session starts.";
    case "room_closed":
      return "This session's room has closed. If you need more time, message your practitioner.";
    case "invalid_status":
      return "This session isn't active — it may have been cancelled or already completed.";
    case "booking_not_found":
      return "We couldn't find that session on your account.";
    case "ticket_invalid":
      return "That link has expired. Open the session again from your list.";
    default:
      return message;
  }
}

/** How long until the room opens, phrased for a countdown. */
export function timeUntil(opensAt: string, now = new Date()): string {
  const ms = new Date(opensAt).getTime() - now.getTime();
  if (ms <= 0) return "now";

  const minutes = Math.ceil(ms / 60000);
  if (minutes < 60) return `in ${minutes} ${minutes === 1 ? "minute" : "minutes"}`;

  const hours = Math.round(minutes / 60);
  if (hours < 24) return `in ${hours} ${hours === 1 ? "hour" : "hours"}`;

  const days = Math.round(hours / 24);
  return `in ${days} ${days === 1 ? "day" : "days"}`;
}
