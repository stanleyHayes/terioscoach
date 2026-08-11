import { render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import WorkWithMePage from "./page";

const services = [
  {
    id: "s1",
    name: "Wellness coaching",
    description: "Ongoing one-on-one coaching to build sustainable rhythms.",
    durationMinutes: 45,
    priceKobo: 45000,
    currency: "GHS",
    sortOrder: 1,
  },
  {
    id: "s2",
    name: "Nursing consultation",
    description: "Clinical guidance from a registered nurse, by video.",
    durationMinutes: 90,
    priceKobo: 120000,
    currency: "GHS",
    sortOrder: 2,
  },
];

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const fetchMock = vi.fn();

function renderPage(searchParams: Promise<{ service?: string }>) {
  return WorkWithMePage({ searchParams }).then(render);
}

beforeEach(() => {
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  fetchMock.mockReset();
});

describe("Work With Me page", () => {
  it("renders the header and compact service list with prices", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(200, { items: services }));

    await renderPage(Promise.resolve({}));

    expect(
      screen.getByRole("heading", {
        level: 1,
        name: /begin with a single step/i,
      }),
    ).toBeTruthy();
    const chooser = screen.getByRole("region", {
      name: /choose your service/i,
    });
    expect(within(chooser).getByText("Wellness coaching")).toBeTruthy();
    expect(within(chooser).getByText(/1 h 30 min/)).toBeTruthy();
    expect(within(chooser).getByText("GH₵1,200.00")).toBeTruthy();
  });

  it("links every service's primary action to the #book placeholder", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(200, { items: services }));

    await renderPage(Promise.resolve({}));

    const chooseLinks = screen.getAllByRole("link", { name: /choose/i });
    expect(chooseLinks).toHaveLength(2);
    for (const link of chooseLinks) {
      expect(link.getAttribute("href")).toBe("#book");
    }
  });

  it("renders the three booking steps and the account-creation note", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(200, { items: services }));

    await renderPage(Promise.resolve({}));

    const steps = screen.getByRole("region", {
      name: /three steps, no paperwork/i,
    });
    expect(within(steps).getByText(/choose a service/i)).toBeTruthy();
    expect(within(steps).getByText(/pick a time/i)).toBeTruthy();
    expect(within(steps).getByText(/confirm & pay/i)).toBeTruthy();
    expect(
      within(steps).getByText(/you create yours during booking/i),
    ).toBeTruthy();
  });

  it("pre-highlights the service named in ?service=", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(200, { items: services }));

    await renderPage(Promise.resolve({ service: "s2" }));

    const marker = screen.getByText("Selected");
    const card = marker.closest("[aria-current]");
    expect(card).not.toBeNull();
    expect(within(card as HTMLElement).getByText("Nursing consultation")).toBeTruthy();
  });

  it("highlights nothing when ?service= is absent or unknown", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(200, { items: services }));

    await renderPage(Promise.resolve({ service: "nope" }));

    expect(screen.queryByText("Selected")).toBeNull();
  });

  it("shows an inline error instead of crashing when the catalog fails", async () => {
    fetchMock.mockRejectedValueOnce(new TypeError("fetch failed"));

    await renderPage(Promise.resolve({}));

    expect(screen.getByRole("alert")).toBeTruthy();
    expect(
      screen.getByRole("link", { name: /try again/i }).getAttribute("href"),
    ).toBe("/work-with-me");
  });
});
