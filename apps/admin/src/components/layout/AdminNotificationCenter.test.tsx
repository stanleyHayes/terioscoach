import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AdminNotificationCenter } from "./AdminNotificationCenter";

const refresh = vi.fn();
const resource = vi.hoisted(() => vi.fn());
vi.mock("@/lib/use-resource", () => ({ useResource: resource }));

describe("AdminNotificationCenter", () => {
  beforeEach(() => {
    refresh.mockReset();
    resource.mockReturnValue({ data: [{ id: "enquiries", title: "2 new enquiries", description: "Waiting", href: "/enquiries", tone: "attention" }], error: null, refresh });
  });

  it("shows a count and opens the live practice pulse", () => {
    render(<AdminNotificationCenter />);
    fireEvent.click(screen.getByRole("button", { name: "Notifications, 1 item needs attention" }));
    expect(screen.getByRole("dialog", { name: "Practice notifications" })).toBeTruthy();
    expect(screen.getByRole("link", { name: /2 new enquiries/i }).getAttribute("href")).toBe("/enquiries");
  });

  it("refreshes from the popover", () => {
    render(<AdminNotificationCenter />);
    fireEvent.click(screen.getByRole("button", { name: /Notifications, 1/ }));
    fireEvent.click(screen.getByRole("button", { name: "Refresh notifications" }));
    expect(refresh).toHaveBeenCalledTimes(1);
  });
});
