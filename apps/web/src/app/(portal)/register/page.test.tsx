import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import RegisterPage from "./page";

const replaceMock = vi.fn();
const registerMock = vi.fn();
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
      login: vi.fn(),
      register: registerMock,
      logout: vi.fn(),
    }),
  };
});

afterEach(() => {
  replaceMock.mockReset();
  registerMock.mockReset();
  searchParamsValue = new URLSearchParams();
});

function fillValidForm() {
  fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Ama Serwaa" } });
  fireEvent.change(screen.getByLabelText("Email"), { target: { value: "ama@example.com" } });
  fireEvent.change(screen.getByLabelText("Password"), {
    target: { value: "twelve-chars-1" },
  });
}

describe("RegisterPage", () => {
  it("renders the wordmark, fields, submit button, and login cross-link", () => {
    render(<RegisterPage />);

    expect(screen.getByText("Terios Wellness")).toBeTruthy();
    expect(screen.getByLabelText("Name")).toBeTruthy();
    expect(screen.getByLabelText("Email")).toBeTruthy();
    expect(screen.getByLabelText("Password")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Create account" })).toBeTruthy();
    expect(screen.getByRole("link", { name: "Sign in" }).getAttribute("href")).toBe("/login");
  });

  it("shows custom validation messages instead of native bubbles", () => {
    render(<RegisterPage />);

    fireEvent.submit(
      screen.getByRole("button", { name: "Create account" }).closest("form")!,
    );

    expect(screen.getByText("Enter your name")).toBeTruthy();
    expect(screen.getByText("Enter your email")).toBeTruthy();
    expect(screen.getByText("Choose a password")).toBeTruthy();
    expect(registerMock).not.toHaveBeenCalled();
  });

  it("enforces the 12-character password rule with a branded message", () => {
    render(<RegisterPage />);

    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Ama Serwaa" } });
    fireEvent.change(screen.getByLabelText("Email"), { target: { value: "ama@example.com" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "short" } });
    fireEvent.submit(
      screen.getByRole("button", { name: "Create account" }).closest("form")!,
    );

    expect(screen.getByText("Use at least 12 characters for your password")).toBeTruthy();
    expect(registerMock).not.toHaveBeenCalled();
  });

  it("registers and redirects to the portal on success", async () => {
    registerMock.mockResolvedValueOnce(undefined);
    render(<RegisterPage />);

    fillValidForm();
    fireEvent.submit(
      screen.getByRole("button", { name: "Create account" }).closest("form")!,
    );

    await waitFor(() =>
      expect(registerMock).toHaveBeenCalledWith(
        "Ama Serwaa",
        "ama@example.com",
        "twelve-chars-1",
      ),
    );
    await waitFor(() => expect(replaceMock).toHaveBeenCalledWith("/portal"));
  });

  it("shows the brand-voice banner when the email is already taken", async () => {
    const { ApiError } = await import("@/lib/api");
    registerMock.mockRejectedValueOnce(new ApiError(409, "email_taken", "Taken"));
    render(<RegisterPage />);

    fillValidForm();
    fireEvent.submit(
      screen.getByRole("button", { name: "Create account" }).closest("form")!,
    );

    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toContain(
        "An account already exists for this email. Try signing in instead.",
      ),
    );
    expect(replaceMock).not.toHaveBeenCalled();
  });

  it("honours a same-site ?next= after registering (booking-flow restore)", async () => {
    searchParamsValue = new URLSearchParams({ next: "/portal/book?service=s1" });
    registerMock.mockResolvedValueOnce(undefined);
    render(<RegisterPage />);

    fillValidForm();
    fireEvent.submit(
      screen.getByRole("button", { name: "Create account" }).closest("form")!,
    );

    await waitFor(() => expect(replaceMock).toHaveBeenCalledWith("/portal/book?service=s1"));
  });

  it("carries ?next= through to the sign-in cross-link", () => {
    searchParamsValue = new URLSearchParams({ next: "/portal/book?service=s1" });
    render(<RegisterPage />);

    expect(screen.getByRole("link", { name: "Sign in" }).getAttribute("href")).toBe(
      `/login?next=${encodeURIComponent("/portal/book?service=s1")}`,
    );
  });
});
