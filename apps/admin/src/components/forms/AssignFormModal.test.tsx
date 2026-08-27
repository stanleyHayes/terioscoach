import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import type { ClientSummary } from "@/lib/clients";
import type { FormDefinition } from "@/lib/forms";
import { AssignFormModal } from "./AssignFormModal";

const listClients = vi.hoisted(() => vi.fn());

vi.mock("@/lib/clients", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/clients")>();
  return { ...original, clientsApi: { ...original.clientsApi, list: listClients } };
});

vi.mock("@/lib/auth", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/auth")>();
  // One frozen value, not a fresh literal per render. The real provider
  // memoizes; a mock that doesn't would re-run every useResource effect on
  // every render and hide exactly the kind of refetch loop worth catching.
  const value = {
    status: "authenticated",
    user: { id: "prac-1", email: "t@example.com", role: "practitioner", name: "Terios" },
    session: { accessToken: "a1", refreshToken: "r1" },
    refreshCallbacks: { onTokensRefreshed: vi.fn() },
    logout: vi.fn(),
  };
  return { ...original, useAuth: () => value };
});

const form: FormDefinition = {
  id: "form-1",
  title: "Intake and consent",
  description: "",
  fields: [],
  template: false,
  sortOrder: 1,
  active: true,
  createdAt: "2026-08-01T09:00:00.000Z",
  updatedAt: "2026-08-01T09:00:00.000Z",
};

function client(overrides: Partial<ClientSummary> = {}): ClientSummary {
  return {
    id: "client-1",
    name: "Ama Serwaa",
    email: "ama@example.com",
    tags: [],
    totalSessions: 2,
    ...overrides,
  };
}

const roster = [
  client(),
  client({ id: "client-2", name: "Kojo Mensah", email: "kojo@example.com" }),
];

function open(onAssign = vi.fn().mockResolvedValue(undefined), onClose = vi.fn()) {
  render(<AssignFormModal form={form} onClose={onClose} onAssign={onAssign} />);
  return { onAssign, onClose };
}

describe("AssignFormModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listClients.mockResolvedValue(roster);
  });

  it("names the form being sent", async () => {
    open();
    expect(screen.getByRole("heading", { name: /send "intake and consent"/i })).toBeTruthy();
    expect(await screen.findByRole("radio", { name: /ama serwaa/i })).toBeTruthy();
  });

  it("says it is loading rather than showing an empty roster", () => {
    listClients.mockReturnValue(new Promise(() => {}));
    open();

    // "You have no clients yet" while the request is open would be a lie
    // about the practice.
    expect(screen.getByRole("status")).toBeTruthy();
    expect(screen.queryByText(/you have no clients yet/i)).toBeNull();
  });

  it("filters on name and on email", async () => {
    open();
    await screen.findByRole("radio", { name: /ama serwaa/i });
    const search = screen.getByLabelText(/find a client/i);

    fireEvent.change(search, { target: { value: "kojo" } });
    expect(screen.getAllByRole("radio")).toHaveLength(1);
    expect(screen.getByRole("radio", { name: /kojo mensah/i })).toBeTruthy();

    // A practitioner who remembers the address but not the spelling of the
    // name has to be able to find them too.
    fireEvent.change(search, { target: { value: "AMA@EXAMPLE" } });
    expect(screen.getByRole("radio", { name: /ama serwaa/i })).toBeTruthy();
  });

  it("ignores surrounding whitespace in the search", async () => {
    open();
    await screen.findByRole("radio", { name: /ama serwaa/i });

    fireEvent.change(screen.getByLabelText(/find a client/i), { target: { value: "   " } });
    expect(screen.getAllByRole("radio")).toHaveLength(2);
  });

  it("distinguishes an empty search result from an empty practice", async () => {
    open();
    await screen.findByRole("radio", { name: /ama serwaa/i });

    fireEvent.change(screen.getByLabelText(/find a client/i), { target: { value: "nobody" } });
    expect(screen.getByText(/no one matches that/i)).toBeTruthy();

    fireEvent.change(screen.getByLabelText(/find a client/i), { target: { value: "" } });
    expect(screen.queryByText(/no one matches that/i)).toBeNull();
  });

  it("says the practice has no clients when it has none", async () => {
    listClients.mockResolvedValue([]);
    open();

    expect(await screen.findByText(/you have no clients yet/i)).toBeTruthy();
  });

  it("will not send until someone is chosen", async () => {
    const { onAssign } = open();
    await screen.findByRole("radio", { name: /ama serwaa/i });
    const send = screen.getByRole("button", { name: /send it/i });

    // Sending a consent form to nobody, or to whoever happened to be first
    // in the list, is not a mistake worth allowing.
    expect(send.hasAttribute("disabled")).toBe(true);
    fireEvent.click(send);
    expect(onAssign).not.toHaveBeenCalled();
  });

  it("sends to the chosen client and closes", async () => {
    const { onAssign, onClose } = open();
    fireEvent.click(await screen.findByRole("radio", { name: /kojo mensah/i }));

    expect(screen.getByRole("radio", { name: /kojo mensah/i }).getAttribute("aria-checked")).toBe("true");
    fireEvent.click(screen.getByRole("button", { name: /send it/i }));

    await waitFor(() => expect(onAssign).toHaveBeenCalledWith("client-2"));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it("selects exactly one client at a time", async () => {
    open();
    fireEvent.click(await screen.findByRole("radio", { name: /ama serwaa/i }));
    fireEvent.click(screen.getByRole("radio", { name: /kojo mensah/i }));

    const checked = screen.getAllByRole("radio").filter((r) => r.getAttribute("aria-checked") === "true");
    expect(checked).toHaveLength(1);
    expect(checked[0].textContent).toMatch(/kojo mensah/i);
  });

  it("stays open and explains itself when sending fails", async () => {
    const onAssign = vi.fn().mockRejectedValue(new ApiError(409, "already_assigned", "They already have this form."));
    const { onClose } = open(onAssign);
    fireEvent.click(await screen.findByRole("radio", { name: /ama serwaa/i }));

    fireEvent.click(screen.getByRole("button", { name: /send it/i }));

    expect((await screen.findByRole("alert")).textContent).toMatch(/already have this form/i);
    // Closing on failure would leave the practitioner believing it was
    // sent.
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: /send it/i }).hasAttribute("disabled")).toBe(false);
  });

  it("falls back to a generic message for an unrecognised failure", async () => {
    const onAssign = vi.fn().mockRejectedValue(new Error("boom"));
    open(onAssign);
    fireEvent.click(await screen.findByRole("radio", { name: /ama serwaa/i }));

    fireEvent.click(screen.getByRole("button", { name: /send it/i }));

    expect((await screen.findByRole("alert")).textContent).toMatch(/something went wrong/i);
  });

  it("reports a roster that will not load", async () => {
    listClients.mockRejectedValue(new ApiError(500, "server_error", "clients are unavailable"));
    open();

    expect((await screen.findByRole("alert")).textContent).toMatch(/clients are unavailable/i);
    expect(screen.queryByRole("radiogroup")).toBeNull();
  });

  it("cancels without sending anything", async () => {
    const { onAssign, onClose } = open();
    fireEvent.click(await screen.findByRole("radio", { name: /ama serwaa/i }));

    fireEvent.click(screen.getByRole("button", { name: /cancel/i }));

    expect(onClose).toHaveBeenCalledTimes(1);
    expect(onAssign).not.toHaveBeenCalled();
  });
});
