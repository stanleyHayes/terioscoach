import { render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { API_BASE_URL } from "@/lib/api";
import ServicesPage from "./page";

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

beforeEach(() => {
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  fetchMock.mockReset();
});

describe("Services page", () => {
  it("fetches the public catalog fresh (no-store) and renders the header", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(200, { items: services }));

    render(await ServicesPage());

    expect(fetchMock).toHaveBeenCalledWith(`${API_BASE_URL}/v1/services`, {
      method: "GET",
      headers: {},
      cache: "no-store",
    });
    expect(
      screen.getByRole("heading", {
        level: 1,
        name: /every session, clearly priced/i,
      }),
    ).toBeTruthy();
  });

  it("renders a card per service with formatted duration and price", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(200, { items: services }));

    render(await ServicesPage());

    const menu = screen.getByRole("region", { name: /service menu/i });
    expect(within(menu).getByText("Wellness coaching")).toBeTruthy();
    expect(within(menu).getByText("Nursing consultation")).toBeTruthy();
    // duration · price, price localized from minor units
    expect(within(menu).getByText(/45 min/)).toBeTruthy();
    expect(within(menu).getByText(/1 h 30 min/)).toBeTruthy();
    expect(within(menu).getByText("GH₵450.00")).toBeTruthy();
    expect(within(menu).getByText("GH₵1,200.00")).toBeTruthy();
  });

  it("links each card's secondary action to work-with-me with the service id", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(200, { items: services }));

    render(await ServicesPage());

    const bookLinks = screen.getAllByRole("link", { name: /book this/i });
    expect(bookLinks).toHaveLength(2);
    expect(bookLinks[0].getAttribute("href")).toBe("/work-with-me?service=s1");
    expect(bookLinks[1].getAttribute("href")).toBe("/work-with-me?service=s2");
  });

  it("gives every service an accompanying image", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(200, { items: services }));

    render(await ServicesPage());

    expect(screen.getByRole("img", { name: /wellness coaching at terios wellness/i })).toBeTruthy();
    expect(screen.getByRole("img", { name: /nursing consultation at terios wellness/i })).toBeTruthy();
  });

  it("shows a branded inline error with a retry link when the fetch fails", async () => {
    fetchMock.mockRejectedValueOnce(new TypeError("fetch failed"));

    render(await ServicesPage());

    expect(screen.getByRole("alert")).toBeTruthy();
    expect(screen.getByText(/the service menu didn’t load/i)).toBeTruthy();
    expect(
      screen.getByRole("link", { name: /try again/i }).getAttribute("href"),
    ).toBe("/services");
  });

  it("shows the API error state too (e.g. 503 service_unavailable)", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(503, {
        error: { code: "service_unavailable", message: "Down" },
      }),
    );

    render(await ServicesPage());

    expect(screen.getByRole("alert")).toBeTruthy();
  });

  it("renders the empty state when the catalog has no services", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(200, { items: [] }));

    render(await ServicesPage());

    expect(screen.getByText(/no services are published yet/i)).toBeTruthy();
    expect(
      screen.getByText(/there simply is not a live service to show right now/i),
    ).toBeTruthy();
  });
});
