import { render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { PracticeReport } from "@/lib/insights";
import OverviewPage from "./page";

const state = vi.hoisted(() => ({
  user: { role: "practitioner", name: "Hayford Stanley", permissions: [] as string[] },
  data: null as PracticeReport | null,
  error: null as string | null,
}));

vi.mock("@/lib/auth", () => ({ useAuth: () => ({ user: state.user }) }));
vi.mock("@/lib/use-resource", () => ({
  useResource: () => ({ data: state.data, error: state.error, refresh: vi.fn(), set: vi.fn() }),
}));

function report(overrides: Partial<PracticeReport> = {}): PracticeReport {
  return {
    from: "2026-08-01T00:00:00Z",
    to: "2026-09-01T00:00:00Z",
    granularity: "day",
    summary: {
      sessionsCompleted: 7,
      sessionsUpcoming: 3,
      cancellations: 1,
      noShows: 0,
      newClients: 2,
      incomeKobo: 150000,
      refundedKobo: 10000,
      netKobo: 140000,
      currency: "GHS",
    },
    byService: [],
    series: [{ start: "2026-08-01T00:00:00Z", sessions: 2, incomeKobo: 50000 }],
    reviews: { count: 0, average: 0, distribution: {} },
    ...overrides,
  };
}

describe("OverviewPage", () => {
  beforeEach(() => {
    state.user = { role: "practitioner", name: "Hayford Stanley", permissions: [] };
    state.data = null;
    state.error = null;
  });

  it("greets the signed-in user and exposes every practice shortcut", () => {
    render(<OverviewPage />);
    expect(screen.getByRole("heading", { name: /welcome back, hayford/i })).toBeTruthy();
    const shortcuts = screen.getByRole("region", { name: /practice shortcuts/i });
    for (const name of ["Schedule", "Clients", "Availability", "Enquiries"]) {
      expect(within(shortcuts).getByText(name)).toBeTruthy();
    }
  });

  it("shows a loading snapshot only to users allowed to view reports", () => {
    const { rerender } = render(<OverviewPage />);
    expect(screen.getByRole("status", { name: /loading practice snapshot/i })).toBeTruthy();

    state.user = { role: "staff", name: "Ama Mensah", permissions: [] };
    rerender(<OverviewPage />);
    expect(screen.queryByRole("status", { name: /loading practice snapshot/i })).toBeNull();

    state.user = { role: "staff", name: "Ama Mensah", permissions: ["reports.view"] };
    rerender(<OverviewPage />);
    expect(screen.getByRole("status", { name: /loading practice snapshot/i })).toBeTruthy();
  });

  it("renders live KPIs and the accessible activity table", () => {
    state.data = report();
    render(<OverviewPage />);
    expect(screen.getByText("GH₵1,500.00")).toBeTruthy();
    expect(screen.getByText("GH₵1,400.00 net")).toBeTruthy();
    const table = screen.getByRole("table", { name: /daily practice activity/i });
    expect(within(table).getByText("50000")).toBeTruthy();
  });

  it("uses a compact empty state when the report has no series", () => {
    state.data = report({ series: [] });
    render(<OverviewPage />);
    expect(screen.getByRole("heading", { name: /your trend will start here/i })).toBeTruthy();
  });

  it("keeps a failed snapshot contained without hiding navigation", () => {
    state.error = "offline";
    render(<OverviewPage />);
    expect(screen.getByRole("alert").textContent).toMatch(/could not be loaded/i);
    expect(screen.getByRole("link", { name: /open calendar/i })).toBeTruthy();
  });
});
