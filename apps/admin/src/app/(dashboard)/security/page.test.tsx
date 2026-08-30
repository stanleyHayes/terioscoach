import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import SecurityPage from "./page";

const { beginMfa, confirmMfa, disableMfa, logout, setMfaEnabled, authState } = vi.hoisted(() => ({ beginMfa: vi.fn(), confirmMfa: vi.fn(), disableMfa: vi.fn(), logout: vi.fn(), setMfaEnabled: vi.fn(), authState: { mfaEnabled: false } }));

vi.mock("@/lib/api", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/api")>();
  return { ...original, authApi: { ...original.authApi, beginMfa, confirmMfa, disableMfa } };
});
vi.mock("@/lib/auth", () => ({ useAuth: () => ({ user: { id: "u1", name: "Admin", email: "admin@example.com", role: "practitioner", mfaEnabled: authState.mfaEnabled }, accessToken: "access", logout, setMfaEnabled }) }));

afterEach(() => { beginMfa.mockReset(); confirmMfa.mockReset(); disableMfa.mockReset(); logout.mockReset(); setMfaEnabled.mockReset(); authState.mfaEnabled = false; });

function enterCode(label: string, code = "123456") {
  for (const [index, digit] of [...code].entries()) fireEvent.change(screen.getByLabelText(`${label} digit ${index + 1}`), { target: { value: digit } });
}

describe("SecurityPage", () => {
  it("keeps MFA off until a scanned enrollment is verified", async () => {
    beginMfa.mockResolvedValue({ secret: "JBSWY3DPEHPK3PXP", otpAuthUrl: "otpauth://totp/Terios:test?secret=JBSWY3DPEHPK3PXP" });
    confirmMfa.mockResolvedValue(undefined);
    render(<SecurityPage />);
    expect(screen.getByText("MFA is off")).toBeTruthy();
    expect(screen.queryByLabelText(/Authenticator code digit 1/)).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Enable MFA" }));
    expect(await screen.findByTitle("Terios MFA enrollment QR code")).toBeTruthy();
    enterCode("Authenticator code");
    fireEvent.click(screen.getByRole("button", { name: /Verify and enable/ }));
    await waitFor(() => expect(confirmMfa).toHaveBeenCalledWith("access", "123456"));
    expect(setMfaEnabled).toHaveBeenCalledWith(true);
    expect(await screen.findByText("MFA is enabled")).toBeTruthy();
  });

  it("requires a current segmented code to disable MFA and signs out afterward", async () => {
    authState.mfaEnabled = true; disableMfa.mockResolvedValue(undefined); logout.mockResolvedValue(undefined);
    render(<SecurityPage />);
    enterCode("Current code to disable MFA");
    fireEvent.click(screen.getByRole("button", { name: "Disable MFA" }));
    await waitFor(() => expect(disableMfa).toHaveBeenCalledWith("access", "123456"));
    expect(logout).toHaveBeenCalledWith("MFA was disabled. Sign in again to continue.");
  });
});
