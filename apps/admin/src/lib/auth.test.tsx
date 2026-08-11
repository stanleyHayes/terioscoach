import { act, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  AuthProvider,
  NOT_PRACTITIONER_MESSAGE,
  REFRESH_TOKEN_KEY,
  useAuth,
} from "./auth";
import { API_BASE_URL } from "./api";

function jsonResponse(status: number, body?: unknown): Response {
  return new Response(body === undefined ? null : JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const practitioner = {
  id: "u1",
  email: "akosua@terios.com",
  role: "practitioner",
  name: "Akosua Mensah",
};

const fetchMock = vi.fn();

/** Probe component exposing the auth context to assertions. */
function Probe() {
  const auth = useAuth();
  return (
    <div>
      <span data-testid="status">{auth.status}</span>
      <span data-testid="user">{auth.user?.name ?? "none"}</span>
      <button onClick={() => auth.login("akosua@terios.com", "secret")}>login</button>
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
  sessionStorage.clear();
});

afterEach(() => {
  // note: no vi.unstubAllGlobals() — it would undo the storage stubs from vitest.setup.ts
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

  it("login stores the refresh token and exposes the practitioner user", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(200, { accessToken: "a1", refreshToken: "r1", user: practitioner }),
    );
    renderProvider();
    await waitFor(() =>
      expect(screen.getByTestId("status").textContent).toBe("unauthenticated"),
    );

    await act(async () => {
      screen.getByText("login").click();
    });

    expect(screen.getByTestId("status").textContent).toBe("authenticated");
    expect(screen.getByTestId("user").textContent).toBe("Akosua Mensah");
    expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBe("r1");
  });

  it("rejects non-practitioner logins, invalidates the token, and stores nothing", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse(200, {
          accessToken: "a1",
          refreshToken: "r1",
          user: { ...practitioner, role: "client" },
        }),
      )
      .mockResolvedValueOnce(new Response(null, { status: 204 })); // best-effort logout

    let thrown: unknown;
    function ThrowingProbe() {
      const auth = useAuth();
      return (
        <button
          onClick={() =>
            auth.login("client@example.com", "secret").catch((e) => {
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

    expect(thrown).toMatchObject({ code: "not_a_practitioner" });
    expect((thrown as Error).message).toBe(NOT_PRACTITIONER_MESSAGE);
    expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBeNull();
    // the issued refresh token was invalidated server-side
    expect(fetchMock).toHaveBeenLastCalledWith(`${API_BASE_URL}/v1/auth/logout`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refreshToken: "r1" }),
    });
  });

  it("logout clears the session and calls the logout endpoint", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(200, { accessToken: "a1", refreshToken: "r1", user: practitioner }),
    );
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
        jsonResponse(200, { accessToken: "a2", refreshToken: "r2" }),
      )
      .mockResolvedValueOnce(jsonResponse(200, { user: practitioner }));

    renderProvider();

    await waitFor(() =>
      expect(screen.getByTestId("status").textContent).toBe("authenticated"),
    );
    expect(screen.getByTestId("user").textContent).toBe("Akosua Mensah");
    expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBe("r2");
  });

  it("drops the session when the stored refresh token is rejected", async () => {
    localStorage.setItem(REFRESH_TOKEN_KEY, "stale");
    fetchMock.mockResolvedValueOnce(
      jsonResponse(401, { error: { code: "invalid_token", message: "expired" } }),
    );

    renderProvider();

    await waitFor(() =>
      expect(screen.getByTestId("status").textContent).toBe("unauthenticated"),
    );
    expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBeNull();
  });
});
