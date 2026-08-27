import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import type { ClientRecord, SessionNote } from "@/lib/clients";
import ClientRecordPage from "./page";

const get = vi.hoisted(() => vi.fn());
const updateProfile = vi.hoisted(() => vi.fn());
const noteGet = vi.hoisted(() => vi.fn());
const noteSave = vi.hoisted(() => vi.fn());
const noteShare = vi.hoisted(() => vi.fn());

vi.mock("next/navigation", () => ({ useParams: () => ({ id: "client-1" }) }));

vi.mock("@/lib/clients", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/clients")>();
  return {
    ...original,
    clientsApi: { list: vi.fn(), get, updateProfile },
    notesApi: { get: noteGet, save: noteSave, share: noteShare },
  };
});

vi.mock("@/lib/auth", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/auth")>();
  // One frozen value, not a fresh literal per render. The real provider
  // memoizes; a mock that doesn't would re-run every useResource effect on
  // every render and hide exactly the kind of refetch loop worth catching.
  const value = {
    status: "authenticated",
    user: { id: "prac-1", email: "t@example.com", role: "practitioner", name: "Terios" },
    session: { accessToken: "a1", refreshToken: "r1" },
    refreshCallbacks: { onTokensRefreshed: vi.fn() },
    logout: vi.fn(),
  };
  return { ...original, useAuth: () => value };
});

function record(overrides: Partial<ClientRecord> = {}): ClientRecord {
  return {
    id: "client-1",
    name: "Ama Serwaa",
    email: "ama@example.com",
    phone: "+233200000000",
    tags: ["returning", "evenings"],
    practiceNotes: "Prefers a quiet room.",
    recentBookings: [
      {
        id: "bk-past",
        serviceId: "svc-1",
        startAt: "2026-07-01T09:00:00.000Z",
        endAt: "2026-07-01T10:00:00.000Z",
        status: "completed",
        paymentStatus: "paid",
      },
      {
        id: "bk-next",
        serviceId: "svc-1",
        startAt: "2099-01-01T09:00:00.000Z",
        endAt: "2099-01-01T10:00:00.000Z",
        status: "confirmed",
      },
    ],
    payments: { totalPaidKobo: 45000, totalRefundedKobo: 5000, paymentCount: 2, currency: "GHS" },
    documentCount: 1,
    formSubmissionCount: 3,
    ...overrides,
  };
}

function note(overrides: Partial<SessionNote> = {}): SessionNote {
  return {
    id: "note-1",
    bookingId: "bk-past",
    clientId: "client-1",
    practitionerId: "prac-1",
    privateNotes: "Reported shoulder tension.",
    sharedFeedback: "",
    sharedResources: [],
    createdAt: "2026-07-01T11:00:00.000Z",
    updatedAt: "2026-07-01T11:00:00.000Z",
    ...overrides,
  };
}

