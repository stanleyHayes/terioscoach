import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ForgotPasswordPage from "./page";

const forgotPassword = vi.hoisted(() => vi.fn());

vi.mock("@/lib/api", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/api")>();
  return { ...original, authApi: { ...original.authApi, forgotPassword } };
});

function submit(email: string) {
  fireEvent.change(screen.getByLabelText(/email/i), { target: { value: email } });
  fireEvent.click(screen.getByRole("button", { name: /send reset link/i }));
}

describe("client password recovery request", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    forgotPassword.mockResolvedValue(undefined);
  });

  it("sends the request and confirms without revealing whether the account exists", async () => {
    render(<ForgotPasswordPage />);
    submit("ama@example.com");

    await waitFor(() => expect(forgotPassword).toHaveBeenCalledWith("ama@example.com"));
    expect(await screen.findByText(/check your inbox/i)).toBeTruthy();
    // The same words either way. Anything else turns this form into a way
    // of asking whether someone is a client of this practice — which is
    // health information.
    expect(screen.getByText(/if an account uses that address/i)).toBeTruthy();
    expect(screen.getByText(/whether or not the address is registered/i)).toBeTruthy();
  });

  it("trims a pasted address", async () => {
    render(<ForgotPasswordPage />);
    submit("  ama@example.com  ");

    await waitFor(() => expect(forgotPassword).toHaveBeenCalledWith("ama@example.com"));
  });

  it("catches a malformed address before any round trip", async () => {
    render(<ForgotPasswordPage />);
    submit("ama@");

    expect((await screen.findByRole("alert")).textContent).toMatch(/valid email address/i);
    expect(forgotPassword).not.toHaveBeenCalled();
  });

  it("announces the error exactly once", async () => {
    render(<ForgotPasswordPage />);
    submit("nope");

    // The field renders its own alert; a second copy alongside it makes a
    // screen reader say the same sentence twice.
    await waitFor(() => expect(screen.getAllByRole("alert")).toHaveLength(1));
  });

  it("clears the error as soon as the address is edited", async () => {
    render(<ForgotPasswordPage />);
    submit("nope");
    await screen.findByRole("alert");

    fireEvent.change(screen.getByLabelText(/email/i), { target: { value: "ama@example.com" } });

    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("keeps the form usable when the request fails", async () => {
    forgotPassword.mockRejectedValue(new Error("network"));
    render(<ForgotPasswordPage />);
    submit("ama@example.com");

    expect((await screen.findByRole("alert")).textContent).toMatch(/couldn't send the recovery email/i);
    // Not the success screen: "check your inbox" after a failure leaves
    // someone waiting for mail that was never sent.
    expect(screen.queryByText(/check your inbox/i)).toBeNull();
    expect(screen.getByRole("button", { name: /send reset link/i })).toBeTruthy();
  });

  it("offers a way back to sign in from both states", async () => {
    render(<ForgotPasswordPage />);
    expect(screen.getByRole("link", { name: /back to sign in/i })).toBeTruthy();

    submit("ama@example.com");
    await screen.findByText(/check your inbox/i);

    expect(screen.getByRole("link", { name: /back to sign in/i })).toBeTruthy();
  });
});
