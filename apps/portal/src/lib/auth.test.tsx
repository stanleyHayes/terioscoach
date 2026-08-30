import { act, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AuthProvider, REFRESH_TOKEN_KEY, useAuth } from "./auth";
import { API_BASE_URL } from "./api";

function jsonResponse(status: number, body?: unknown): Response {
  return new Response(body === undefined ? null : JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const clientUser = {
  id: "u1",
  email: "ama@example.com",
  role: "client",
  name: "Ama Serwaa",
};

const tokens = {
  accessToken: "a1",
  accessTokenExpiresAt: "2026-08-11T17:00:00Z",
  refreshToken: "r1",
};

const fetchMock = vi.fn();

/** Probe component exposing the auth context to assertions. */
function Probe() {
  const auth = useAuth();
  return (
    <div>
      <span data-testid="status">{auth.status}</span>
      <span data-testid="user">{auth.user?.name ?? "none"}</span>
      <span data-testid="role">{auth.user?.role ?? "none"}</span>
      <span data-testid="access">{auth.accessToken ?? "none"}</span>
      <button onClick={() => auth.login("ama@example.com", "secret-secret")}>login</button>
      <button onClick={() => auth.register("Ama Serwaa", "ama@example.com", "secret-secret")}>
        register
      </button>
      <button onClick={() => auth.logout()}>logout</button>
    </div>
  );
}

function renderProvider() {
  return render(
    <AuthProvider>
      <Probe />
    </AuthProvider>,
  );
}

beforeEach(() => {
  vi.stubGlobal("fetch", fetchMock);
  localStorage.clear();
});

afterEach(() => {
  fetchMock.mockReset();
});

describe("AuthProvider", () => {
  it("starts unauthenticated when no refresh token is stored", async () => {
    renderProvider();

    await waitFor(() =>
      expect(screen.getByTestId("status").textContent).toBe("unauthenticated"),
    );
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("login stores the refresh token and exposes the client user", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(200, { ...tokens, user: clientUser }));
    renderProvider();
    await waitFor(() =>
      expect(screen.getByTestId("status").textContent).toBe("unauthenticated"),
    );

    await act(async () => {
      screen.getByText("login").click();
    });

    expect(screen.getByTestId("status").textContent).toBe("authenticated");
    expect(screen.getByTestId("user").textContent).toBe("Ama Serwaa");
    expect(screen.getByTestId("role").textContent).toBe("client");
    expect(screen.getByTestId("access").textContent).toBe("a1");
    expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBe("r1");
  });

  it("register signs the new client in immediately (201 returns tokens)", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(201, { ...tokens, user: clientUser }));
    renderProvider();
    await waitFor(() =>
      expect(screen.getByTestId("status").textContent).toBe("unauthenticated"),
    );

    await act(async () => {
      screen.getByText("register").click();
    });

    expect(fetchMock).toHaveBeenCalledWith(`${API_BASE_URL}/v1/auth/register`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        email: "ama@example.com",
        password: "secret-secret",
        name: "Ama Serwaa",
      }),
    });
    expect(screen.getByTestId("status").textContent).toBe("authenticated");
    expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBe("r1");
  });

  it("propagates ApiError from a failed login and stores nothing", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(401, { error: { code: "invalid_credentials", message: "Nope" } }),
    );

    let thrown: unknown;
    function ThrowingProbe() {
      const auth = useAuth();
      return (
        <button
          onClick={() =>
            auth.login("ama@example.com", "wrong").catch((e) => {
              thrown = e;
            })
          }
        >
          login
        </button>
      );
    }
    render(
      <AuthProvider>
        <ThrowingProbe />
      </AuthProvider>,
    );
    await waitFor(() => expect(fetchMock).not.toHaveBeenCalled());

    await act(async () => {
      screen.getByText("login").click();
    });

    expect(thrown).toMatchObject({ code: "invalid_credentials", status: 401 });
    expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBeNull();
  });

  it("logout clears the session and calls the logout endpoint", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(200, { ...tokens, user: clientUser }));
    renderProvider();
    await waitFor(() =>
      expect(screen.getByTestId("status").textContent).toBe("unauthenticated"),
    );
    await act(async () => {
      screen.getByText("login").click();
    });
    expect(screen.getByTestId("status").textContent).toBe("authenticated");

    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));
    await act(async () => {
      screen.getByText("logout").click();
    });

    expect(screen.getByTestId("status").textContent).toBe("unauthenticated");
    expect(screen.getByTestId("user").textContent).toBe("none");
    expect(screen.getByTestId("access").textContent).toBe("none");
    expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBeNull();
    expect(fetchMock).toHaveBeenLastCalledWith(`${API_BASE_URL}/v1/auth/logout`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refreshToken: "r1" }),
    });
  });

  it("restores a session from a stored refresh token via refresh + me", async () => {
    localStorage.setItem(REFRESH_TOKEN_KEY, "stored-refresh");
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse(200, {
          accessToken: "a2",
          accessTokenExpiresAt: "2026-08-11T18:00:00Z",
          refreshToken: "r2",
          user: clientUser,
        }),
      )
      .mockResolvedValueOnce(jsonResponse(200, { user: clientUser }));

    renderProvider();

    await waitFor(() =>
      expect(screen.getByTestId("status").textContent).toBe("authenticated"),
    );
    expect(screen.getByTestId("user").textContent).toBe("Ama Serwaa");
    expect(screen.getByTestId("access").textContent).toBe("a2");
    // rotation: the new refresh token replaces the stored one
    expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBe("r2");
    // /me was called with the rotated access token
    expect(fetchMock).toHaveBeenLastCalledWith(`${API_BASE_URL}/v1/auth/me`, {
      method: "GET",
      headers: { Authorization: "Bearer a2" },
    });
  });

  it("drops the session when the stored refresh token is rejected", async () => {
    localStorage.setItem(REFRESH_TOKEN_KEY, "stale");
    fetchMock.mockResolvedValueOnce(
      jsonResponse(401, { error: { code: "token_expired", message: "expired" } }),
    );

    renderProvider();

    await waitFor(() =>
      expect(screen.getByTestId("status").textContent).toBe("unauthenticated"),
    );
    expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBeNull();
  });
});
