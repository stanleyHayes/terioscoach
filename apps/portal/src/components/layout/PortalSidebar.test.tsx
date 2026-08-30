import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { PortalSidebar } from "./PortalSidebar";

let pathname = "/portal";
vi.mock("next/navigation", () => ({ usePathname: () => pathname }));

describe("PortalSidebar", () => {
  beforeEach(() => { pathname = "/portal"; localStorage.clear(); });

  it("groups the complete client journey and marks the active route", () => {
    render(<PortalSidebar userName="Ama Serwaa" userEmail="ama@example.com" />);
    expect(screen.getByRole("complementary", { name: "Client portal navigation" })).toBeTruthy();
    for (const group of ["Your care", "Care records", "Account"]) expect(screen.getByRole("button", { name: new RegExp(group, "i") })).toBeTruthy();
    expect(screen.getByRole("link", { name: "Overview" }).getAttribute("aria-current")).toBe("page");
    expect(screen.getByText("ama@example.com")).toBeTruthy();
  });

  it("collapses groups and remembers the choice", () => {
    render(<PortalSidebar userName="Ama" />);
    const care = screen.getByRole("button", { name: /your care/i });
    expect(care.getAttribute("aria-expanded")).toBe("true");
    fireEvent.click(care);
    expect(care.getAttribute("aria-expanded")).toBe("false");
    expect(localStorage.getItem("terios-portal-nav-groups")).not.toContain("care");
  });

  it("uses the drawer overlay and closes after navigation on mobile", () => {
    const close = vi.fn();
    render(<PortalSidebar userName="Ama" mobileOpen onMobileClose={close} />);
    fireEvent.click(screen.getByRole("button", { name: "Close navigation" }));
    fireEvent.click(screen.getByRole("link", { name: "Overview" }));
    expect(close).toHaveBeenCalledTimes(2);
  });

  it("asks the shell to expand when a compact group is selected", () => {
    const expand = vi.fn();
    render(<PortalSidebar userName="Ama" collapsed onRequestExpand={expand} />);
    fireEvent.click(screen.getByTitle("Your care — expand sidebar"));
    expect(expand).toHaveBeenCalled();
  });
});
