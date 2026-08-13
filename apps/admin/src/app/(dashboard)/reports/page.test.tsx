import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { PracticeReport } from "@/lib/insights";
import ReportsPage from "./page";

const practice = vi.hoisted(() => vi.fn());

vi.mock("@/lib/insights", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/insights")>();
  return { ...original, reportsApi: { practice, upcomingLoad: vi.fn() } };
});

vi.mock("@/lib/auth", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/auth")>();
  return {
    ...original,
    useAuth: () => ({
      status: "authenticated",
      user: { id: "prac-1", email: "t@example.com", role: "practitioner", name: "Terios" },
      session: { accessToken: "a1", refreshToken: "r1" },
      refreshCallbacks: { onTokensRefreshed: vi.fn() },
      logout: vi.fn(),
    }),
  };
});

function report(overrides: Partial<PracticeReport> = {}): PracticeReport {
  return {
    from: "2026-08-01T00:00:00Z",
    to: "2026-09-01T00:00:00Z",
    granularity: "day",
    summary: {
      sessionsCompleted: 12,
      sessionsUpcoming: 3,
      cancellations: 1,
      noShows: 2,
      newClients: 4,
      incomeKobo: 250000,
      refundedKobo: 25000,
      netKobo: 225000,
      currency: "GHS",
    },
    byService: [
      { serviceId: "svc-1", name: "Deep Tissue Massage", sessions: 8, incomeKobo: 200000 },
      { serviceId: "svc-2", name: "", sessions: 1, incomeKobo: 0 },
    ],
    series: [
      { start: "2026-08-01T00:00:00Z", sessions: 2, incomeKobo: 50000 },
      { start: "2026-08-02T00:00:00Z", sessions: 0, incomeKobo: 0 },
    ],
    reviews: { count: 6, average: 4.8, distribution: { "5": 5, "4": 1 } },
    ...overrides,
  };
}

describe("ReportsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    practice.mockResolvedValue(report());
  });

  it("shows the headline numbers for the month", async () => {
    const { container } = render(<ReportsPage />);
    await screen.findByText("12");

    const stats = within(container.querySelector("dl")!);
    expect(stats.getByText("Sessions").nextElementSibling?.textContent).toBe("12");
    expect(stats.getByText("Coming up").nextElementSibling?.textContent).toBe("3");
    expect(stats.getByText("New clients").nextElementSibling?.textContent).toBe("4");
    expect(stats.getByText("Taken").nextElementSibling?.textContent).toContain("2,500.00");
  });

  it("keeps cancellations and no-shows apart — they read differently", async () => {
    render(<ReportsPage />);
    await screen.findByText("12");

    expect(screen.getByText("Cancellations").nextElementSibling?.textContent).toBe("1");
    expect(screen.getByText("No shows").nextElementSibling?.textContent).toBe("2");
  });

  it("breaks income down per service, naming a retired one honestly", async () => {
    render(<ReportsPage />);

    const section = await screen.findByRole("region", { name: /by service/i });
    expect(within(section).getByText("Deep Tissue Massage")).toBeTruthy();
    // A service deleted since the sessions happened still has to be named.
    expect(within(section).getByText("Retired service")).toBeTruthy();
  });

  it("gives the chart an accessible table of the same numbers", async () => {
    render(<ReportsPage />);

    const section = await screen.findByRole("region", { name: /income by day/i });
    const table = within(section).getByRole("table");
    // The bars are decorative; the table is the readable truth.
    expect(within(table).getByText("2")).toBeTruthy();
  });

  it("refetches when the grouping changes", async () => {
    render(<ReportsPage />);
    await screen.findByText("12");

    fireEvent.click(screen.getByRole("button", { name: "week" }));

    await waitFor(() => {
      const lastCall = practice.mock.calls[practice.mock.calls.length - 1][2];
      expect(lastCall.granularity).toBe("week");
    });
  });

  it("moves between months", async () => {
    render(<ReportsPage />);
    await screen.findByText("12");
    const first = practice.mock.calls[0][2];

    fireEvent.click(screen.getByRole("button", { name: /previous/i }));

    await waitFor(() => {
      const last = practice.mock.calls[practice.mock.calls.length - 1][2];
      expect(new Date(last.from).getTime()).toBeLessThan(new Date(first.from).getTime());
    });
  });

  it("shows the review average when there are published reviews", async () => {
    render(<ReportsPage />);

    expect(await screen.findByText("4.8")).toBeTruthy();
    expect(screen.getByText(/6 published reviews/i)).toBeTruthy();
  });

  it("omits the review line when nothing has been published", async () => {
    practice.mockResolvedValue(
      report({ reviews: { count: 0, average: 0, distribution: {} } }),
    );

    render(<ReportsPage />);
    await screen.findByText("12");

    expect(screen.queryByText(/published review/i)).toBeNull();
  });

  it("says so plainly when a period is empty", async () => {
    practice.mockResolvedValue(report({ series: [], byService: [] }));

    render(<ReportsPage />);

    expect(await screen.findByText(/nothing in this period/i)).toBeTruthy();
    expect(screen.getByText(/no sessions in this period/i)).toBeTruthy();
  });

  it("offers a retry when the report will not load", async () => {
    const { ApiError } = await import("@/lib/api");
    practice.mockRejectedValue(new ApiError(0, "network_error", "Can't reach the server."));

    render(<ReportsPage />);

    expect(await screen.findByRole("button", { name: /try again/i })).toBeTruthy();
  });
});
