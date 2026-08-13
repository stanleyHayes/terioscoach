import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { SessionNote } from "@/lib/clients";
import { NotesComposer, isMissingNote, parseResources } from "./NotesComposer";

const save = vi.hoisted(() => vi.fn());
const share = vi.hoisted(() => vi.fn());

vi.mock("@/lib/clients", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/clients")>();
  return { ...original, notesApi: { save, share, get: vi.fn() } };
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

function note(overrides: Partial<SessionNote> = {}): SessionNote {
  return {
    id: "note-1",
    bookingId: "booking-1",
    clientId: "client-1",
    practitionerId: "prac-1",
    privateNotes: "Tension in left shoulder.",
    sharedFeedback: "Great progress.",
    sharedResources: ["https://example.com/stretch"],
    createdAt: "2026-08-10T09:00:00Z",
    updatedAt: "2026-08-10T09:00:00Z",
    ...overrides,
  };
}

describe("NotesComposer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    save.mockImplementation((_s, _c, bookingId, content) =>
      Promise.resolve(note({ bookingId, ...content })),
    );
    share.mockImplementation((_s, _c, bookingId) =>
      Promise.resolve(note({ bookingId, sharedAt: "2026-08-11T09:00:00Z" })),
    );
  });

  it("keeps the private and shared halves visibly apart", () => {
    render(<NotesComposer bookingId="booking-1" note={note()} onSaved={vi.fn()} />);

    // The boundary is the whole point of this screen, so it is stated in
    // the UI rather than left to the practitioner to remember.
    expect(screen.getByText(/never shared/i)).toBeTruthy();
    expect(
      screen.getByText(/the client cannot see this, before or after sharing/i),
    ).toBeTruthy();
  });

  it("says an unshared note is not visible to the client", () => {
    render(<NotesComposer bookingId="booking-1" note={note()} onSaved={vi.fn()} />);

    expect(screen.getByText(/not shared yet/i)).toBeTruthy();
    expect(screen.getByText(/invisible to the client until you share/i)).toBeTruthy();
  });

  it("saves the edited content", async () => {
    const onSaved = vi.fn();
    render(<NotesComposer bookingId="booking-1" note={note()} onSaved={onSaved} />);

    fireEvent.change(screen.getByRole("textbox", { name: /private notes/i }), {
      target: { value: "Revised private." },
    });
    fireEvent.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() => {
      expect(save).toHaveBeenCalledWith(
        expect.anything(),
        expect.anything(),
        "booking-1",
        expect.objectContaining({ privateNotes: "Revised private." }),
      );
    });
    expect(onSaved).toHaveBeenCalled();
  });

  it("will not save when nothing has changed", () => {
    render(<NotesComposer bookingId="booking-1" note={note()} onSaved={vi.fn()} />);

    expect(screen.getByRole("button", { name: /^save$/i })).toHaveProperty("disabled", true);
  });

  it("asks before sharing, because sharing cannot be undone", async () => {
    render(<NotesComposer bookingId="booking-1" note={note()} onSaved={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: /share with client/i }));

    expect(screen.getByText(/this cannot be undone/i)).toBeTruthy();
    expect(share).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: /not yet/i }));
    expect(screen.queryByText(/this cannot be undone/i)).toBeNull();
    expect(share).not.toHaveBeenCalled();
  });

  it("shares once confirmed", async () => {
    const onSaved = vi.fn();
    render(<NotesComposer bookingId="booking-1" note={note()} onSaved={onSaved} />);

    fireEvent.click(screen.getByRole("button", { name: /share with client/i }));
    fireEvent.click(screen.getByRole("button", { name: /yes, share it/i }));

    await waitFor(() => {
      expect(share).toHaveBeenCalledWith(expect.anything(), expect.anything(), "booking-1");
    });
    expect(onSaved).toHaveBeenCalled();
  });

  it("saves unsaved edits before sharing them", async () => {
    render(<NotesComposer bookingId="booking-1" note={note()} onSaved={vi.fn()} />);

    fireEvent.change(screen.getByRole("textbox", { name: /^feedback$/i }), {
      target: { value: "Updated feedback." },
    });
    fireEvent.click(screen.getByRole("button", { name: /share with client/i }));
    fireEvent.click(screen.getByRole("button", { name: /yes, share it/i }));

    // Sharing publishes what is stored; a practitioner who typed and then
    // pressed Share means both to happen.
    await waitFor(() => {
      expect(save).toHaveBeenCalledWith(
        expect.anything(),
        expect.anything(),
        "booking-1",
        expect.objectContaining({ sharedFeedback: "Updated feedback." }),
      );
    });
    await waitFor(() => {
      expect(share).toHaveBeenCalled();
    });
  });

  it("cannot share an empty note", () => {
    render(
      <NotesComposer
        bookingId="booking-1"
        note={note({ sharedFeedback: "" })}
        onSaved={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: /share with client/i })).toHaveProperty(
      "disabled",
      true,
    );
  });

  it("shows an already-shared note as shared, with no way to unshare", () => {
    render(
      <NotesComposer
        bookingId="booking-1"
        note={note({ sharedAt: "2026-08-11T09:00:00Z" })}
        onSaved={vi.fn()}
      />,
    );

    expect(screen.getByText("Shared")).toBeTruthy();
    expect(screen.getByText(/sharing cannot be undone/i)).toBeTruthy();
    expect(screen.queryByRole("button", { name: /share with client/i })).toBeNull();
    expect(screen.queryByRole("button", { name: /unshare/i })).toBeNull();
  });

  it("starts empty for a session with no note yet", () => {
    render(<NotesComposer bookingId="booking-2" note={null} onSaved={vi.fn()} />);

    expect(screen.getByRole("textbox", { name: /private notes/i })).toHaveProperty("value", "");
    expect(screen.getByRole("textbox", { name: /^feedback$/i })).toHaveProperty("value", "");
  });

  it("swaps content when a different session is opened", () => {
    const { rerender } = render(
      <NotesComposer bookingId="booking-1" note={note()} onSaved={vi.fn()} />,
    );
    expect(screen.getByRole("textbox", { name: /private notes/i })).toHaveProperty(
      "value",
      "Tension in left shoulder.",
    );

    rerender(
      <NotesComposer
        bookingId="booking-2"
        note={note({ bookingId: "booking-2", privateNotes: "Different session." })}
        onSaved={vi.fn()}
      />,
    );

    // Otherwise the previous client's notes would sit in the box.
    expect(screen.getByRole("textbox", { name: /private notes/i })).toHaveProperty("value", "Different session.");
  });

  it("surfaces a failed save", async () => {
    const { ApiError } = await import("@/lib/api");
    save.mockRejectedValueOnce(new ApiError(400, "validation_error", "Notes are too long."));
    render(<NotesComposer bookingId="booking-1" note={note()} onSaved={vi.fn()} />);

    fireEvent.change(screen.getByRole("textbox", { name: /private notes/i }), { target: { value: "x" } });
    fireEvent.click(screen.getByRole("button", { name: /^save$/i }));

    expect((await screen.findByRole("alert")).textContent).toContain("Notes are too long.");
  });
});

describe("parseResources", () => {
  it("takes one link per line and drops blanks", () => {
    expect(parseResources("  https://a.test \n\n https://b.test \n")).toEqual([
      "https://a.test",
      "https://b.test",
    ]);
  });

  it("returns nothing for an empty box", () => {
    expect(parseResources("   \n  ")).toEqual([]);
  });
});

describe("isMissingNote", () => {
  it("treats a 404 as 'nothing written yet' rather than a failure", async () => {
    const { ApiError } = await import("@/lib/api");
    expect(isMissingNote(new ApiError(404, "note_not_found", "not found"))).toBe(true);
    expect(isMissingNote(new ApiError(500, "internal_error", "boom"))).toBe(false);
    expect(isMissingNote(new Error("network"))).toBe(false);
  });
});
