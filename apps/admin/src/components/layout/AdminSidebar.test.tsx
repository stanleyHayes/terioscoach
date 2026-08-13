import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { AdminSidebar } from "./AdminSidebar";

vi.mock("next/navigation", () => ({
  usePathname: () => "/",
}));

describe("AdminSidebar", () => {
  it("renders the wordmark and all nav items", () => {
    render(<AdminSidebar userName="Akosua Mensah" />);

    expect(screen.getByText("Terios")).toBeTruthy();
    for (const label of [
      "Overview",
      "Calendar",
      "Availability",
      "Clients",
      "Services",
      "Payments",
      "Content",
      "Forms",
      "Enquiries",
      "Reviews",
      "Reports",
    ]) {
      expect(screen.getByRole("link", { name: label })).toBeTruthy();
    }
  });

  it("links Calendar and Availability to their routes", () => {
    render(<AdminSidebar userName="Akosua Mensah" />);

    expect(screen.getByRole("link", { name: "Calendar" }).getAttribute("href")).toBe(
      "/calendar",
    );
    expect(screen.getByRole("link", { name: "Availability" }).getAttribute("href")).toBe(
      "/availability",
    );
  });

  it("marks the current page with aria-current", () => {
    render(<AdminSidebar userName="Akosua Mensah" />);

    expect(
      screen.getByRole("link", { name: "Overview" }).getAttribute("aria-current"),
    ).toBe("page");
    expect(
      screen.getByRole("link", { name: "Calendar" }).hasAttribute("aria-current"),
    ).toBe(false);
  });

  it("renders the practitioner user card", () => {
    render(<AdminSidebar userName="Akosua Mensah" />);

    expect(screen.getByText("Akosua Mensah")).toBeTruthy();
    expect(screen.getByText("Practitioner")).toBeTruthy();
    expect(screen.getByText("AM")).toBeTruthy();
  });
});
