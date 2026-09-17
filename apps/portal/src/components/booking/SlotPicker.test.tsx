import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SlotPicker } from "./SlotPicker";

const getSlots = vi.hoisted(() => vi.fn());

vi.mock("@/lib/bookings", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/bookings")>();
  return { ...original, getSlots };
});

describe("SlotPicker timezone handling", () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.setSystemTime(new Date("2027-03-13T12:00:00.000Z"));
    getSlots.mockResolvedValue({
      serviceId: "svc-1",
      durationMinutes: 60,
      timezone: "America/New_York",
      slots: [],
    });
  });

  afterEach(() => {
    vi.useRealTimers();
    getSlots.mockReset();
  });

  it("builds consecutive client-visible day keys across DST changes", async () => {
    render(
      <SlotPicker
        serviceId="svc-1"
        selectedSlot={null}
        onSelect={vi.fn()}
        timeZone="America/New_York"
      />,
    );

    await waitFor(() =>
      expect(getSlots).toHaveBeenCalledWith({
        serviceId: "svc-1",
        from: "2027-03-13",
        to: "2027-03-13",
        tz: "America/New_York",
      }),
    );

    expect(screen.getByRole("button", { name: "Sat13" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Sun14" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Mon15" })).toBeTruthy();
  });

  it.each([
    ["America/New_York", "4:00 AM"],
    ["America/Los_Angeles", "1:00 AM"],
    ["Europe/London", "9:00 AM"],
    ["Africa/Lagos", "10:00 AM"],
  ])("renders returned UTC slots converted for %s", async (timeZone, expectedLabel) => {
    getSlots.mockResolvedValueOnce({
      serviceId: "svc-1",
      durationMinutes: 60,
      timezone: timeZone,
      slots: [
        {
          startAt: "2027-01-04T09:00:00.000Z",
          endAt: "2027-01-04T10:00:00.000Z",
        },
      ],
    });

    render(
      <SlotPicker
        serviceId="svc-1"
        selectedSlot={null}
        onSelect={vi.fn()}
        timeZone={timeZone}
      />,
    );

    expect(await screen.findByRole("option", { name: expectedLabel })).toBeTruthy();
    expect(getSlots).toHaveBeenCalledWith({
      serviceId: "svc-1",
      from: expect.any(String),
      to: expect.any(String),
      tz: timeZone,
    });
  });
});
