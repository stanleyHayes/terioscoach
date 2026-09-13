import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import type { Service } from "@/lib/services";
import ServicesPage from "./page";
import { ServiceEditorPage } from "./ServiceEditorPage";
const push = vi.hoisted(() => vi.fn());
vi.mock("next/navigation", () => ({ useRouter: () => ({ push }) }));
vi.mock("@/lib/agreements", () => ({ agreementsApi: { list: vi.fn().mockResolvedValue([]) } }));

const logoutMock = vi.fn();
const listAllMock = vi.fn();
const createMock = vi.fn();
const updateMock = vi.fn();
const removeMock = vi.fn();

const session = { accessToken: "access", refreshToken: "refresh" };
const refreshCallbacks = { onTokensRefreshed: vi.fn() };

vi.mock("@/lib/auth", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/auth")>();
  return {
    ...original,
    useAuth: () => ({
      status: "authenticated",
      user: { id: "u1", email: "akosua@terios.com", role: "practitioner", name: "Akosua" },
      accessToken: session.accessToken,
      session,
      refreshCallbacks,
      login: vi.fn(),
      logout: logoutMock,
    }),
  };
});

vi.mock("@/lib/services", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/services")>();
  return {
    ...original,
    servicesApi: {
      listAll: (...args: unknown[]) => listAllMock(...args),
      create: (...args: unknown[]) => createMock(...args),
      update: (...args: unknown[]) => updateMock(...args),
      remove: (...args: unknown[]) => removeMock(...args),
    },
  };
});

function service(overrides: Partial<Service> = {}): Service {
  return {
    id: "svc-1",
    practitionerId: "prac-1",
    name: "Aromatherapy massage",
    description: "Full body, slow pressure",
    durationMinutes: 90,
    priceKobo: 25000,
    currency: "GHS",
    active: true,
    sortOrder: 0,
    createdAt: "2026-08-01T10:00:00Z",
    updatedAt: "2026-08-01T10:00:00Z",
    ...overrides,
  };
}

const massage = service();
const facial = service({
  id: "svc-2",
  name: "Deep cleanse facial",
  description: "",
  durationMinutes: 45,
  priceKobo: 15050,
  active: false,
  sortOrder: 1,
  createdAt: "2026-08-02T10:00:00Z",
});

afterEach(() => {
  push.mockReset();
  logoutMock.mockReset();
  listAllMock.mockReset();
  createMock.mockReset();
  updateMock.mockReset();
  removeMock.mockReset();
});

