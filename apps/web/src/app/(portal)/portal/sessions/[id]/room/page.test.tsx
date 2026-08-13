import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ClientSessionRoomPage from "./page";

/**
 * The client's video room route (CX-05).
 *
 * This route did not exist. `portal/sessions/page.tsx` had been offering a
 * "Join" link to it since the sessions list was written, and the component
 * it should have rendered — `components/portal/VideoRoom` — was complete
 * and had 15 passing tests, imported by nothing. Every client who clicked
 * Join for their paid session got a 404.
 *
 * Nothing caught it because the two halves were tested separately: the
 * component's tests render it directly, and the sessions list's tests
 * assert the link's href without following it. The test below is the one
 * that would have failed — it renders what the route actually serves.
 */

const push = vi.hoisted(() => vi.fn());
const useVideoRoom = vi.hoisted(() => vi.fn());

vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "booking-42" }),
  useRouter: () => ({ push, replace: vi.fn() }),
  usePathname: () => "/portal/sessions/booking-42/room",
}));

vi.mock("@/lib/use-video-room", () => ({ useVideoRoom }));

/** A room in whatever state a test needs. */
function room(overrides: Record<string, unknown> = {}) {
  return {
    state: "idle",
    error: null,
    localStream: null,
    remoteStream: null,
    access: null,
    reconnecting: false,
    micOn: true,
    cameraOn: true,
    toggleMic: vi.fn(),
    toggleCamera: vi.fn(),
    join: vi.fn(),
    leave: vi.fn(),
    sharingScreen: false,
    toggleScreenShare: vi.fn(),
    messages: [],
    unreadCount: 0,
    sendChat: vi.fn(),
    markChatRead: vi.fn(),
    peerState: { micOn: true, cameraOn: true, handRaised: false, recording: false },
    handRaised: false,
    toggleHand: vi.fn(),
    activeReaction: null,
    sendReaction: vi.fn(),
    mics: [],
    cameras: [],
    selectedMicId: null,
    selectedCameraId: null,
    selectMic: vi.fn(),
    selectCamera: vi.fn(),
    quality: null,
    recordingSupported: false,
    recording: false,
    recordingSeconds: 0,
    toggleRecording: vi.fn(),
    captionsSupported: false,
    captionsEnabled: false,
    toggleCaptions: vi.fn(),
    peerCaption: null,
    ...overrides,
  };
}

describe("Client session room route", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useVideoRoom.mockReturnValue(room());
  });

  it("renders a room for the booking in the URL", () => {
    render(<ClientSessionRoomPage />);

    // The id must come from the route, not from anywhere else: a hardcoded
    // or stale id would put the client in someone else's session.
    expect(useVideoRoom).toHaveBeenCalledWith("booking-42");
    expect(screen.getByRole("heading", { name: /your session/i })).toBeTruthy();
  });

  it("offers a way back to the sessions list", () => {
    render(<ClientSessionRoomPage />);

    const back = screen.getByRole("link", { name: /back to your sessions/i });
    expect(back.getAttribute("href")).toBe("/portal/sessions");
  });

  it("names the practitioner as the person being waited for", () => {
    // "waiting" rather than "connecting": while connecting the tile says
    // "Connecting…" and names nobody.
    useVideoRoom.mockReturnValue(room({ state: "waiting" }));
    render(<ClientSessionRoomPage />);

    // The admin room passes "your client"; this one must not, or the
    // client is told they are waiting for themselves.
    expect(screen.getByText(/waiting for your practitioner to join/i)).toBeTruthy();
    expect(screen.getByLabelText(/your practitioner's camera/i)).toBeTruthy();
  });

  it("returns the client to their sessions when they leave", () => {
    const leave = vi.fn();
    useVideoRoom.mockReturnValue(room({ state: "connected", leave }));
    render(<ClientSessionRoomPage />);

    fireEvent.click(screen.getByRole("button", { name: /leave|end/i }));

    expect(leave).toHaveBeenCalled();
    // Leaving must not strand them on a dead room page.
    expect(push).toHaveBeenCalledWith("/portal/sessions");
  });

  it("shows the API's reason when the room refuses entry", () => {
    useVideoRoom.mockReturnValue(
      room({ state: "failed", error: "The room isn't open yet. You can join 10 minutes before." }),
    );
    render(<ClientSessionRoomPage />);

    // The window and the ownership check belong to the API; this page
    // reports its answer rather than guessing at one.
    expect(screen.getByText(/isn't open yet/i)).toBeTruthy();
  });
});
