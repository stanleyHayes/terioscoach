import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import LoginPage from "./page";

const replaceMock = vi.fn();
const loginMock = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: replaceMock }),
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
      logout: vi.fn(),
    }),
  };
});

afterEach(() => {
  replaceMock.mockReset();
  loginMock.mockReset();
});

describe("LoginPage", () => {
  it("renders the wordmark, email and password fields, and the submit button", () => {
    render(<LoginPage />);

    expect(screen.getByText("Terios")).toBeTruthy();
    expect(screen.getByLabelText("Email")).toBeTruthy();
    expect(screen.getByLabelText("Password")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Sign in" })).toBeTruthy();
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

  it("toggles password visibility with the Eye button", () => {
    render(<LoginPage />);

    const password = screen.getByLabelText("Password");
    expect(password.getAttribute("type")).toBe("password");

    fireEvent.click(screen.getByRole("button", { name: "Show password" }));
    expect(password.getAttribute("type")).toBe("text");

    fireEvent.click(screen.getByRole("button", { name: "Hide password" }));
    expect(password.getAttribute("type")).toBe("password");
  });

  it("signs in and navigates to the dashboard on success", async () => {
    loginMock.mockResolvedValueOnce(undefined);
    render(<LoginPage />);

    fireEvent.change(screen.getByLabelText("Email"), { target: { value: "a@b.com" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "secret" } });
    fireEvent.submit(screen.getByRole("button", { name: "Sign in" }).closest("form")!);

    await waitFor(() => expect(loginMock).toHaveBeenCalledWith("a@b.com", "secret"));
    await waitFor(() => expect(replaceMock).toHaveBeenCalledWith("/"));
  });

  it("shows a branded error banner for invalid credentials", async () => {
    const { ApiError } = await import("@/lib/api");
    loginMock.mockRejectedValueOnce(new ApiError(401, "invalid_credentials", "Nope"));
    render(<LoginPage />);

    fireEvent.change(screen.getByLabelText("Email"), { target: { value: "a@b.com" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "wrong" } });
    fireEvent.submit(screen.getByRole("button", { name: "Sign in" }).closest("form")!);

    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toContain(
        "That email and password don't match. Try again.",
      ),
    );
    expect(replaceMock).not.toHaveBeenCalled();
  });

  it("asks for six separate authenticator digits only after the API requires MFA", async () => {
    const { ApiError } = await import("@/lib/api");
    loginMock.mockRejectedValueOnce(new ApiError(401, "mfa_required", "Code required"));
    loginMock.mockResolvedValueOnce(undefined);
    render(<LoginPage />);
    fireEvent.change(screen.getByLabelText("Email"), { target: { value: "a@b.com" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "secret" } });
    fireEvent.submit(screen.getByRole("button", { name: "Sign in" }).closest("form")!);

    await waitFor(() => expect(screen.getAllByLabelText(/Authenticator code digit/)).toHaveLength(6));
    expect(screen.queryByLabelText("Email")).toBeNull();
    for (const [index, digit] of [..."123456"].entries()) {
      fireEvent.change(screen.getByLabelText(`Authenticator code digit ${index + 1}`), { target: { value: digit } });
    }
    fireEvent.submit(screen.getByRole("button", { name: "Verify and sign in" }).closest("form")!);
    await waitFor(() => expect(loginMock).toHaveBeenLastCalledWith("a@b.com", "secret", "123456"));
    await waitFor(() => expect(replaceMock).toHaveBeenCalledWith("/"));
  });

  it("surfaces the practitioner-role rejection from the auth layer", async () => {
    const { ApiError } = await import("@/lib/api");
    const { NOT_PRACTITIONER_MESSAGE } = await import("@/lib/auth");
    loginMock.mockRejectedValueOnce(
      new ApiError(403, "not_a_practitioner", NOT_PRACTITIONER_MESSAGE),
    );
    render(<LoginPage />);

    fireEvent.change(screen.getByLabelText("Email"), { target: { value: "a@b.com" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "secret" } });
    fireEvent.submit(screen.getByRole("button", { name: "Sign in" }).closest("form")!);

    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toContain(NOT_PRACTITIONER_MESSAGE),
    );
  });
});
