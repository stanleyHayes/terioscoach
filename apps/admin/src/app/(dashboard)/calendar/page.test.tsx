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

vi.mock("@/lib/clients", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/clients")>();
  return {
    ...original,
    clientsApi: {
      ...original.clientsApi,
      list: vi.fn().mockResolvedValue([]),
    },
  };
});

vi.mock("@/lib/services", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/services")>();
  return {
    ...original,
    servicesApi: {
      ...original.servicesApi,
      listAll: vi.fn().mockResolvedValue([]),
    },
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
      name: "9:00 AM to 10:00 AM, client-1, svc-1, Confirmed",
    });
    expect(block).toBeTruthy();
    expect(listBookingsMock).toHaveBeenCalledWith(session, refreshCallbacks, {
      from: currentWeekRange().from,
      to: currentWeekRange().to,
    });
    expect(screen.getByText("Times in GMT")).toBeTruthy();
  });

  it("lists future consultations beyond the current week in date order and restores the calendar", async () => {
    listBookingsMock.mockResolvedValue([
      booking({ id: "later", clientId: "Later client", startAt: "2099-10-20T09:00:00Z", endAt: "2099-10-20T10:00:00Z" }),
      booking({ id: "nearer", clientId: "Nearer client", startAt: "2099-10-15T09:00:00Z", endAt: "2099-10-15T10:00:00Z" }),
    ]);
    render(<CalendarPage />);
    await screen.findByRole("grid");
    const before = Date.now();
    fireEvent.click(screen.getByRole("button", { name: "Upcoming list" }));
    const list = await screen.findByRole("region", { name: "Upcoming consultations" });
    const params = listBookingsMock.mock.lastCall?.[2];
    expect(params.status).toBe("confirmed");
    expect(params.to).toBeUndefined();
    expect(Date.parse(params.from)).toBeGreaterThanOrEqual(before);
    expect(within(list).getAllByRole("listitem")[0].textContent).toContain("Nearer client");
    expect(within(list).getAllByRole("listitem")[1].textContent).toContain("Later client");
    expect(list.textContent).toContain("Africa/Accra");
    fireEvent.click(screen.getByRole("button", { name: "Weekly calendar" }));
    await screen.findByRole("grid");
    expect(listBookingsMock).toHaveBeenLastCalledWith(session, refreshCallbacks, currentWeekRange());
  });

  it("opens existing booking controls from the list and removes cancelled consultations", async () => {
    const upcoming = booking({ startAt: "2099-10-15T09:00:00Z", endAt: "2099-10-15T10:00:00Z" });
    listBookingsMock.mockResolvedValue([upcoming]);
    cancelBookingMock.mockResolvedValue({ ...upcoming, status: "cancelled" });
    render(<CalendarPage />);
    await screen.findByRole("grid");
    fireEvent.click(screen.getByRole("button", { name: "Upcoming list" }));
    const list = await screen.findByRole("region", { name: "Upcoming consultations" });
    fireEvent.click(within(list).getByRole("button", { name: /client-1/ }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByRole("link", { name: "Start session" }).getAttribute("href")).toContain("/sessions/bk-1/room");
    fireEvent.click(within(dialog).getByRole("button", { name: "Cancel booking" }));
    await waitFor(() => expect(cancelBookingMock).toHaveBeenCalledWith(session, refreshCallbacks, "bk-1"));
    expect(await screen.findByText("No upcoming consultations")).toBeTruthy();
  });

  it("shows list loading and retry states before an empty result", async () => {
    listBookingsMock.mockResolvedValue([]).mockResolvedValueOnce([]).mockReturnValueOnce(new Promise(() => {}));
    render(<CalendarPage />);
    await screen.findByRole("grid");
    fireEvent.click(screen.getByRole("button", { name: "Upcoming list" }));
    expect(screen.getByText("Loading upcoming consultations…")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Weekly calendar" }));
    listBookingsMock.mockRejectedValueOnce(new ApiError(0, "network_error", "List unavailable"));
    await screen.findByRole("grid");
    fireEvent.click(screen.getByRole("button", { name: "Upcoming list" }));
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("List unavailable");
    listBookingsMock.mockResolvedValueOnce([]);
    fireEvent.click(within(alert).getByRole("button", { name: "Try again" }));
    expect(await screen.findByText("No upcoming consultations")).toBeTruthy();
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
        name: "9:00 AM to 10:00 AM, client-1, svc-1, Confirmed",
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
        name: "9:00 AM to 10:00 AM, client-1, svc-1, Confirmed",
      }),
    );
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Complete" }));

    await waitFor(() =>
      expect(completeBookingMock).toHaveBeenCalledWith(session, refreshCallbacks, "bk-1"),
    );
    expect(
      await screen.findByRole("button", {
        name: "9:00 AM to 10:00 AM, client-1, svc-1, Completed",
      }),
    ).toBeTruthy();
  });
});
