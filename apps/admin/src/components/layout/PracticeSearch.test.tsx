import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { PracticeSearch, filterPracticeClients } from "./PracticeSearch";

const { push, list } = vi.hoisted(() => ({ push: vi.fn(), list: vi.fn() }));

vi.mock("next/navigation", () => ({ useRouter: () => ({ push }) }));
vi.mock("@/lib/auth", () => ({
  useAuth: () => ({
    session: { accessToken: "access", refreshToken: "refresh" },
    refreshCallbacks: { onTokensRefreshed: vi.fn() },
  }),
}));
vi.mock("@/lib/clients", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/clients")>();
  return { ...original, clientsApi: { ...original.clientsApi, list } };
});

const clients = [
  { id: "client-1", name: "Ama Mensah", email: "ama@example.com", phone: "+1 555 0100", tags: ["Nutrition"], totalSessions: 3 },
  { id: "client-2", name: "Kojo Asare", email: "kojo@example.com", tags: ["Mindfulness"], totalSessions: 1 },
];

describe("PracticeSearch", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    list.mockResolvedValue(clients);
  });

  it("opens from the visible control and navigates to a matching record", async () => {
    render(<PracticeSearch />);
    fireEvent.click(screen.getByRole("button", { name: /find clients and records/i }));
    const input = await screen.findByRole("textbox", { name: /search client records/i });
    await waitFor(() => expect(screen.getByRole("button", { name: /ama mensah/i })).toBeTruthy());

    fireEvent.change(input, { target: { value: "nutrition" } });
    fireEvent.click(screen.getByRole("button", { name: /ama mensah/i }));

    expect(push).toHaveBeenCalledWith("/clients/client-1");
  });

  it("opens with Command-K and submits the first result with Enter", async () => {
    render(<PracticeSearch />);
    fireEvent.keyDown(document, { key: "k", metaKey: true });
    const input = await screen.findByRole("textbox", { name: /search client records/i });
    await waitFor(() => expect(screen.getByText("Kojo Asare")).toBeTruthy());

    fireEvent.change(input, { target: { value: "kojo@example.com" } });
    fireEvent.keyDown(input, { key: "Enter" });

    expect(push).toHaveBeenCalledWith("/clients/client-2");
  });

  it("normalizes accents and searches phone numbers and tags", () => {
    expect(filterPracticeClients(clients, "AMA")).toHaveLength(1);
    expect(filterPracticeClients(clients, "555 0100")).toHaveLength(1);
    expect(filterPracticeClients(clients, "mindfulness")[0]?.id).toBe("client-2");
  });
});
