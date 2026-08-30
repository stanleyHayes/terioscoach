import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AdminTopbar } from "./AdminTopbar";

const pathname = vi.hoisted(() => vi.fn());

vi.mock("next/navigation", () => ({ usePathname: pathname }));
vi.mock("./AdminNotificationCenter", () => ({ AdminNotificationCenter: () => <button>Notifications</button> }));

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

    fireEvent.click(screen.getByRole("button", { name: /naa adjeley/i }));
    fireEvent.click(screen.getByRole("button", { name: /sign out/i }));
    expect(onSignOut).toHaveBeenCalledTimes(1);
  });

  it("shows the sign-out in progress rather than letting it be pressed twice", () => {
    const onSignOut = vi.fn();
    render(
      <AdminTopbar userName="Naa Adjeley" onSignOut={onSignOut} signingOut />,
    );

    fireEvent.click(screen.getByRole("button", { name: /naa adjeley/i }));
    const button = screen.getByRole("button", { name: /sign out/i });
    expect(button.hasAttribute("disabled")).toBe(true);
    fireEvent.click(button);
    expect(onSignOut).not.toHaveBeenCalled();
  });

  it("names the current section", () => {
    pathname.mockReturnValue("/clients");
    render(<AdminTopbar userName="Naa Adjeley" onSignOut={vi.fn()} />);

    expect(screen.getByText("Clients")).toBeTruthy();
  });

  it("keeps a nested route inside its section", () => {
    pathname.mockReturnValue("/clients/client-1");
    render(<AdminTopbar userName="Naa Adjeley" onSignOut={vi.fn()} />);

    expect(screen.getByText("Clients")).toBeTruthy();
  });

  it("does not treat Overview as the parent of everything", () => {
    pathname.mockReturnValue("/reports");
    render(<AdminTopbar userName="Naa Adjeley" onSignOut={vi.fn()} />);

    expect(screen.getByText("Reports")).toBeTruthy();
  });

  it("highlights Overview only on the overview itself", () => {
    pathname.mockReturnValue("/");
    render(<AdminTopbar userName="Naa Adjeley" onSignOut={vi.fn()} />);

    expect(screen.getByText("Overview")).toBeTruthy();
  });

  it("shows the staff role in the account menu", () => {
    render(
      <AdminTopbar
        userName="Naa Adjeley"
        userRole="Care coordinator"
        onSignOut={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /naa adjeley/i }));
    expect(screen.getByText("Care coordinator")).toBeTruthy();
  });

  it("opens page-specific help and links the full user guide", () => {
    pathname.mockReturnValue("/services");
    render(<AdminTopbar userName="Naa Adjeley" onSignOut={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: "Help with Services" }));
    expect(screen.getByRole("dialog", { name: "How to use Services" })).toBeTruthy();
    expect(screen.getByText(/active and Availability/i)).toBeTruthy();
    expect(screen.getByRole("link", { name: "Open full user guide" }).getAttribute("href")).toBe("/guide");
  });

  it("links the user guide from the account dropdown", () => {
    render(<AdminTopbar userName="Naa Adjeley" onSignOut={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: /naa adjeley/i }));
    expect(screen.getByRole("menuitem", { name: "User guide" }).getAttribute("href")).toBe("/guide");
  });
});
