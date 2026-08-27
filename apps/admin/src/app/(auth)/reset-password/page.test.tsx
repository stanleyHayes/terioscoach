import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import ResetPasswordPage from "./page";

const resetPassword = vi.hoisted(() => vi.fn());
const searchParams = vi.hoisted(() => ({ current: new URLSearchParams("token=recovery-token") }));

vi.mock("next/navigation", () => ({
  useSearchParams: () => searchParams.current,
}));

vi.mock("@/lib/api", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/api")>();
  return { ...original, authApi: { ...original.authApi, resetPassword } };
});

function fill(password: string, confirm = password) {
  fireEvent.change(screen.getByLabelText(/^new password/i), { target: { value: password } });
  fireEvent.change(screen.getByLabelText(/confirm new password/i), { target: { value: confirm } });
  fireEvent.click(screen.getByRole("button", { name: /update password/i }));
}

describe("practitioner password reset", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    searchParams.current = new URLSearchParams("token=recovery-token");
    resetPassword.mockResolvedValue(undefined);
  });

  it("sends the token from the link with the new password", async () => {
    render(<ResetPasswordPage />);
    fill("a sufficiently long password");

    await waitFor(() =>
      expect(resetPassword).toHaveBeenCalledWith("recovery-token", "a sufficiently long password"),
    );
    expect(await screen.findByText(/access restored/i)).toBeTruthy();
    // The server revokes every other session on reset; saying so is what
    // tells a practitioner whose account was taken that they have it back.
    expect(screen.getByText(/every previous practice session has been signed out/i)).toBeTruthy();
  });

  it("refuses a password shorter than the policy before any round trip", async () => {
    render(<ResetPasswordPage />);
    fill("short");

    expect((await screen.findByRole("alert")).textContent).toMatch(/at least 12 characters/i);
    expect(resetPassword).not.toHaveBeenCalled();
  });

  it("refuses a mismatched confirmation", async () => {
    render(<ResetPasswordPage />);
    fill("a sufficiently long password", "a different long password");

    // Locking someone out of their own practice with a typo they cannot see
    // is the failure this prevents.
    expect((await screen.findByRole("alert")).textContent).toMatch(/do not match/i);
    expect(resetPassword).not.toHaveBeenCalled();
  });

  it("says so when the link arrived without a token", async () => {
    searchParams.current = new URLSearchParams();
    render(<ResetPasswordPage />);
    fill("a sufficiently long password");

    expect((await screen.findByRole("alert")).textContent).toMatch(/incomplete/i);
    expect(resetPassword).not.toHaveBeenCalled();
  });

  it("distinguishes an expired link from a server problem", async () => {
    resetPassword.mockRejectedValue(
      new ApiError(400, "password_reset_invalid", "token is invalid or expired"),
    );
    render(<ResetPasswordPage />);
    fill("a sufficiently long password");

    // Two different fixes: request a new link, versus try again in a
    // moment. Telling a practitioner the wrong one wastes their evening.
    expect((await screen.findByRole("alert")).textContent).toMatch(/invalid or has expired/i);
    expect(screen.getByRole("link", { name: /request a new link/i })).toBeTruthy();
  });

  it("falls back to a retry message for any other failure", async () => {
    resetPassword.mockRejectedValue(new Error("network"));
    render(<ResetPasswordPage />);
    fill("a sufficiently long password");

    expect((await screen.findByRole("alert")).textContent).toMatch(/couldn't update your password/i);
    expect(screen.queryByText(/access restored/i)).toBeNull();
  });

  it("clears the error as soon as either field is edited", async () => {
    render(<ResetPasswordPage />);
    fill("short");
    await screen.findByRole("alert");

    fireEvent.change(screen.getByLabelText(/^new password/i), { target: { value: "longer now" } });
    expect(screen.queryByRole("alert")).toBeNull();

    fill("short again", "mismatch");
    await screen.findByRole("alert");
    fireEvent.change(screen.getByLabelText(/confirm new password/i), { target: { value: "x" } });
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("sends the practitioner to sign in once the password is changed", async () => {
    render(<ResetPasswordPage />);
    fill("a sufficiently long password");

    expect(await screen.findByRole("link", { name: /return to practice sign in/i })).toBeTruthy();
    // The form is gone: re-submitting a spent one-time token only produces
    // an error.
    expect(screen.queryByRole("button", { name: /update password/i })).toBeNull();
  });
});
