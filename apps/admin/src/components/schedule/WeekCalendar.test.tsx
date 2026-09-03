import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi, type Mock } from "vitest";
import { ApiError } from "@/lib/api";
import type { Booking } from "@/lib/schedule";
import { WeekCalendar, type WeekCalendarProps } from "./WeekCalendar";

/**
 * All fixtures sit in the practice zone (Africa/Accra ≡ UTC), so positioning
 * math is deterministic regardless of the machine running the tests.
 * Week under test: Monday 2026-08-10 → Sunday 2026-08-16.
 */
const WEEK_START = { year: 2026, month: 8, day: 10 };
const TIME_ZONE = "Africa/Accra";

function booking(overrides: Partial<Booking> = {}): Booking {
  return {
    id: "bk-1",
    clientId: "client-1",
    practitionerId: "prac-1",
    serviceId: "svc-1",
    startAt: "2026-08-11T09:00:00.000Z",
    endAt: "2026-08-11T10:30:00.000Z",
    status: "confirmed",
    createdAt: "2026-08-01T10:00:00.000Z",
    updatedAt: "2026-08-01T10:00:00.000Z",
    ...overrides,
  };
}

interface Handlers {
  onPrevWeek: Mock<() => void>;
  onNextWeek: Mock<() => void>;
  onToday: Mock<() => void>;
  onAction: Mock<WeekCalendarProps["onAction"]>;
  onReschedule: Mock<WeekCalendarProps["onReschedule"]>;
}

