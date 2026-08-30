import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import PortalSettingsPage from "./page";

const updateProfile = vi.hoisted(() => vi.fn());
const changePassword = vi.hoisted(() => vi.fn());
const logout = vi.hoisted(() => vi.fn());
const setUserProfile = vi.hoisted(() => vi.fn());
vi.mock("@/lib/api", async (load) => ({ ...(await load<typeof import("@/lib/api")>()), accountApi: { updateProfile, changePassword } }));
vi.mock("@/lib/auth", () => ({ useAuth: () => ({ user: { id: "u1", name: "Ama", email: "ama@example.com" }, session: { accessToken: "a" }, onTokensRefreshed: vi.fn(), setUserProfile, logout }) }));

describe("PortalSettingsPage", () => {
  beforeEach(() => { vi.clearAllMocks(); localStorage.clear(); updateProfile.mockResolvedValue({ user: { id: "u1", name: "Ama Serwaa", email: "ama@example.com" } }); changePassword.mockResolvedValue(undefined); });

  it("updates profile and preferences", async () => {
    render(<PortalSettingsPage />);
    fireEvent.change(screen.getByRole("textbox", { name: /full name/i }), { target: { value: "Ama Serwaa" } });
    fireEvent.click(screen.getByRole("button", { name: /save profile/i }));
    await waitFor(() => expect(setUserProfile).toHaveBeenCalled());
    expect(screen.getByRole("status").textContent).toContain("Profile updated");
    fireEvent.click(screen.getByRole("checkbox"));
    fireEvent.click(screen.getByRole("button", { name: /save preferences/i }));
    expect(localStorage.getItem("terios.portal.preferences")).toContain("false");
  });

  it("changes the password and signs every device out", async () => {
    render(<PortalSettingsPage />);
    fireEvent.change(screen.getByLabelText(/current password/i), { target: { value: "old-password-123" } });
    fireEvent.change(screen.getByLabelText(/new password/i), { target: { value: "new-password-123" } });
    fireEvent.click(screen.getByRole("button", { name: /update password/i }));
    await waitFor(() => expect(logout).toHaveBeenCalled());
  });

  it("shows safe profile and password failures", async () => {
    const { ApiError } = await import("@/lib/api");
    updateProfile.mockRejectedValueOnce(new ApiError(400, "name_required", "Enter your name"));
    changePassword.mockRejectedValueOnce(new Error("offline"));
    render(<PortalSettingsPage />);
    fireEvent.click(screen.getByRole("button", { name: /save profile/i }));
    expect(await screen.findByText("Enter your name")).toBeTruthy();
    fireEvent.change(screen.getByLabelText(/current password/i), { target: { value: "old-password-123" } });
    fireEvent.change(screen.getByLabelText(/new password/i), { target: { value: "new-password-123" } });
    fireEvent.click(screen.getByRole("button", { name: /update password/i }));
    expect(await screen.findByText("Password could not be updated.")).toBeTruthy();
  });
});
