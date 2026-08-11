import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import PortalLayout from "./layout";

const replaceMock = vi.fn();
const logoutMock = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: replaceMock }),
  usePathname: () => "/portal",
}));

let authState = {
  status: "loading",
  user: null as { id: string; email: string; role: string; name: string } | null,
};

vi.mock("@/lib/auth", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/auth")>();
  return {
    ...original,
    useAuth: () => ({
      ...authState,
      accessToken: authState.user ? "a1" : null,
      login: vi.fn(),
      register: vi.fn(),
      logout: logoutMock,
    }),
  };
});

afterEach(() => {
  replaceMock.mockReset();
  logoutMock.mockReset();
  authState = { status: "loading", user: null };
});

describe("PortalLayout guard", () => {
  it("shows a branded loading state while the session is restoring", () => {
    authState = { status: "loading", user: null };
    render(<PortalLayout>Secret content</PortalLayout>);

    expect(screen.getByText("Preparing your portal…")).toBeTruthy();
    expect(screen.queryByText("Secret content")).toBeNull();
    expect(replaceMock).not.toHaveBeenCalled();
  });

  it("redirects unauthenticated visitors to /login", async () => {
    authState = { status: "unauthenticated", user: null };
    render(<PortalLayout>Secret content</PortalLayout>);

    await waitFor(() => expect(replaceMock).toHaveBeenCalledWith("/login"));
    expect(screen.queryByText("Secret content")).toBeNull();
  });

  it("renders the shell and children for authenticated clients", () => {
    authState = {
      status: "authenticated",
      user: { id: "u1", email: "ama@example.com", role: "client", name: "Ama Serwaa" },
    };
    render(<PortalLayout>Secret content</PortalLayout>);

    expect(screen.getByText("Secret content")).toBeTruthy();
    expect(screen.getByRole("navigation", { name: "Portal" })).toBeTruthy();
    expect(replaceMock).not.toHaveBeenCalled();
  });

  it("signs out via the user menu and returns to /login", async () => {
    logoutMock.mockResolvedValueOnce(undefined);
    authState = {
      status: "authenticated",
      user: { id: "u1", email: "ama@example.com", role: "client", name: "Ama Serwaa" },
    };
    render(<PortalLayout>Secret content</PortalLayout>);

    fireEvent.click(screen.getByRole("button", { name: /Ama Serwaa/ }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Sign out" }));

    await waitFor(() => expect(logoutMock).toHaveBeenCalled());
    await waitFor(() => expect(replaceMock).toHaveBeenCalledWith("/login"));
  });
});