describe("ClientRecordPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    get.mockResolvedValue(record());
    noteGet.mockResolvedValue(note());
    updateProfile.mockImplementation((_s, _c, _id, patch) => Promise.resolve({ id: "client-1", ...patch }));
  });

  it("shows a skeleton before the record arrives, not an empty file", async () => {
    let release: (value: ClientRecord) => void = () => {};
    get.mockReturnValue(new Promise<ClientRecord>((resolve) => { release = resolve; }));
    render(<ClientRecordPage />);

    // "No sessions on file yet" while the request is still open would tell
    // the practitioner something false about a real client.
    expect(screen.getByRole("status")).toBeTruthy();
    expect(screen.queryByText(/no sessions on file yet/i)).toBeNull();

    release(record());
    expect(await screen.findByText("Ama Serwaa")).toBeTruthy();
  });

  it("loads the client file and its rollups", async () => {
    render(<ClientRecordPage />);

    expect(await screen.findByRole("heading", { name: "Ama Serwaa" })).toBeTruthy();
    expect(get).toHaveBeenCalledWith(expect.anything(), expect.anything(), "client-1");
    expect(screen.getByText("ama@example.com")).toBeTruthy();
    // Money is minor units on the wire and must never reach the screen raw.
    expect(screen.getByText(/450/)).toBeTruthy();
    expect(screen.getByText("1 · 3")).toBeTruthy();
  });

  it("lists upcoming sessions before past ones", async () => {
    render(<ClientRecordPage />);
    await screen.findByRole("heading", { name: "Ama Serwaa" });

    const toggles = screen.getAllByRole("button", { name: /notes/i });
    // The next visit is what the practitioner is preparing for; it belongs
    // at the top even though it is the newer record.
    expect(toggles[0].textContent).toMatch(/confirmed/i);
    expect(toggles[1].textContent).toMatch(/completed/i);
    expect(toggles[1].textContent).toMatch(/paid/i);
  });

  it("opens a session's notes and closes them again", async () => {
    render(<ClientRecordPage />);
    await screen.findByRole("heading", { name: "Ama Serwaa" });
    const past = screen.getAllByRole("button", { name: /notes/i })[1];

    fireEvent.click(past);
    await waitFor(() => expect(noteGet).toHaveBeenCalledWith(expect.anything(), expect.anything(), "bk-past"));
    await waitFor(() => expect(past.getAttribute("aria-expanded")).toBe("true"));

    fireEvent.click(past);
    expect(past.getAttribute("aria-expanded")).toBe("false");
  });

  it("treats a session with no note yet as empty, not broken", async () => {
    noteGet.mockRejectedValue(new ApiError(404, "note_not_found", "no note"));
    render(<ClientRecordPage />);
    await screen.findByRole("heading", { name: "Ama Serwaa" });

    fireEvent.click(screen.getAllByRole("button", { name: /notes/i })[1]);

    // Most sessions have no note until the practitioner writes one. An
    // error banner there would be wrong on the common path.
    await waitFor(() => expect(noteGet).toHaveBeenCalled());
    const panel = document.getElementById("notes-bk-past")!;
    await waitFor(() => expect(panel.textContent).not.toMatch(/those notes didn't load/i));
  });

  it("reports a genuine failure to load notes", async () => {
    noteGet.mockRejectedValue(new ApiError(500, "server_error", "boom"));
    render(<ClientRecordPage />);
    await screen.findByRole("heading", { name: "Ama Serwaa" });

    const past = screen.getAllByRole("button", { name: /notes/i })[1];
    fireEvent.click(past);

    // Scoped to the panel that was opened: the message renders inside every
    // row's panel, and the others are `hidden`, so an unscoped query counts
    // the ones nobody can see.
    const panel = document.getElementById(String(past.getAttribute("aria-controls")))!;
    await waitFor(() => expect(panel.textContent).toMatch(/those notes didn't load/i));
    expect(panel.hasAttribute("hidden")).toBe(false);
  });

  it("keeps the save button off until something is actually edited", async () => {
    render(<ClientRecordPage />);
    await screen.findByRole("heading", { name: "Ama Serwaa" });
    const save = screen.getByRole("button", { name: /save detail/i });

    expect(save.hasAttribute("disabled")).toBe(true);

    fireEvent.change(screen.getByLabelText(/phone/i), { target: { value: "+233555000111" } });
    expect(save.hasAttribute("disabled")).toBe(false);
  });

  it("saves practice-side detail and splits tags on commas", async () => {
    render(<ClientRecordPage />);
    await screen.findByRole("heading", { name: "Ama Serwaa" });

    fireEvent.change(screen.getByLabelText(/tags/i), { target: { value: " vip , , mornings " } });
    fireEvent.click(screen.getByRole("button", { name: /save detail/i }));

    await waitFor(() =>
      expect(updateProfile).toHaveBeenCalledWith(
        expect.anything(),
        expect.anything(),
        "client-1",
        expect.objectContaining({
          // Trimmed, and the empty entry between the commas dropped — a
          // blank tag is a filter nobody can ever match.
          tags: ["vip", "mornings"],
          phone: "+233200000000",
          practiceNotes: "Prefers a quiet room.",
        }),
      ),
    );
  });

  it("shows the saved values afterwards, without a second fetch", async () => {
    render(<ClientRecordPage />);
    await screen.findByRole("heading", { name: "Ama Serwaa" });
    get.mockClear();

    fireEvent.change(screen.getByLabelText(/private summary/i), {
      target: { value: "Now prefers afternoons." },
    });
    fireEvent.click(screen.getByRole("button", { name: /save detail/i }));

    await waitFor(() =>
      expect(screen.getByRole("button", { name: /save detail/i }).hasAttribute("disabled")).toBe(true),
    );
    expect((screen.getByLabelText(/private summary/i) as HTMLTextAreaElement).value).toBe(
      "Now prefers afternoons.",
    );
    expect(get).not.toHaveBeenCalled();
  });

  it("keeps the practitioner's edit on screen when the save fails", async () => {
    updateProfile.mockRejectedValue(new ApiError(500, "server_error", "could not save"));
    render(<ClientRecordPage />);
    await screen.findByRole("heading", { name: "Ama Serwaa" });

    fireEvent.change(screen.getByLabelText(/private summary/i), { target: { value: "Do not lose me." } });
    fireEvent.click(screen.getByRole("button", { name: /save detail/i }));

    expect(await screen.findByRole("alert")).toBeTruthy();
    // Discarding typed notes on a failed save is how a practitioner loses
    // what they just wrote about a client.
    expect((screen.getByLabelText(/private summary/i) as HTMLTextAreaElement).value).toBe(
      "Do not lose me.",
    );
  });

  it("offers a retry when the record itself will not load", async () => {
    get.mockRejectedValue(new ApiError(500, "server_error", "the practice record is unavailable"));
    render(<ClientRecordPage />);

    expect(await screen.findByRole("alert")).toBeTruthy();
    get.mockResolvedValue(record());
    fireEvent.click(screen.getByRole("button", { name: /try again/i }));

    expect(await screen.findByRole("heading", { name: "Ama Serwaa" })).toBeTruthy();
  });

  it("says so plainly when a client has no sessions yet", async () => {
    get.mockResolvedValue(record({ recentBookings: [], documentCount: 0, formSubmissionCount: 1 }));
    render(<ClientRecordPage />);

    expect(await screen.findByText(/no sessions on file yet/i)).toBeTruthy();
    // Singular and plural both read correctly — "1 documents" is the kind
    // of thing a practitioner reads every day.
    expect(screen.getByText(/0 documents/)).toBeTruthy();
    expect(screen.getByText(/1 form on file/)).toBeTruthy();
  });

  it("labels a single document and form in the singular", async () => {
    get.mockResolvedValue(record({ documentCount: 1, formSubmissionCount: 1 }));
    render(<ClientRecordPage />);
    await screen.findByRole("heading", { name: "Ama Serwaa" });

    expect(screen.getByText(/1 document$/)).toBeTruthy();
    expect(screen.getByText(/1 form on file/)).toBeTruthy();
  });

  it("says the practice detail is private, because it is", async () => {
    render(<ClientRecordPage />);
    await screen.findByRole("heading", { name: "Ama Serwaa" });

    // The private/shared split is the single rule a practitioner most needs
    // to trust; the screen has to state which side it is on.
    expect(screen.getByText(/the client never sees any of this/i)).toBeTruthy();
  });
});
