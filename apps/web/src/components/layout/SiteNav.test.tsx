import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { SiteNav } from "./SiteNav";

vi.mock("next/navigation", () => ({
  usePathname: () => "/",
}));

describe("SiteNav", () => {
  it("renders the wordmark and all nav links", () => {
    render(<SiteNav />);
    expect(screen.getAllByText("Terios Wellness").length).toBeGreaterThanOrEqual(1);
    for (const label of [
      "Home",
      "About",
      "Services",
      "Work With Me",
      "Blog",
      "FAQ",
      "Contact",
    ]) {
      expect(screen.getByRole("link", { name: label })).toBeTruthy();
    }
    expect(screen.getByRole("link", { name: "Book now" })).toBeTruthy();
  });

  it("marks the active route with aria-current", () => {
    render(<SiteNav />);
    expect(
      screen.getByRole("link", { name: "Home" }).getAttribute("aria-current"),
    ).toBe("page");
    expect(
      screen.getByRole("link", { name: "About" }).getAttribute("aria-current"),
    ).toBeNull();
  });

  it("opens the mobile menu on hamburger click and closes on Escape", () => {
    render(<SiteNav />);

    const toggle = screen.getByRole("button", { name: "Open menu" });
    toggle.focus();
    expect(toggle.getAttribute("aria-expanded")).toBe("false");
    expect(screen.queryByRole("dialog")).toBeNull();

    fireEvent.click(toggle);
    expect(toggle.getAttribute("aria-expanded")).toBe("true");
    const dialog = screen.getByRole("dialog", { name: "Site menu" });
    expect(dialog).toBeTruthy();
    // Focus is moved into the overlay.
    expect(dialog.contains(document.activeElement)).toBe(true);

    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(toggle.getAttribute("aria-expanded")).toBe("false");
    // Focus returns to the trigger.
    expect(document.activeElement).toBe(toggle);
  });

  it("closes the mobile menu via the close button", () => {
    render(<SiteNav />);
    fireEvent.click(screen.getByRole("button", { name: "Open menu" }));
    expect(screen.getByRole("dialog")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Close menu" }));
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});
