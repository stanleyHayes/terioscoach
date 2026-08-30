import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { PortalTopbar } from "./PortalTopbar";

vi.mock("next/navigation", () => ({ usePathname: () => "/portal/sessions" }));

describe("PortalTopbar", () => {
  const props = { userName: "Ama Serwaa", userEmail: "ama@example.com", onSignOut: vi.fn() };

  it("shows the current location and two-sided actions", () => {
    render(<PortalTopbar {...props} />);
    expect(screen.getByText("Consultations")).toBeTruthy();
    expect(screen.getByRole("link", { name: "Book a session" }).getAttribute("href")).toBe("/portal/book");
    expect(screen.getByRole("link", { name: "Visit Terios website" }).getAttribute("href")).toBe("https://terioscoach.com");
  });

  it("opens a contrast-safe account menu and signs out", () => {
    const signOut = vi.fn();
    render(<PortalTopbar {...props} onSignOut={signOut} />);
    fireEvent.click(screen.getByRole("button", { name: /Ama Serwaa/ }));
    expect(screen.getByRole("menu", { name: "Account" })).toBeTruthy();
    expect(screen.getByText("ama@example.com")).toBeTruthy();
    fireEvent.click(screen.getByRole("menuitem", { name: "Sign out" }));
    expect(signOut).toHaveBeenCalled();
  });

  it("controls the desktop rail and mobile drawer", () => {
    const collapse = vi.fn(); const mobile = vi.fn();
    render(<PortalTopbar {...props} onToggleCollapse={collapse} onOpenMobileNav={mobile} />);
    fireEvent.click(screen.getByRole("button", { name: "Collapse sidebar" }));
    fireEvent.click(screen.getByRole("button", { name: "Open navigation" }));
    expect(collapse).toHaveBeenCalled(); expect(mobile).toHaveBeenCalled();
  });
});