describe("ServicesPage", () => {
  it("renders the table with services from the API", async () => {
    listAllMock.mockResolvedValue([massage, facial]);
    render(<ServicesPage />);

    expect(await screen.findByText("Aromatherapy massage")).toBeTruthy();
    expect(screen.getByText("Deep cleanse facial")).toBeTruthy();
    expect(screen.getByText("GH₵250.00")).toBeTruthy();
    expect(screen.getByText("GH₵150.50")).toBeTruthy();
    expect(screen.getByText("1 hr 30 min")).toBeTruthy();
    expect(screen.getByText("45 min")).toBeTruthy();
    expect(screen.getAllByText("Published").length).toBeGreaterThan(0);
    expect(screen.getByText("Retired")).toBeTruthy();
    expect(listAllMock).toHaveBeenCalledWith(session, refreshCallbacks);
  });

  it("shows the empty state when there are no services", async () => {
    listAllMock.mockResolvedValue([]);
    render(<ServicesPage />);

    expect(await screen.findByText("No services yet")).toBeTruthy();
  });

  it("toggles active optimistically and PATCHes the service", async () => {
    listAllMock.mockResolvedValue([massage]);
    updateMock.mockResolvedValue({ ...massage, active: false });
    render(<ServicesPage />);

    const toggle = await screen.findByRole("switch", {
      name: "Retire Aromatherapy massage",
    });
    fireEvent.click(toggle);

    // Optimistic: the switch flips before the API answers.
    expect(
      screen.getByRole("switch", { name: "Republish Aromatherapy massage" }),
    ).toBeTruthy();
    await waitFor(() =>
      expect(updateMock).toHaveBeenCalledWith(session, refreshCallbacks, "svc-1", {
        active: false,
      }),
    );
    expect(await screen.findByText("Retired")).toBeTruthy();
  });

  it("reverts the toggle and shows a banner when the PATCH fails", async () => {
    listAllMock.mockResolvedValue([massage]);
    updateMock.mockRejectedValue(new ApiError(500, "unknown_error", "Couldn't save that. Try again."));
    render(<ServicesPage />);

    fireEvent.click(
      await screen.findByRole("switch", { name: "Retire Aromatherapy massage" }),
    );

    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toContain("Couldn't save that. Try again."),
    );
    const toggle = screen.getByRole("switch", { name: "Retire Aromatherapy massage" });
    expect(toggle.getAttribute("aria-checked")).toBe("true");
    expect(screen.getAllByText("Published").length).toBeGreaterThan(0);
  });

  it("validates the create form without native bubbles", async () => {
    listAllMock.mockResolvedValue([massage]);
    render(<ServiceEditorPage />);

    await screen.findByRole("heading", { name: "New service" });
    const dialog = document.getElementById("service-form")!;
    fireEvent.submit(document.getElementById("service-form")!);

    expect(within(dialog).getByText("Give the service a name")).toBeTruthy();
    expect(
      within(dialog).getByText("Enter a duration between 5 and 480 minutes"),
    ).toBeTruthy();
    expect(
      within(dialog).getByText("Enter a price in US dollars, e.g. 250 or 250.50"),
    ).toBeTruthy();
    expect(createMock).not.toHaveBeenCalled();
  });

  it("creates a service, converting the dollar price to minor units", async () => {
    listAllMock.mockResolvedValue([massage]);
    const created = service({
      id: "svc-3",
      name: "Sauna session",
      description: "",
      imageUrl: "",
      durationMinutes: 45,
      priceKobo: 25050,
      sortOrder: 2,
      createdAt: "2026-08-03T10:00:00Z",
    });
    createMock.mockResolvedValue(created);
    render(<ServiceEditorPage />);

    await screen.findByRole("heading", { name: "New service" });
    const dialog = document.getElementById("service-form")!;
    fireEvent.change(within(dialog).getByLabelText(/^Name/), {
      target: { value: "Sauna session" },
    });
    fireEvent.change(within(dialog).getByLabelText(/^Duration/), {
      target: { value: "45" },
    });
    fireEvent.change(within(dialog).getByLabelText(/^Price/), {
      target: { value: "250.50" },
    });
    fireEvent.submit(document.getElementById("service-form")!);

    await waitFor(() => expect(createMock).toHaveBeenCalled());
    expect(createMock.mock.calls[0]![2]).toEqual({
      name: "Sauna session",
      description: "",
      imageUrl: "",
      durationMinutes: 45,
      priceKobo: 25050,
      currency: "USD",
      // No agreement chosen: the service is open to anyone.
      agreementId: "",
    });
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(push).toHaveBeenCalledWith("/services");
  });

  it("pre-fills the edit form with the stored values", async () => {
    listAllMock.mockResolvedValue([facial]);
    render(<ServiceEditorPage serviceId="svc-2" />);
    await screen.findByRole("heading", { name: "Edit service" });
    const dialog = document.getElementById("service-form")!;

    expect((within(dialog).getByLabelText(/^Name/) as HTMLInputElement).value).toBe(
      "Deep cleanse facial",
    );
    expect((within(dialog).getByLabelText(/^Duration/) as HTMLInputElement).value).toBe("45");
    // 15050 kobo → "150.5" major units
    expect((within(dialog).getByLabelText(/^Price/) as HTMLInputElement).value).toBe("150.5");
  });

  it("confirms delete in a modal that names the service, then removes the row", async () => {
    listAllMock.mockResolvedValue([massage]);
    removeMock.mockResolvedValue(undefined);
    render(<ServicesPage />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Delete Aromatherapy massage" }),
    );
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Delete this service?")).toBeTruthy();
    expect(dialog.textContent).toContain("Aromatherapy massage");
    expect(dialog.textContent).toContain("Past bookings keep their record of this service.");

    fireEvent.click(within(dialog).getByRole("button", { name: "Delete service" }));

    await waitFor(() =>
      expect(removeMock).toHaveBeenCalledWith(session, refreshCallbacks, "svc-1"),
    );
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(screen.queryByText("Aromatherapy massage")).toBeNull();
    expect(await screen.findByText("No services yet")).toBeTruthy();
  });

  it("reorders with the up/down buttons by PATCHing swapped sortOrders", async () => {
    listAllMock.mockResolvedValue([massage, facial]);
    updateMock
      .mockResolvedValueOnce({ ...facial, sortOrder: 0 })
      .mockResolvedValueOnce({ ...massage, sortOrder: 1 });
    render(<ServicesPage />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Move Aromatherapy massage down" }),
    );

    await waitFor(() => expect(updateMock).toHaveBeenCalledTimes(2));
    expect(updateMock).toHaveBeenCalledWith(session, refreshCallbacks, "svc-2", {
      sortOrder: 0,
    });
    expect(updateMock).toHaveBeenCalledWith(session, refreshCallbacks, "svc-1", {
      sortOrder: 1,
    });
  });
});


