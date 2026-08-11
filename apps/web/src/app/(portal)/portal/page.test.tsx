import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import PortalOverviewPage from "./page";

vi.mock("@/lib/auth", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/auth")>();
  return {
    ...original,
    useAuth: () => ({
      status: "authenticated",
      user: { id: "u1", email: "ama@example.com", role: "client", name: "Ama Serwaa" },
      accessToken: "a1",
      login: vi.fn(),
      register: vi.fn(),
      logout: vi.fn(),
    }),
  };
});

describe("PortalOverviewPage", () => {
  it("welcomes the client by name", () => {
    render(<PortalOverviewPage />);

    expect(
      screen.getByRole("heading", { level: 1, name: /welcome back, ama serwaa/i }),
    ).toBeTruthy();
  });

  it("renders the sessions empty state per the design-system voice", () => {
    render(<PortalOverviewPage />);

    expect(screen.getByRole("heading", { level: 2, name: "No sessions yet" })).toBeTruthy();
    expect(screen.getByText(/Your upcoming sessions will appear here/)).toBeTruthy();
  });
});
