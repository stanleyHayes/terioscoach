import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { PortalTopbar } from "./PortalTopbar";

vi.mock("next/navigation", () => ({ usePathname: () => "/portal/sessions" }));
vi.mock("./PortalNotificationCenter", () => ({ PortalNotificationCenter: () => <button>Notifications</button> }));

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

  it("opens contextual help for the current page", () => {
    render(<PortalTopbar {...props} />);
    fireEvent.click(screen.getByRole("button", { name: "Help with Consultations" }));
    expect(screen.getByRole("dialog", { name: "How to use Consultations" })).toBeTruthy();
    expect(screen.getByRole("link", { name: "Open full user guide" }).getAttribute("href")).toBe("/portal/guide");
  });

  it("links the user guide from the account menu", () => {
    render(<PortalTopbar {...props} />);
    fireEvent.click(screen.getByRole("button", { name: /Ama Serwaa/ }));
    expect(screen.getByRole("menuitem", { name: "User guide" }).getAttribute("href")).toBe("/portal/guide");
  });
});
