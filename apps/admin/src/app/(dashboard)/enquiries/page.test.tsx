import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Enquiry } from "@/lib/inbox";
import EnquiriesPage from "./page";

const list = vi.hoisted(() => vi.fn());
const setStatus = vi.hoisted(() => vi.fn());
const remove = vi.hoisted(() => vi.fn());

vi.mock("@/lib/inbox", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/inbox")>();
  return { ...original, enquiriesApi: { list, setStatus, remove } };
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

function enquiry(overrides: Partial<Enquiry> = {}): Enquiry {
  return {
    id: "enquiry-1",
    name: "Ama Serwaa",
    email: "ama@example.com",
    subject: "Booking question",
    message: "Do you offer prenatal massage?",
    status: "new",
    createdAt: "2026-08-10T09:00:00Z",
    updatedAt: "2026-08-10T09:00:00Z",
    ...overrides,
  };
}

describe("EnquiriesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    list.mockResolvedValue([enquiry()]);
    setStatus.mockImplementation((_s, _c, id, status) =>
      Promise.resolve(enquiry({ id, status })),
    );
    remove.mockResolvedValue(undefined);
  });

  it("shows the inbox with how many are waiting", async () => {
    render(<EnquiriesPage />);

    expect(await screen.findByText("Ama Serwaa")).toBeTruthy();
    expect(screen.getByText(/1 waiting for a reply/i)).toBeTruthy();
  });

  it("marks an enquiry read when it is opened — that is what read means", async () => {
    render(<EnquiriesPage />);
    const row = await screen.findByRole("button", { name: /ama serwaa/i });

    fireEvent.click(row);

    await waitFor(() => {
      expect(setStatus).toHaveBeenCalledWith(
        expect.anything(),
        expect.anything(),
        "enquiry-1",
        "read",
      );
    });
    expect(await screen.findByText(/everything here has been seen/i)).toBeTruthy();
  });

  it("does not re-mark an enquiry that has already been read", async () => {
    list.mockResolvedValue([enquiry({ status: "read" })]);
    render(<EnquiriesPage />);

    fireEvent.click(await screen.findByRole("button", { name: /ama serwaa/i }));

    await waitFor(() => {
      expect(screen.getByText(/do you offer prenatal massage/i)).toBeTruthy();
    });
    expect(setStatus).not.toHaveBeenCalled();
  });

  it("offers every other triage state, so a mistake can be undone", async () => {
    list.mockResolvedValue([enquiry({ status: "replied" })]);
    render(<EnquiriesPage />);

    fireEvent.click(await screen.findByRole("button", { name: /ama serwaa/i }));

    for (const label of [/mark new/i, /mark read/i, /mark archived/i]) {
      expect(screen.getByRole("button", { name: label })).toBeTruthy();
    }
    // Not its own state.
    expect(screen.queryByRole("button", { name: /mark replied/i })).toBeNull();
  });

  it("filters by status", async () => {
    list.mockResolvedValue([
      enquiry({ id: "e1", name: "New person", status: "new" }),
      enquiry({ id: "e2", name: "Archived person", status: "archived" }),
    ]);
    render(<EnquiriesPage />);
    await screen.findByText("New person");

    fireEvent.click(screen.getByRole("button", { name: "Archived" }));

    expect(screen.getByText("Archived person")).toBeTruthy();
    expect(screen.queryByText("New person")).toBeNull();
  });

  it("removes a deleted enquiry from the list", async () => {
    // Opened already-read, so no mark-read write is in flight: while one is,
    // the row's actions are deliberately disabled.
    list.mockResolvedValue([enquiry({ status: "read" })]);
    render(<EnquiriesPage />);
    fireEvent.click(await screen.findByRole("button", { name: /ama serwaa/i }));

    fireEvent.click(screen.getByRole("button", { name: /delete the enquiry from ama/i }));

    await waitFor(() => {
      expect(screen.queryByText("Ama Serwaa")).toBeNull();
    });
  });

  it("blocks further actions on a row while a write is in flight", async () => {
    // A second click on a row mid-write would race the first; the row locks
    // until the write settles.
    let resolve: (value: Enquiry) => void = () => {};
    setStatus.mockImplementationOnce(
      () => new Promise<Enquiry>((r) => {
        resolve = r;
      }),
    );
    render(<EnquiriesPage />);

    fireEvent.click(await screen.findByRole("button", { name: /ama serwaa/i }));

    const remove = await screen.findByRole("button", { name: /delete the enquiry from ama/i });
    expect(remove).toHaveProperty("disabled", true);

    resolve(enquiry({ status: "read" }));
    await waitFor(() => {
      expect(remove).toHaveProperty("disabled", false);
    });
  });

  it("explains a failed write without losing the list", async () => {
    const { ApiError } = await import("@/lib/api");
    setStatus.mockRejectedValueOnce(new ApiError(500, "internal_error", "Something went wrong."));
    render(<EnquiriesPage />);

    fireEvent.click(await screen.findByRole("button", { name: /ama serwaa/i }));

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("Something went wrong.");
    expect(screen.getByText("Ama Serwaa")).toBeTruthy();
  });

  it("invites nothing when the inbox is empty", async () => {
    list.mockResolvedValue([]);
    render(<EnquiriesPage />);

    expect(await screen.findByRole("heading", { name: /no enquiries yet/i })).toBeTruthy();
  });

  it("offers a retry when the inbox will not load", async () => {
    const { ApiError } = await import("@/lib/api");
    list.mockRejectedValue(new ApiError(0, "network_error", "Can't reach the server."));
    render(<EnquiriesPage />);

    const alert = await screen.findByRole("alert");
    expect(within(alert).getByRole("button", { name: /try again/i })).toBeTruthy();
  });
});
