import { describe, expect, it } from "vitest";
import { API_BASE_URL } from "./api";
import {
  CHAT_MAX_LEN,
  explainJoinFailure,
  hasSpeechRecognition,
  reconnectDelays,
  scoreConnection,
  shouldOffer,
  signalUrl,
  timeUntil,
} from "./video";

describe("signalUrl", () => {
  it("switches the API origin to a websocket scheme and carries the ticket", () => {
    const url = signalUrl("booking-1", "tick et/+");

    expect(url.startsWith(API_BASE_URL.replace(/^http/, "ws"))).toBe(true);
    expect(url).toContain("/v1/sessions/booking-1/signal");
    // A ticket is base64url but the encoding is explicit rather than lucky.
    expect(url).toContain(`ticket=${encodeURIComponent("tick et/+")}`);
  });
});

describe("shouldOffer", () => {
  it("makes the second arrival offer, so both peers never offer at once", () => {
    // Glare — both sides offering simultaneously — is recoverable in
    // WebRTC but not worth the machinery. Arriving second is a fact each
    // peer already knows.
    expect(shouldOffer(0)).toBe(false);
    expect(shouldOffer(1)).toBe(true);
  });
});

describe("explainJoinFailure", () => {
  it("turns each refusal into something the client can act on", () => {
    expect(explainJoinFailure("room_not_open", "")).toMatch(/ten minutes before/i);
    expect(explainJoinFailure("room_closed", "")).toMatch(/has closed/i);
    expect(explainJoinFailure("invalid_status", "")).toMatch(/cancelled|completed/i);
    expect(explainJoinFailure("booking_not_found", "")).toMatch(/couldn't find/i);
    expect(explainJoinFailure("ticket_invalid", "")).toMatch(/expired/i);
  });

  it("falls back to the server's own message for anything unrecognised", () => {
    expect(explainJoinFailure("something_new", "Server said this.")).toBe("Server said this.");
  });
});

describe("timeUntil", () => {
  const now = new Date("2026-08-20T09:00:00Z");

  it("counts down in the largest sensible unit", () => {
    expect(timeUntil("2026-08-20T09:30:00Z", now)).toBe("in 30 minutes");
    // Exactly an hour reads as an hour, not as sixty minutes.
    expect(timeUntil("2026-08-20T10:00:00Z", now)).toBe("in 1 hour");
    expect(timeUntil("2026-08-20T13:00:00Z", now)).toBe("in 4 hours");
    expect(timeUntil("2026-08-23T09:00:00Z", now)).toBe("in 3 days");
  });

  it("uses the singular where it belongs", () => {
    expect(timeUntil("2026-08-20T09:01:00Z", now)).toBe("in 1 minute");
    expect(timeUntil("2026-08-20T11:00:00Z", now)).toBe("in 2 hours");
  });

  it("says 'now' once the moment has arrived or passed", () => {
    expect(timeUntil("2026-08-20T09:00:00Z", now)).toBe("now");
    expect(timeUntil("2026-08-20T08:00:00Z", now)).toBe("now");
  });
});

describe("scoreConnection", () => {
  it("scores a conversational call good, a degraded one fair, a broken one poor", () => {
    expect(scoreConnection(80, 0.005)).toBe("good");
    expect(scoreConnection(200, 0)).toBe("fair");
    expect(scoreConnection(80, 0.04)).toBe("fair");
    expect(scoreConnection(500, 0)).toBe("poor");
    expect(scoreConnection(80, 0.12)).toBe("poor");
  });
});

describe("hasSpeechRecognition", () => {
  it("is false where the Web Speech API is absent (jsdom, Firefox)", () => {
    expect(hasSpeechRecognition()).toBe(false);
  });

  it("is true when either the standard or prefixed recognizer exists", () => {
    const w = window as unknown as Record<string, unknown>;
    w.SpeechRecognition = class {};
    expect(hasSpeechRecognition()).toBe(true);
    delete w.SpeechRecognition;
    w.webkitSpeechRecognition = class {};
    expect(hasSpeechRecognition()).toBe(true);
    delete w.webkitSpeechRecognition;
  });
});

describe("reconnect policy", () => {
  it("gives up after three backoff attempts and falls back to manual rejoin", () => {
    expect(reconnectDelays).toHaveLength(3);
    for (const delay of reconnectDelays) expect(delay).toBeGreaterThan(0);
  });

  it("caps chat lines", () => {
    expect(CHAT_MAX_LEN).toBe(500);
  });
});
