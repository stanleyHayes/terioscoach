import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { StaffMember } from "@/lib/team";
import TeamPage from "./page";

const list = vi.hoisted(() => vi.fn());
const create = vi.hoisted(() => vi.fn());
const update = vi.hoisted(() => vi.fn());
const authState = vi.hoisted(() => ({ role: "practitioner", permissions: [] as string[] }));

vi.mock("@/lib/team", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/team")>();
  return { ...original, teamApi: { list, create, update } };
});
vi.mock("@/lib/auth", () => ({
  useAuth: () => ({
    user: { id: "owner", name: "Owner", role: authState.role, permissions: authState.permissions },
    session: { accessToken: "a", refreshToken: "r" },
    refreshCallbacks: { onTokensRefreshed: vi.fn() },
    logout: vi.fn(),
  }),
}));

const owner: StaffMember = {
  id: "owner",
  email: "owner@example.com",
  name: "Practice Owner",
  role: "practitioner",
  roleName: "Owner",
  permissions: [],
  disabled: false,
  mfaEnabled: true,
};
const staff: StaffMember = {
  id: "staff-1",
  email: "ama@example.com",
  name: "Ama Mensah",
  role: "staff",
  roleName: "Care coordinator",
  permissions: ["dashboard.view", "schedule.manage"],
  disabled: false,
  mfaEnabled: false,
};

describe("TeamPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    authState.role = "practitioner";
    authState.permissions = [];
    list.mockResolvedValue({ items: [owner, staff] });
  });

  it("summarizes accounts and distinguishes owner from staff", async () => {
    render(<TeamPage />);
    expect(await screen.findByText("Practice Owner")).toBeTruthy();
    expect(screen.getByText("Ama Mensah")).toBeTruthy();
    expect(screen.getByText("Staff accounts").parentElement?.parentElement?.querySelector("dd")?.textContent).toBe("1");
    expect(screen.getByText("MFA enabled").parentElement?.parentElement?.querySelector("dd")?.textContent).toBe("1");
    expect(screen.getByText((_, element) => element?.textContent === "Full practice access · MFA on")).toBeTruthy();
  });

  it("creates a staff account from a preset and reveals the one-time password", async () => {
    create.mockResolvedValue({ member: staff, temporaryPassword: "Temp-123!" });
    render(<TeamPage />);
    await screen.findByText("Ama Mensah");
    fireEvent.click(screen.getByRole("button", { name: /add staff member/i }));
    fireEvent.change(screen.getByLabelText(/full name/i), { target: { value: "Kojo Asare" } });
    fireEvent.change(screen.getByLabelText(/email address/i), { target: { value: "kojo@example.com" } });
    fireEvent.click(screen.getByRole("button", { name: "Finance officer" }));
    fireEvent.click(screen.getByRole("button", { name: /create account/i }));
    await waitFor(() => expect(create).toHaveBeenCalled());
    expect(await screen.findByText("Temp-123!")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Done" }));
    expect(screen.queryByText("Temp-123!")).toBeNull();
  });

  it("validates a blank create form without calling the API", async () => {
    render(<TeamPage />);
    await screen.findByText("Ama Mensah");
    fireEvent.click(screen.getByRole("button", { name: /add staff member/i }));
    fireEvent.click(screen.getByRole("button", { name: /create account/i }));
    expect(await screen.findByText(/add a name, email, role/i)).toBeTruthy();
    expect(create).not.toHaveBeenCalled();
  });

  it("edits permissions and disables a staff account", async () => {
    update.mockResolvedValue({ member: { ...staff, disabled: true } });
    render(<TeamPage />);
    await screen.findByText("Ama Mensah");
    fireEvent.click(screen.getByRole("button", { name: /manage access/i }));
    const dialog = screen.getByRole("dialog", { name: /manage staff access/i });
    fireEvent.click(within(dialog).getByRole("button", { name: "Payments" }));
    fireEvent.click(within(dialog).getByRole("switch", { name: /account enabled/i }));
    fireEvent.click(within(dialog).getByRole("button", { name: /save access/i }));
    await waitFor(() => expect(update).toHaveBeenCalled());
    const draft = update.mock.calls[0][3];
    expect(draft.permissions).toContain("payments.manage");
    expect(draft.disabled).toBe(true);
  });

  it("shows an actionable empty state and hides management from unprivileged staff", async () => {
    list.mockResolvedValue({ items: [] });
    const { rerender } = render(<TeamPage />);
    expect(await screen.findByRole("heading", { name: /no staff members yet/i })).toBeTruthy();
    expect(screen.getAllByRole("button", { name: /add staff member/i })).toHaveLength(2);

    authState.role = "staff";
    authState.permissions = [];
    rerender(<TeamPage />);
    expect(screen.getAllByRole("button", { name: /add staff member/i })).toHaveLength(1);
  });

  it("surfaces list and mutation errors", async () => {
    list.mockRejectedValueOnce(new Error("Team list unavailable"));
    const { unmount } = render(<TeamPage />);
    expect(await screen.findByText(/something went wrong/i)).toBeTruthy();
    unmount();

    list.mockResolvedValue({ items: [owner, staff] });
    update.mockRejectedValue(new Error("Could not save access"));
    render(<TeamPage />);
    await screen.findByText("Ama Mensah");
    fireEvent.click(screen.getByRole("button", { name: /manage access/i }));
    fireEvent.click(screen.getByRole("button", { name: /save access/i }));
    expect(await screen.findByText("Could not save access")).toBeTruthy();
  });
});
