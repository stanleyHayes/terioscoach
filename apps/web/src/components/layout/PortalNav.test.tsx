import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { PortalNav } from "./PortalNav";

vi.mock("next/navigation", () => ({
  usePathname: () => "/portal",
}));

const props = {
  userName: "Ama Serwaa",
  userEmail: "ama@example.com",
  onSignOut: vi.fn(),
};

describe("PortalNav", () => {
  it("renders the wordmark and all six sections", () => {
    render(<PortalNav {...props} />);

    expect(screen.getByRole("link", { name: "Terios Wellness" })).toBeTruthy();
    for (const label of ["Overview", "Sessions", "Forms", "Documents", "Payments", "Reviews"]) {
      expect(screen.getByRole("link", { name: label })).toBeTruthy();
    }
  });

  it("marks Overview active and points Sessions at the real page", () => {
    render(<PortalNav {...props} />);

    expect(screen.getByRole("link", { name: "Overview" }).getAttribute("aria-current")).toBe(
      "page",
    );
    expect(screen.getByRole("link", { name: "Sessions" }).getAttribute("href")).toBe(
      "/portal/sessions",
    );
    expect(
      screen.getByRole("link", { name: "Sessions" }).getAttribute("aria-current"),
    ).toBeNull();
  });

  it("renders the primary Book action into the booking flow", () => {
    render(<PortalNav {...props} />);

    expect(screen.getByRole("link", { name: "Book" }).getAttribute("href")).toBe("/portal/book");
  });

  it("offers an explicit route back to the public website", () => {
    render(<PortalNav userName="Ama Serwaa" onSignOut={vi.fn()} />);
    expect(screen.getByRole("link", { name: "Back to website" }).getAttribute("href")).toBe("/");
  });

  it("opens the user menu, shows the account details, and signs out", () => {
    const onSignOut = vi.fn();
    render(<PortalNav {...props} onSignOut={onSignOut} />);

    const trigger = screen.getByRole("button", { name: /Ama Serwaa/ });
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
    expect(screen.queryByRole("menu")).toBeNull();

    fireEvent.click(trigger);
    expect(trigger.getAttribute("aria-expanded")).toBe("true");
    expect(screen.getByRole("menu", { name: "Account" })).toBeTruthy();
    expect(screen.getByText("ama@example.com")).toBeTruthy();

    fireEvent.click(screen.getByRole("menuitem", { name: "Sign out" }));
    expect(onSignOut).toHaveBeenCalled();
    expect(screen.queryByRole("menu")).toBeNull();
  });

  it("closes the menu on Escape", () => {
    render(<PortalNav {...props} />);

    fireEvent.click(screen.getByRole("button", { name: /Ama Serwaa/ }));
    expect(screen.getByRole("menu")).toBeTruthy();

    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("menu")).toBeNull();
  });
});
