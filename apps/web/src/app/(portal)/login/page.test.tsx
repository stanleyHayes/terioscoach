import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import LoginPage from "./page";

const replaceMock = vi.fn();
const loginMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: replaceMock }),
  useSearchParams: () => searchParamsValue,
}));

vi.mock("@/lib/auth", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/auth")>();
  return {
    ...original,
    useAuth: () => ({
      status: "unauthenticated",
      user: null,
      accessToken: null,
      login: loginMock,
      register: vi.fn(),
      logout: vi.fn(),
    }),
  };
});

afterEach(() => {
  replaceMock.mockReset();
  loginMock.mockReset();
  searchParamsValue = new URLSearchParams();
});

describe("LoginPage", () => {
  it("renders the wordmark, fields, submit button, and register cross-link", () => {
    render(<LoginPage />);

    expect(screen.getByText("Terios Wellness")).toBeTruthy();
    expect(screen.getByLabelText("Email")).toBeTruthy();
    expect(screen.getByLabelText("Password")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Sign in" })).toBeTruthy();
    expect(
      screen.getByRole("link", { name: "Create an account" }).getAttribute("href"),
    ).toBe("/register");
  });

  it("shows custom validation messages instead of native bubbles", () => {
    render(<LoginPage />);

    fireEvent.submit(screen.getByRole("button", { name: "Sign in" }).closest("form")!);

    expect(screen.getByText("Enter your email")).toBeTruthy();
    expect(screen.getByText("Enter your password")).toBeTruthy();
    expect(loginMock).not.toHaveBeenCalled();
  });

  it("validates the email format with a branded message", () => {
    render(<LoginPage />);

    fireEvent.change(screen.getByLabelText("Email"), { target: { value: "not-an-email" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "secret" } });
    fireEvent.submit(screen.getByRole("button", { name: "Sign in" }).closest("form")!);

    expect(
      screen.getByText("Enter a valid email address, e.g. you@example.com"),
    ).toBeTruthy();
    expect(loginMock).not.toHaveBeenCalled();
  });

  it("signs in and redirects to the portal on success", async () => {
    loginMock.mockResolvedValueOnce(undefined);
    render(<LoginPage />);

    fireEvent.change(screen.getByLabelText("Email"), { target: { value: "a@b.com" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "secret" } });
    fireEvent.submit(screen.getByRole("button", { name: "Sign in" }).closest("form")!);

    await waitFor(() => expect(loginMock).toHaveBeenCalledWith("a@b.com", "secret"));
    await waitFor(() => expect(replaceMock).toHaveBeenCalledWith("/portal"));
  });

  it("shows the brand-voice banner for invalid credentials", async () => {
    const { ApiError } = await import("@/lib/api");
    loginMock.mockRejectedValueOnce(new ApiError(401, "invalid_credentials", "Nope"));
    render(<LoginPage />);

    fireEvent.change(screen.getByLabelText("Email"), { target: { value: "a@b.com" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "wrong" } });
    fireEvent.submit(screen.getByRole("button", { name: "Sign in" }).closest("form")!);

    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toContain(
        "That email and password don't match our records.",
      ),
    );
    expect(replaceMock).not.toHaveBeenCalled();
  });

  it("surfaces a network error with what-to-do-next copy", async () => {
    const { ApiError } = await import("@/lib/api");
    loginMock.mockRejectedValueOnce(
      new ApiError(0, "network_error", "We can't reach the server. Check your connection and try again."),
    );
    render(<LoginPage />);

    fireEvent.change(screen.getByLabelText("Email"), { target: { value: "a@b.com" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "secret" } });
    fireEvent.submit(screen.getByRole("button", { name: "Sign in" }).closest("form")!);

    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toContain(
        "We can't reach the server. Check your connection and try again.",
      ),
    );
  });

  it("honours a same-site ?next= after sign-in (booking-flow restore)", async () => {
    searchParamsValue = new URLSearchParams({
      next: "/portal/book?service=s1&slot=2026-08-20T09%3A30%3A00Z",
    });
    loginMock.mockResolvedValueOnce(undefined);
    render(<LoginPage />);

    fireEvent.change(screen.getByLabelText("Email"), { target: { value: "a@b.com" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "secret" } });
    fireEvent.submit(screen.getByRole("button", { name: "Sign in" }).closest("form")!);

    await waitFor(() =>
      expect(replaceMock).toHaveBeenCalledWith(
        "/portal/book?service=s1&slot=2026-08-20T09%3A30%3A00Z",
      ),
    );
  });

  it("carries ?next= through to the register cross-link", () => {
    searchParamsValue = new URLSearchParams({ next: "/portal/book?service=s1" });
    render(<LoginPage />);

    expect(
      screen.getByRole("link", { name: "Create an account" }).getAttribute("href"),
    ).toBe(`/register?next=${encodeURIComponent("/portal/book?service=s1")}`);
  });

  it("ignores an external ?next= and falls back to the portal", async () => {
    searchParamsValue = new URLSearchParams({ next: "https://evil.example" });
    loginMock.mockResolvedValueOnce(undefined);
    render(<LoginPage />);

    fireEvent.change(screen.getByLabelText("Email"), { target: { value: "a@b.com" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "secret" } });
    fireEvent.submit(screen.getByRole("button", { name: "Sign in" }).closest("form")!);

    await waitFor(() => expect(replaceMock).toHaveBeenCalledWith("/portal"));
  });
});
