import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import type { AvailabilityRule, TimeOff } from "@/lib/schedule";
import AvailabilityPage from "./page";

const logoutMock = vi.fn();
const getRulesMock = vi.fn();
const putRulesMock = vi.fn();
const addTimeOffMock = vi.fn();

const session = { accessToken: "access", refreshToken: "refresh" };
const refreshCallbacks = { onTokensRefreshed: vi.fn() };

vi.mock("@/lib/auth", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/auth")>();
  return {
    ...original,
    useAuth: () => ({
      status: "authenticated",
      user: { id: "u1", email: "akosua@terios.com", role: "practitioner", name: "Akosua" },
      accessToken: session.accessToken,
      session,
      refreshCallbacks,
      login: vi.fn(),
      logout: logoutMock,
    }),
  };
});

vi.mock("@/lib/schedule", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/schedule")>();
  return {
    ...original,
    scheduleApi: {
      getRules: (...args: unknown[]) => getRulesMock(...args),
      putRules: (...args: unknown[]) => putRulesMock(...args),
      addTimeOff: (...args: unknown[]) => addTimeOffMock(...args),
      listBookings: vi.fn(),
      rescheduleBooking: vi.fn(),
      cancelBooking: vi.fn(),
      completeBooking: vi.fn(),
      noShowBooking: vi.fn(),
    },
  };
});

/** Monday 09:00–17:00 with a 15-minute buffer; every other day closed. */
const RULES: AvailabilityRule[] = [
  { weekday: 1, windows: [{ startMin: 540, endMin: 1020 }], bufferMinutes: 15 },
];

function mondayRegion() {
  return screen.getByRole("region", { name: "Monday" });
}

function saveButton() {
  return screen.getByRole("button", { name: "Save changes" });
}

afterEach(() => {
  logoutMock.mockReset();
  getRulesMock.mockReset();
  putRulesMock.mockReset();
  addTimeOffMock.mockReset();
});

