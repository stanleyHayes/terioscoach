import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { PortalNotificationCenter } from "./PortalNotificationCenter";

const refresh = vi.fn();
const resource = vi.hoisted(() => vi.fn());
vi.mock("@/lib/use-portal-data", () => ({ usePortalData: resource }));

describe("PortalNotificationCenter", () => {
  beforeEach(() => {
    refresh.mockReset();
    resource.mockReturnValue({ data: [{ id: "form-one", title: "Wellness intake", description: "Ready", href: "/portal/forms/one", tone: "attention" }], error: null, refresh });
  });

  it("gives clients an actionable notification center", () => {
    render(<PortalNotificationCenter />);
    fireEvent.click(screen.getByRole("button", { name: "Notifications, 1 item needs attention" }));
    expect(screen.getByRole("dialog", { name: "Client notifications" })).toBeTruthy();
    expect(screen.getByRole("link", { name: /Wellness intake/i }).getAttribute("href")).toBe("/portal/forms/one");
  });

  it("explains the empty state", () => {
    resource.mockReturnValue({ data: [], error: null, refresh });
    render(<PortalNotificationCenter />);
    fireEvent.click(screen.getByRole("button", { name: "Notifications" }));
    expect(screen.getByText("You’re all caught up")).toBeTruthy();
  });
});
