import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { NAV_ITEMS } from "@/components/layout/AdminSidebar";
import { AdminTopbar } from "./AdminTopbar";

const pathname = vi.hoisted(() => vi.fn());

vi.mock("next/navigation", () => ({ usePathname: pathname }));

describe("AdminTopbar", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    pathname.mockReturnValue("/");
  });

  it("names the signed-in practitioner", () => {
    render(<AdminTopbar userName="Naa Adjeley" onSignOut={vi.fn()} />);

    // A shared machine in a practice: whose records are on screen is not a
    // detail worth hiding.
    expect(screen.getByText("Naa Adjeley")).toBeTruthy();
  });

  it("signs out on request", () => {
    const onSignOut = vi.fn();
    render(<AdminTopbar userName="Naa Adjeley" onSignOut={onSignOut} />);

    fireEvent.click(screen.getByRole("button", { name: /sign out/i }));
    expect(onSignOut).toHaveBeenCalledTimes(1);
  });

  it("shows the sign-out in progress rather than letting it be pressed twice", () => {
    const onSignOut = vi.fn();
    render(<AdminTopbar userName="Naa Adjeley" onSignOut={onSignOut} signingOut />);

    const button = screen.getByRole("button", { name: /sign out/i });
    expect(button.hasAttribute("disabled")).toBe(true);
    fireEvent.click(button);
    expect(onSignOut).not.toHaveBeenCalled();
  });

  it("carries the whole practice navigation for small screens", () => {
    render(<AdminTopbar userName="Naa Adjeley" onSignOut={vi.fn()} />);

    const nav = screen.getByRole("navigation", { name: /practice sections/i });
    const links = [...nav.querySelectorAll("a")].map((a) => a.textContent);
    // The sidebar is hidden below lg, so anything missing here is a screen
    // a practitioner cannot reach on a phone.
    expect(links).toEqual(NAV_ITEMS.map((item) => item.label));
  });

  it("marks the current section as the current page", () => {
    pathname.mockReturnValue("/clients");
    render(<AdminTopbar userName="Naa Adjeley" onSignOut={vi.fn()} />);

    const nav = screen.getByRole("navigation", { name: /practice sections/i });
    const current = [...nav.querySelectorAll("[aria-current='page']")].map((el) => el.textContent);
    expect(current).toEqual(["Clients"]);
  });

  it("keeps a nested route inside its section", () => {
    pathname.mockReturnValue("/clients/client-1");
    render(<AdminTopbar userName="Naa Adjeley" onSignOut={vi.fn()} />);

    const nav = screen.getByRole("navigation", { name: /practice sections/i });
    // A client file is still the Clients section; losing the highlight
    // there is how a practitioner loses their place.
    expect([...nav.querySelectorAll("[aria-current='page']")].map((el) => el.textContent)).toEqual([
      "Clients",
    ]);
  });

  it("does not treat Overview as the parent of everything", () => {
    pathname.mockReturnValue("/reports");
    render(<AdminTopbar userName="Naa Adjeley" onSignOut={vi.fn()} />);

    const nav = screen.getByRole("navigation", { name: /practice sections/i });
    const current = [...nav.querySelectorAll("[aria-current='page']")].map((el) => el.textContent);
    // Overview's href is "/", which is a prefix of every other route. A
    // naive startsWith would light up two sections at once.
    expect(current).toEqual(["Reports"]);
  });

  it("highlights Overview only on the overview itself", () => {
    pathname.mockReturnValue("/");
    render(<AdminTopbar userName="Naa Adjeley" onSignOut={vi.fn()} />);

    const nav = screen.getByRole("navigation", { name: /practice sections/i });
    expect([...nav.querySelectorAll("[aria-current='page']")].map((el) => el.textContent)).toEqual([
      "Overview",
    ]);
  });
});
