import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ForgotPasswordPage from "./page";

const forgotPassword = vi.hoisted(() => vi.fn());

vi.mock("@/lib/api", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/api")>();
  return { ...original, authApi: { ...original.authApi, forgotPassword } };
});

function submit(email: string) {
  fireEvent.change(screen.getByLabelText(/practitioner email/i), { target: { value: email } });
  fireEvent.click(screen.getByRole("button", { name: /send recovery link/i }));
}

describe("practitioner password recovery request", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    forgotPassword.mockResolvedValue(undefined);
  });

  it("sends the recovery request and confirms without naming the account", async () => {
    render(<ForgotPasswordPage />);
    submit("practice@example.com");

    await waitFor(() => expect(forgotPassword).toHaveBeenCalledWith("practice@example.com"));
    expect(await screen.findByText(/check your inbox/i)).toBeTruthy();
    // "If that address belongs to the practice" — the same words whether or
    // not the account exists. Anything else turns this screen into an
    // address oracle for the one account that matters most.
    expect(screen.getByText(/if that address belongs to the practice/i)).toBeTruthy();
    expect(screen.getByText(/same message for every address/i)).toBeTruthy();
  });

  it("trims the address before sending it", async () => {
    render(<ForgotPasswordPage />);
    submit("  practice@example.com  ");

    // A copied-and-pasted address usually arrives with whitespace, and the
    // server compares exactly.
    await waitFor(() => expect(forgotPassword).toHaveBeenCalledWith("practice@example.com"));
  });

  it("catches a malformed address before any round trip", async () => {
    render(<ForgotPasswordPage />);
    submit("not-an-address");

    expect((await screen.findByRole("alert")).textContent).toMatch(/valid email address/i);
    expect(forgotPassword).not.toHaveBeenCalled();
  });

  it("clears the error as soon as the practitioner starts fixing it", async () => {
    render(<ForgotPasswordPage />);
    submit("nope");
    await screen.findByRole("alert");

    fireEvent.change(screen.getByLabelText(/practitioner email/i), {
      target: { value: "practice@example.com" },
    });

    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("keeps the form usable when the request fails", async () => {
    forgotPassword.mockRejectedValue(new Error("network"));
    render(<ForgotPasswordPage />);
    submit("practice@example.com");

    expect((await screen.findByRole("alert")).textContent).toMatch(/couldn't send the recovery email/i);
    // Not the success screen: showing "check your inbox" after a failure
    // leaves the practitioner waiting for mail that was never sent.
    expect(screen.queryByText(/check your inbox/i)).toBeNull();
    expect(screen.getByRole("button", { name: /send recovery link/i })).toBeTruthy();
  });

  it("offers a way back to sign in from both states", async () => {
    render(<ForgotPasswordPage />);
    expect(screen.getByRole("link", { name: /back to sign in/i })).toBeTruthy();

    submit("practice@example.com");
    await screen.findByText(/check your inbox/i);

    expect(screen.getByRole("link", { name: /back to sign in/i })).toBeTruthy();
  });
});