it("opens dedicated new and edit routes without a form dialog", async () => {
  listAllMock.mockResolvedValue([facial]);
  render(<ServicesPage />);
  fireEvent.click(screen.getByRole("button", { name: "New service" }));
  expect(push).toHaveBeenCalledWith("/services/new");
  fireEvent.click(await screen.findByRole("button", { name: "Edit Deep cleanse facial" }));
  expect(push).toHaveBeenCalledWith("/services/svc-2/edit");
  expect(screen.queryByRole("dialog")).toBeNull();
});

it("saves edits to the selected service and keeps failures visible", async () => {
  listAllMock.mockResolvedValue([facial]);
  updateMock.mockRejectedValueOnce(new ApiError(400, "invalid_service", "Please check the service details."))
    .mockResolvedValueOnce({ ...facial, name: "Updated facial" });
  render(<ServiceEditorPage serviceId="svc-2" />);
  const name = await screen.findByLabelText(/^Name/);
  fireEvent.change(name, { target: { value: "Updated facial" } });
  fireEvent.submit(document.getElementById("service-form")!);
  await screen.findByText("Please check the service details.");
  expect(push).not.toHaveBeenCalled();
  expect((name as HTMLInputElement).value).toBe("Updated facial");
  fireEvent.submit(document.getElementById("service-form")!);
  await waitFor(() => expect(push).toHaveBeenCalledWith("/services"));
  expect(updateMock.mock.lastCall?.[2]).toBe("svc-2");
  expect(updateMock.mock.lastCall?.[3].name).toBe("Updated facial");
});

it("does not turn an unknown service into a create form", async () => {
  listAllMock.mockResolvedValue([]);
  render(<ServiceEditorPage serviceId="deleted" />);
  expect(await screen.findByText(/This service was not found/)).toBeTruthy();
  expect(screen.queryByLabelText(/^Name/)).toBeNull();
});

it("confirms discarding edits when returning to services", async () => {
  render(<ServiceEditorPage />);
  fireEvent.change(await screen.findByLabelText(/^Name/), { target: { value: "Unsaved" } });
  fireEvent.click(screen.getByRole("button", { name: "Back to services" }));
  const dialog = await screen.findByRole("dialog");
  expect(push).not.toHaveBeenCalled();
  fireEvent.click(within(dialog).getByRole("button", { name: "Keep editing" }));
  expect((screen.getByLabelText(/^Name/) as HTMLInputElement).value).toBe("Unsaved");
});