describe("AvailabilityPage", () => {
  it("loads rules into the weekly form (switch on, windows and buffer pre-filled)", async () => {
    getRulesMock.mockResolvedValue(RULES);
    render(<AvailabilityPage />);

    const monday = await screen.findByRole("region", { name: "Monday" });
    expect(
      (within(monday).getByRole("switch", { name: "Monday open" }).getAttribute("aria-checked")),
    ).toBe("true");
    expect((within(monday).getByLabelText("Start") as HTMLInputElement).value).toBe("09:00");
    expect((within(monday).getByLabelText("End") as HTMLInputElement).value).toBe("17:00");
    expect(
      (within(monday).getByLabelText("Buffer (minutes)") as HTMLInputElement).value,
    ).toBe("15");

    // Tuesday has no rule → closed.
    const tuesday = screen.getByRole("region", { name: "Tuesday" });
    expect(
      within(tuesday).getByRole("switch", { name: "Tuesday open" }).getAttribute("aria-checked"),
    ).toBe("false");
    expect(within(tuesday).getByText("Closed")).toBeTruthy();
    expect(getRulesMock).toHaveBeenCalledWith(session, refreshCallbacks);
  });

  it("keeps Save changes disabled until the form is dirty, and disables it again after save", async () => {
    getRulesMock.mockResolvedValue(RULES);
    putRulesMock.mockResolvedValue([
      { weekday: 1, windows: [{ startMin: 540, endMin: 750 }], bufferMinutes: 15 },
    ]);
    render(<AvailabilityPage />);
    await screen.findByRole("region", { name: "Monday" });

    expect((saveButton() as HTMLButtonElement).disabled).toBe(true);

    fireEvent.change(within(mondayRegion()).getByLabelText("End"), {
      target: { value: "12:30" },
    });
    expect((saveButton() as HTMLButtonElement).disabled).toBe(false);

    fireEvent.click(saveButton());
    await waitFor(() => expect(putRulesMock).toHaveBeenCalled());

    // PUT payload: only the open day, minutes-since-midnight windows.
    expect(putRulesMock).toHaveBeenCalledWith(session, refreshCallbacks, [
      { weekday: 1, windows: [{ startMin: 540, endMin: 750 }], bufferMinutes: 15 },
    ]);
    expect(await screen.findByRole("status")).toBeTruthy();
    expect(screen.getByText("Availability saved.")).toBeTruthy();
    expect((saveButton() as HTMLButtonElement).disabled).toBe(true);
  });

  it("adds and removes windows on a day", async () => {
    getRulesMock.mockResolvedValue(RULES);
    render(<AvailabilityPage />);
    await screen.findByRole("region", { name: "Monday" });

    fireEvent.click(within(mondayRegion()).getByRole("button", { name: "Add window" }));
    expect(within(mondayRegion()).getByLabelText("Window 2 start")).toBeTruthy();
    expect(within(mondayRegion()).getByLabelText("Window 2 end")).toBeTruthy();

    fireEvent.click(
      within(mondayRegion()).getByRole("button", { name: "Remove window 2 on Monday" }),
    );
    expect(within(mondayRegion()).queryByLabelText("Window 2 start")).toBeNull();
  });

  it("blocks overnight windows client-side before submit", async () => {
    getRulesMock.mockResolvedValue(RULES);
    render(<AvailabilityPage />);
    await screen.findByRole("region", { name: "Monday" });

    fireEvent.change(within(mondayRegion()).getByLabelText("Start"), {
      target: { value: "18:00" },
    });
    fireEvent.change(within(mondayRegion()).getByLabelText("End"), {
      target: { value: "09:00" },
    });
    fireEvent.click(saveButton());

    expect(
      within(mondayRegion()).getByText(
        "End must be after start — overnight windows aren't allowed.",
      ),
    ).toBeTruthy();
    expect(putRulesMock).not.toHaveBeenCalled();
  });

  it("blocks overlapping windows client-side before submit", async () => {
    getRulesMock.mockResolvedValue(RULES);
    render(<AvailabilityPage />);
    await screen.findByRole("region", { name: "Monday" });

    fireEvent.click(within(mondayRegion()).getByRole("button", { name: "Add window" }));
    fireEvent.change(within(mondayRegion()).getByLabelText("Window 2 start"), {
      target: { value: "16:00" },
    });
    fireEvent.change(within(mondayRegion()).getByLabelText("Window 2 end"), {
      target: { value: "18:00" },
    });
    fireEvent.click(saveButton());

    expect(
      within(mondayRegion()).getByText("This window overlaps the one above."),
    ).toBeTruthy();
    expect(putRulesMock).not.toHaveBeenCalled();
  });

  it("validates the buffer range", async () => {
    getRulesMock.mockResolvedValue(RULES);
    render(<AvailabilityPage />);
    await screen.findByRole("region", { name: "Monday" });

    fireEvent.change(within(mondayRegion()).getByLabelText("Buffer (minutes)"), {
      target: { value: "150" },
    });
    fireEvent.click(saveButton());

    expect(
      within(mondayRegion()).getByText("Enter a buffer between 0 and 120 minutes."),
    ).toBeTruthy();
    expect(putRulesMock).not.toHaveBeenCalled();
  });

  it("shows an error banner with retry when rules fail to load", async () => {
    getRulesMock
      .mockRejectedValueOnce(new ApiError(0, "network_error", "Can't reach the server."))
      .mockResolvedValueOnce(RULES);
    render(<AvailabilityPage />);

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("Can't reach the server.");

    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByRole("region", { name: "Monday" })).toBeTruthy();
  });

  it("adds time off with an inclusive end date (endAt = next midnight) and lists it", async () => {
    getRulesMock.mockResolvedValue(RULES);
    const created: TimeOff = {
      id: "to-1",
      practitionerId: "prac-1",
      startAt: "2026-09-01T00:00:00.000Z",
      endAt: "2026-09-04T00:00:00.000Z",
      reason: "Family trip",
      createdAt: "2026-08-11T10:00:00.000Z",
    };
    addTimeOffMock.mockResolvedValue(created);
    render(<AvailabilityPage />);
    await screen.findByRole("region", { name: "Monday" });

    fireEvent.change(screen.getByLabelText(/^Start date/), {
      target: { value: "2026-09-01" },
    });
    fireEvent.change(screen.getByLabelText(/^End date/), {
      target: { value: "2026-09-03" },
    });
    fireEvent.change(screen.getByLabelText(/Reason/), {
      target: { value: "Family trip" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Add time off" }));

    await waitFor(() =>
      expect(addTimeOffMock).toHaveBeenCalledWith(session, refreshCallbacks, {
        startAt: "2026-09-01T00:00:00.000Z",
        endAt: "2026-09-04T00:00:00.000Z",
        reason: "Family trip",
      }),
    );
    expect(await screen.findByText("Family trip")).toBeTruthy();
    expect(screen.getByText("Tue, Sep 1, 2026 – Thu, Sep 3, 2026")).toBeTruthy();
    expect(screen.getByText("Time off saved.")).toBeTruthy();
  });

  it("validates time-off dates before submit", async () => {
    getRulesMock.mockResolvedValue(RULES);
    render(<AvailabilityPage />);
    await screen.findByRole("region", { name: "Monday" });

    fireEvent.change(screen.getByLabelText(/^Start date/), {
      target: { value: "2026-09-10" },
    });
    fireEvent.change(screen.getByLabelText(/^End date/), {
      target: { value: "2026-09-05" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Add time off" }));

    expect(
      screen.getByText("The end date must be on or after the start date."),
    ).toBeTruthy();
    expect(addTimeOffMock).not.toHaveBeenCalled();
  });
});
