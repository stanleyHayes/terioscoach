import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ClientSummary } from "@/lib/clients";
import ClientsPage, { filterClients } from "./page";

const list = vi.hoisted(() => vi.fn());

vi.mock("@/lib/clients", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/clients")>();
  return { ...original, clientsApi: { list, get: vi.fn(), updateProfile: vi.fn() } };
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

function client(overrides: Partial<ClientSummary> = {}): ClientSummary {
  return {
    id: "client-1",
    name: "Ama Serwaa",
    email: "ama@example.com",
    tags: ["regular"],
    totalSessions: 4,
    lastSessionAt: "2026-08-01T09:00:00Z",
    ...overrides,
  };
}

describe("ClientsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    list.mockResolvedValue([client()]);
  });

  it("lists clients with their session count and link into the file", async () => {
    render(<ClientsPage />);

    const link = await screen.findByRole("link", { name: "Ama Serwaa" });
    expect(link.getAttribute("href")).toBe("/clients/client-1");
    expect(screen.getByText("ama@example.com")).toBeTruthy();
    expect(within(screen.getByRole("table")).getByText("4")).toBeTruthy();
    expect(screen.getByText("regular")).toBeTruthy();
  });

  it("says plainly when a client has not been seen yet", async () => {
    list.mockResolvedValue([client({ lastSessionAt: undefined })]);

    render(<ClientsPage />);

    expect(await screen.findByText(/not yet/i)).toBeTruthy();
  });

  it("filters locally as you type", async () => {
    list.mockResolvedValue([client(), client({ id: "client-2", name: "Koffi Mensah", email: "koffi@example.com", tags: [] })]);
    render(<ClientsPage />);
    await screen.findByRole("link", { name: "Ama Serwaa" });

    fireEvent.change(screen.getByLabelText(/search clients/i), { target: { value: "koffi" } });

    expect(screen.getByRole("link", { name: "Koffi Mensah" })).toBeTruthy();
    expect(screen.queryByRole("link", { name: "Ama Serwaa" })).toBeNull();
  });

  it("explains an empty search rather than showing a blank table", async () => {
    render(<ClientsPage />);
    await screen.findByRole("link", { name: "Ama Serwaa" });

    fireEvent.change(screen.getByLabelText(/search clients/i), { target: { value: "zzz" } });

    expect(screen.getByRole("heading", { name: /nobody matches that/i })).toBeTruthy();
  });

  it("welcomes a practice with no clients yet", async () => {
    list.mockResolvedValue([]);

    render(<ClientsPage />);

    expect(await screen.findByRole("heading", { name: /no clients yet/i })).toBeTruthy();
  });

  it("offers a retry when the list will not load", async () => {
    const { ApiError } = await import("@/lib/api");
    list.mockRejectedValue(new ApiError(0, "network_error", "Can't reach the server."));

    render(<ClientsPage />);

    expect(await screen.findByRole("button", { name: /try again/i })).toBeTruthy();
  });
});

describe("filterClients", () => {
  const clients = [
    client({ id: "1", name: "Ama Séro", email: "ama@example.com", tags: ["regular"] }),
    client({ id: "2", name: "Koffi Mensah", email: "koffi@example.com", tags: ["deep tissue"] }),
  ];

  it("matches name, email and tags", () => {
    expect(filterClients(clients, "koffi").map((c) => c.id)).toEqual(["2"]);
    expect(filterClients(clients, "ama@").map((c) => c.id)).toEqual(["1"]);
    expect(filterClients(clients, "deep").map((c) => c.id)).toEqual(["2"]);
  });

  it("ignores case and accents", () => {
    expect(filterClients(clients, "SERO").map((c) => c.id)).toEqual(["1"]);
    expect(filterClients(clients, "séro").map((c) => c.id)).toEqual(["1"]);
  });

  it("returns everything for a blank query", () => {
    expect(filterClients(clients, "  ")).toHaveLength(2);
  });
});
