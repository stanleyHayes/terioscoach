import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import {
  addDaysCivil,
  dateKey,
  mondayOfWeek,
  todayCivil,
  wallClockToUtcIso,
  PRACTICE_TIMEZONE,
  type Booking,
} from "@/lib/schedule";
import CalendarPage from "./page";

const logoutMock = vi.fn();
const listBookingsMock = vi.fn();
const completeBookingMock = vi.fn();
const cancelBookingMock = vi.fn();
const noShowBookingMock = vi.fn();
const rescheduleBookingMock = vi.fn();

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
      getRules: vi.fn(),
      putRules: vi.fn(),
      addTimeOff: vi.fn(),
      listBookings: (...args: unknown[]) => listBookingsMock(...args),
      rescheduleBooking: (...args: unknown[]) => rescheduleBookingMock(...args),
      cancelBooking: (...args: unknown[]) => cancelBookingMock(...args),
      completeBooking: (...args: unknown[]) => completeBookingMock(...args),
      noShowBooking: (...args: unknown[]) => noShowBookingMock(...args),
    },
  };
});

function booking(overrides: Partial<Booking> = {}): Booking {
  // Placed inside the current week so it always renders on the grid.
  const monday = mondayOfWeek(todayCivil(PRACTICE_TIMEZONE));
  const tuesday = addDaysCivil(monday, 1);
  const key = dateKey(tuesday);
  return {
    id: "bk-1",
    clientId: "client-1",
    practitionerId: "prac-1",
    serviceId: "svc-1",
    startAt: `${key}T09:00:00.000Z`,
    endAt: `${key}T10:00:00.000Z`,
    status: "confirmed",
    createdAt: "2026-08-01T10:00:00.000Z",
    updatedAt: "2026-08-01T10:00:00.000Z",
    ...overrides,
  };
}

/** Expected from/to bounds for the currently visible week. */
function currentWeekRange() {
  const monday = mondayOfWeek(todayCivil(PRACTICE_TIMEZONE));
  return {
    from: wallClockToUtcIso(dateKey(monday), "00:00", PRACTICE_TIMEZONE)!,
    to: wallClockToUtcIso(dateKey(addDaysCivil(monday, 7)), "00:00", PRACTICE_TIMEZONE)!,
  };
}

afterEach(() => {
  logoutMock.mockReset();
  listBookingsMock.mockReset();
  completeBookingMock.mockReset();
  cancelBookingMock.mockReset();
  noShowBookingMock.mockReset();
  rescheduleBookingMock.mockReset();
});

describe("CalendarPage", () => {
  it("fetches the visible week's bookings and renders them on the grid", async () => {
    listBookingsMock.mockResolvedValue([booking()]);
    render(<CalendarPage />);

    const block = await screen.findByRole("button", {
      name: "9:00 AM to 10:00 AM, client client-1, Confirmed",
    });
    expect(block).toBeTruthy();
    expect(listBookingsMock).toHaveBeenCalledWith(session, refreshCallbacks, {
      from: currentWeekRange().from,
      to: currentWeekRange().to,
    });
    expect(screen.getByText("Times in GMT")).toBeTruthy();
  });

  it("shows a skeleton while loading", async () => {
    listBookingsMock.mockReturnValue(new Promise(() => {}));
    render(<CalendarPage />);

    expect(await screen.findByRole("status")).toBeTruthy();
    expect(screen.getByText("Loading your calendar…")).toBeTruthy();
  });

  it("status filter chips refetch with the API status param", async () => {
    listBookingsMock.mockResolvedValue([]);
    render(<CalendarPage />);
    await screen.findByRole("grid");

    fireEvent.click(screen.getByRole("button", { name: "No-show" }));

    await waitFor(() =>
      expect(listBookingsMock).toHaveBeenLastCalledWith(session, refreshCallbacks, {
        from: currentWeekRange().from,
        to: currentWeekRange().to,
        status: "no_show",
      }),
    );
    expect(screen.getByRole("button", { name: "No-show" }).getAttribute("aria-pressed")).toBe(
      "true",
    );
  });

  it("week navigation refetches with the new range", async () => {
    listBookingsMock.mockResolvedValue([]);
    render(<CalendarPage />);
    await screen.findByRole("grid");

    fireEvent.click(screen.getByRole("button", { name: "Next week" }));

    const nextMonday = addDaysCivil(mondayOfWeek(todayCivil(PRACTICE_TIMEZONE)), 7);
    await waitFor(() =>
      expect(listBookingsMock).toHaveBeenLastCalledWith(session, refreshCallbacks, {
        from: wallClockToUtcIso(dateKey(nextMonday), "00:00", PRACTICE_TIMEZONE)!,
        to: wallClockToUtcIso(dateKey(addDaysCivil(nextMonday, 7)), "00:00", PRACTICE_TIMEZONE)!,
      }),
    );
  });

  it("shows an error banner with retry on failure", async () => {
    listBookingsMock
      .mockRejectedValueOnce(new ApiError(0, "network_error", "Can't reach the server."))
      .mockResolvedValueOnce([booking()]);
    render(<CalendarPage />);

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("Can't reach the server.");

    fireEvent.click(within(alert).getByRole("button", { name: "Try again" }));
    expect(
      await screen.findByRole("button", {
        name: "9:00 AM to 10:00 AM, client client-1, Confirmed",
      }),
    ).toBeTruthy();
  });

  it("completing a booking from the detail modal calls the API and updates the block", async () => {
    const done = booking({ status: "completed", completedAt: "2026-08-11T10:00:00.000Z" });
    listBookingsMock.mockResolvedValue([booking()]);
    completeBookingMock.mockResolvedValue(done);
    render(<CalendarPage />);

    fireEvent.click(
      await screen.findByRole("button", {
        name: "9:00 AM to 10:00 AM, client client-1, Confirmed",
      }),
    );
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Complete" }));

    await waitFor(() =>
      expect(completeBookingMock).toHaveBeenCalledWith(session, refreshCallbacks, "bk-1"),
    );
    expect(
      await screen.findByRole("button", {
        name: "9:00 AM to 10:00 AM, client client-1, Completed",
      }),
    ).toBeTruthy();
  });
});