function renderCalendar(bookings: Booking[] = [booking()], handlers: Partial<Handlers> = {}) {
  const props: Handlers = {
    onPrevWeek: vi.fn<() => void>(),
    onNextWeek: vi.fn<() => void>(),
    onToday: vi.fn<() => void>(),
    onAction: vi.fn<WeekCalendarProps["onAction"]>(),
    onReschedule: vi.fn<WeekCalendarProps["onReschedule"]>(),
    ...handlers,
  };
  render(
    <WeekCalendar
      weekStart={WEEK_START}
      timeZone={TIME_ZONE}
      bookings={bookings}
      {...props}
    />,
  );
  return props;
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("WeekCalendar", () => {
  it("renders the week range and the timezone label", () => {
    renderCalendar();

    expect(screen.getByText("Aug 10–16, 2026")).toBeTruthy();
    expect(screen.getByText("Times in GMT")).toBeTruthy();
    for (const day of ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"]) {
      expect(screen.getByText(day)).toBeTruthy();
    }
  });

  it("positions booking blocks from startAt/endAt with status styling", () => {
    renderCalendar([
      booking(),
      booking({
        id: "bk-2",
        clientId: "client-2",
        startAt: "2026-08-12T14:00:00.000Z",
        endAt: "2026-08-12T15:00:00.000Z",
        status: "no_show",
      }),
      booking({
        id: "bk-3",
        clientId: "client-3",
        startAt: "2026-08-10T06:00:00.000Z",
        endAt: "2026-08-10T06:15:00.000Z",
        status: "cancelled",
      }),
    ]);

    // Tuesday 09:00–10:30 → top (540-360)/60*48 = 144px, height 90/60*48 = 72px.
    const block = screen.getByRole("button", {
      name: "9:00 AM to 10:30 AM, client-1, svc-1, Confirmed",
    });
    expect(block.style.top).toBe("144px");
    expect(block.style.height).toBe("72px");
    expect(block.className).toContain("border-l-primary");

    const noShow = screen.getByRole("button", {
      name: "2:00 PM to 3:00 PM, client-2, svc-1, No-show",
    });
    expect(noShow.style.top).toBe("384px"); // (840-360)/60*48
    expect(noShow.style.height).toBe("48px");
    expect(noShow.className).toContain("border-l-warning");

    // 15-minute booking gets the 24px minimum visual height.
    const cancelled = screen.getByRole("button", {
      name: "6:00 AM to 6:15 AM, client-3, svc-1, Cancelled",
    });
    expect(cancelled.style.top).toBe("0px");
    expect(cancelled.style.height).toBe("24px");
    expect(cancelled.className).toContain("line-through");
  });

  it("clamps bookings that fall outside the 06:00–20:00 lanes", () => {
    renderCalendar([
      booking({
        startAt: "2026-08-11T04:00:00.000Z",
        endAt: "2026-08-11T07:00:00.000Z",
      }),
    ]);

    // Starts before the lanes: top clamps to 0, height counts from 06:00.
    const block = screen.getByRole("button", {
      name: "4:00 AM to 7:00 AM, client-1, svc-1, Confirmed",
    });
    expect(block.style.top).toBe("0px");
    expect(block.style.height).toBe("48px");
  });

  it("week nav buttons call the range-change handlers", () => {
    const handlers = renderCalendar();

    fireEvent.click(screen.getByRole("button", { name: "Previous week" }));
    expect(handlers.onPrevWeek).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole("button", { name: "Next week" }));
    expect(handlers.onNextWeek).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole("button", { name: "Today" }));
    expect(handlers.onToday).toHaveBeenCalledTimes(1);
  });

  it("keyboard: arrows move between day columns, Enter focuses the first booking", () => {
    renderCalendar();
    const grid = screen.getByRole("grid");
    const columns = within(grid).getAllByRole("gridcell");
    const monday = columns[0]!;
    const tuesday = columns[1]!;

    monday.focus();
    expect(document.activeElement).toBe(monday);
    fireEvent.keyDown(monday, { key: "ArrowRight" });
    expect(document.activeElement).toBe(tuesday);
    fireEvent.keyDown(tuesday, { key: "End" });
    expect(document.activeElement).toBe(columns[6]);
    fireEvent.keyDown(columns[6]!, { key: "Home" });
    expect(document.activeElement).toBe(monday);

    fireEvent.keyDown(tuesday, { key: "Enter" });
    expect(tuesday.contains(document.activeElement)).toBe(true);
    expect(document.activeElement?.getAttribute("aria-label")).toContain("Confirmed");
  });

  it("opens the detail modal with client, service, time and status", () => {
    renderCalendar();

    fireEvent.click(
      screen.getByRole("button", {
        name: "9:00 AM to 10:30 AM, client-1, svc-1, Confirmed",
      }),
    );

    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText("client-1")).toBeTruthy();
    expect(within(dialog).getByText("svc-1")).toBeTruthy();
    expect(within(dialog).getByText("Confirmed")).toBeTruthy();
    expect(dialog.textContent).toContain("Tue, Aug 11, 2026");
    expect(dialog.textContent).toContain("(GMT)");
  });

  it("Complete calls the action handler and updates the modal status", async () => {
    const completed = booking({ status: "completed", completedAt: "2026-08-11T10:30:00.000Z" });
    const handlers = renderCalendar([booking()], {
      onAction: vi.fn<WeekCalendarProps["onAction"]>().mockResolvedValue(completed),
    });

    fireEvent.click(
      screen.getByRole("button", {
        name: "9:00 AM to 10:30 AM, client-1, svc-1, Confirmed",
      }),
    );
    const dialog = screen.getByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Complete" }));

    await waitFor(() =>
      expect(handlers.onAction).toHaveBeenCalledWith(
        expect.objectContaining({ id: "bk-1" }),
        "complete",
      ),
    );
    // Badge flips to Completed (selector pins the badge span, not the dt row).
    expect(
      await within(dialog).findByText("Completed", { selector: "span" }),
    ).toBeTruthy();
    // Terminal state: the action row is gone.
    expect(within(dialog).queryByRole("button", { name: "Complete" })).toBeNull();
  });

  it("shows API errors from actions inline in the modal", async () => {
    renderCalendar([booking()], {
      onAction: vi
        .fn<WeekCalendarProps["onAction"]>()
        .mockRejectedValue(
          new ApiError(409, "invalid_status", "Only finished sessions can be completed."),
        ),
    });

    fireEvent.click(
      screen.getByRole("button", {
        name: "9:00 AM to 10:30 AM, client-1, svc-1, Confirmed",
      }),
    );
    const dialog = screen.getByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Complete" }));

    await waitFor(() =>
      expect(within(dialog).getByRole("alert").textContent).toContain(
        "Only finished sessions can be completed.",
      ),
    );
  });

  it("reschedule validates inputs, then calls the handler with a UTC instant", async () => {
    const moved = booking({ startAt: "2026-08-12T14:00:00.000Z", endAt: "2026-08-12T15:30:00.000Z" });
    const handlers = renderCalendar([booking()], {
      onReschedule: vi.fn<WeekCalendarProps["onReschedule"]>().mockResolvedValue(moved),
    });

    fireEvent.click(
      screen.getByRole("button", {
        name: "9:00 AM to 10:30 AM, client-1, svc-1, Confirmed",
      }),
    );
    const dialog = screen.getByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Reschedule" }));

    // Invalid first: nonsense date blocks submission.
    fireEvent.change(within(dialog).getByLabelText(/^Date/), {
      target: { value: "tomorrow" },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Save new time" }));
    expect(
      within(dialog).getByText("Enter a date as YYYY-MM-DD, e.g. 2026-08-12"),
    ).toBeTruthy();
    expect(handlers.onReschedule).not.toHaveBeenCalled();

    fireEvent.change(within(dialog).getByLabelText(/^Date/), {
      target: { value: "2026-08-12" },
    });
    fireEvent.change(within(dialog).getByLabelText(/^Time/), {
      target: { value: "14:00" },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Save new time" }));

    await waitFor(() =>
      expect(handlers.onReschedule).toHaveBeenCalledWith(
        expect.objectContaining({ id: "bk-1" }),
        "2026-08-12T14:00:00.000Z",
      ),
    );
    expect(await within(dialog).findByText("2:00 PM–3:30 PM (GMT)")).toBeTruthy();
  });
});
