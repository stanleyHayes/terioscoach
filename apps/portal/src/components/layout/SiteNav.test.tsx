import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { SiteNav } from "./SiteNav";

vi.mock("next/navigation", () => ({
  usePathname: () => "/",
}));

describe("SiteNav", () => {
  it("renders the wordmark and all nav links", () => {
    render(<SiteNav />);
    expect(screen.getAllByRole("link", { name: "Terios Wellness" }).length).toBeGreaterThanOrEqual(1);
    for (const label of [
      "Home",
      "About",
      "Services",
      "Blog",
      "FAQ",
      "Contact",
    ]) {
      expect(screen.getByRole("link", { name: label })).toBeTruthy();
    }
    expect(screen.getByRole("link", { name: "Book now" })).toBeTruthy();
    expect(screen.queryByRole("link", { name: "Work with me" })).toBeNull();
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

  it("settles full-width at the top and contracts after scrolling like Kedland", async () => {
    Object.defineProperty(window, "scrollY", { configurable: true, value: 0, writable: true });
    render(<SiteNav />);

    const header = screen.getByRole("banner");
    const nav = screen.getByRole("navigation", { name: "Main" });
    expect(header.getAttribute("data-header-state")).toBe("settled");
    expect(header.className).toContain("px-0");
    expect(nav.className).toContain("max-w-[100vw]");

    window.scrollY = 120;
    fireEvent.scroll(window);
    await waitFor(() => expect(header.getAttribute("data-header-state")).toBe("floating"));
    expect(header.className).toContain("px-3");
    expect(nav.className).toContain("max-w-[1240px]");

    window.scrollY = 0;
    fireEvent.scroll(window);
    await waitFor(() => expect(header.getAttribute("data-header-state")).toBe("settled"));
  });
});
