import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  ApiError,
  API_BASE_URL,
  authApi,
  authedRequest,
  listServices,
  resetTokenRotation,
  SessionExpiredError,
} from "./api";

function jsonResponse(status: number, body?: unknown): Response {
  return new Response(body === undefined ? null : JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const fetchMock = vi.fn();

const clientUser = { id: "u1", email: "ama@example.com", role: "client", name: "Ama" };

beforeEach(() => {
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  fetchMock.mockReset();
});

describe("api client", () => {
  it("requests and completes password recovery", async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));
    await expect(authApi.forgotPassword("ama@example.com")).resolves.toBeUndefined();
    expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body))).toEqual({ email: "ama@example.com" });

    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));
    await expect(authApi.resetPassword("reset-token", "a new secure password")).resolves.toBeUndefined();
    expect(JSON.parse(String(fetchMock.mock.calls[1]?.[1]?.body))).toEqual({ token: "reset-token", password: "a new secure password" });
  });

  it("posts JSON to the register endpoint and parses the 201 auth response", async () => {
    const payload = {
      accessToken: "a1",
      accessTokenExpiresAt: "2026-08-11T17:00:00Z",
      refreshToken: "r1",
      user: clientUser,
    };
    fetchMock.mockResolvedValueOnce(jsonResponse(201, payload));

    const result = await authApi.register("Ama", "ama@example.com", "secret-secret");

    expect(fetchMock).toHaveBeenCalledWith(`${API_BASE_URL}/v1/auth/register`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        email: "ama@example.com",
        password: "secret-secret",
        name: "Ama",
      }),
    });
    expect(result).toEqual(payload);
  });

  it("posts JSON to the login endpoint and parses the auth response", async () => {
    const payload = {
      accessToken: "a1",
      accessTokenExpiresAt: "2026-08-11T17:00:00Z",
      refreshToken: "r1",
      user: clientUser,
    };
    fetchMock.mockResolvedValueOnce(jsonResponse(200, payload));

    const result = await authApi.login("ama@example.com", "secret-secret");

    expect(fetchMock).toHaveBeenCalledWith(`${API_BASE_URL}/v1/auth/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email: "ama@example.com", password: "secret-secret" }),
    });
    expect(result).toEqual(payload);
  });

  it("maps the {error:{code,message}} shape to a typed ApiError", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(401, { error: { code: "invalid_credentials", message: "Nope" } }),
    );

    const error = await authApi.login("ama@example.com", "wrong").catch((e) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect(error.status).toBe(401);
    expect(error.code).toBe("invalid_credentials");
    expect(error.message).toBe("Nope");
  });

  it("maps email_taken conflicts on register", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(409, { error: { code: "email_taken", message: "Taken" } }),
    );

    const error = await authApi
      .register("Ama", "ama@example.com", "secret-secret")
      .catch((e) => e);

    expect(error).toMatchObject({ status: 409, code: "email_taken" });
  });

  it("falls back to unknown_error when the error body is not the contract shape", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(500, { oops: true }));

    const error = await authApi.me("token").catch((e) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect(error.status).toBe(500);
    expect(error.code).toBe("unknown_error");
  });

  it("maps network failures to a network_error ApiError", async () => {
    fetchMock.mockRejectedValueOnce(new TypeError("fetch failed"));

    const error = await authApi.login("ama@example.com", "secret-secret").catch((e) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect(error.status).toBe(0);
    expect(error.code).toBe("network_error");
  });

  it("sends the bearer token on /v1/auth/me", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(200, { user: clientUser }));

    await authApi.me("access-123");

    expect(fetchMock).toHaveBeenCalledWith(`${API_BASE_URL}/v1/auth/me`, {
      method: "GET",
      headers: { Authorization: "Bearer access-123" },
    });
  });

  it("posts the refresh token and parses rotated tokens with user", async () => {
    const payload = {
      accessToken: "a2",
      accessTokenExpiresAt: "2026-08-11T18:00:00Z",
      refreshToken: "r2",
      user: clientUser,
    };
    fetchMock.mockResolvedValueOnce(jsonResponse(200, payload));

    const result = await authApi.refresh("r1");

    expect(fetchMock).toHaveBeenCalledWith(`${API_BASE_URL}/v1/auth/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refreshToken: "r1" }),
    });
    expect(result).toEqual(payload);
  });

  it("returns undefined for 204 No Content (logout)", async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));

    await expect(authApi.logout("r1")).resolves.toBeUndefined();
  });
});

describe("listServices", () => {
  const items = [
    {
      id: "s1",
      name: "Wellness coaching",
      description: "Ongoing one-on-one coaching.",
      durationMinutes: 45,
      priceKobo: 45000,
      currency: "GHS",
      sortOrder: 1,
    },
  ];

  it("fetches the public catalog with cache: no-store and returns items", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(200, { items }));

    const result = await listServices();

    expect(fetchMock).toHaveBeenCalledWith(`${API_BASE_URL}/v1/services`, {
      method: "GET",
      headers: {},
      cache: "no-store",
    });
    expect(result).toEqual(items);
  });

  it("propagates ApiError for non-2xx responses (e.g. 503 service_unavailable)", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(503, { error: { code: "service_unavailable", message: "Down" } }),
    );

    const error = await listServices().catch((e) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect(error.status).toBe(503);
    expect(error.code).toBe("service_unavailable");
  });
});

describe("authedRequest 401 recovery", () => {
  const session = {
    accessToken: "old-access",
    accessTokenExpiresAt: "2026-08-11T17:00:00Z",
    refreshToken: "refresh-1",
  };
  const rotated = {
    accessToken: "new-access",
    accessTokenExpiresAt: "2026-08-11T18:00:00Z",
    refreshToken: "refresh-2",
    user: clientUser,
  };

  // The rotation state is shared across calls by design; each test starts
  // from a clean one, exactly as a fresh page load would.
  beforeEach(() => {
    resetTokenRotation(null);
  });

  it("refreshes once after a token_expired 401, retries once, and reports the rotated tokens", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse(401, { error: { code: "token_expired", message: "expired" } }),
      )
      .mockResolvedValueOnce(jsonResponse(200, rotated))
      .mockResolvedValueOnce(jsonResponse(200, { user: clientUser }));

    const onTokensRefreshed = vi.fn();
    const result = await authedRequest<{ user: { id: string } }>(
      "/v1/auth/me",
      session,
      { onTokensRefreshed },
    );

    expect(result.user.id).toBe("u1");
    expect(onTokensRefreshed).toHaveBeenCalledWith({
      accessToken: "new-access",
      accessTokenExpiresAt: "2026-08-11T18:00:00Z",
      refreshToken: "refresh-2",
    });
    // retry uses the rotated access token
    expect(fetchMock).toHaveBeenLastCalledWith(`${API_BASE_URL}/v1/auth/me`, {
      method: "GET",
      headers: { Authorization: "Bearer new-access" },
    });
  });

  it("throws SessionExpiredError when the refresh itself is rejected with 401", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse(401, { error: { code: "token_expired", message: "expired" } }),
      )
      .mockResolvedValueOnce(
        jsonResponse(401, { error: { code: "token_invalid", message: "bad refresh" } }),
      );

    await expect(
      authedRequest("/v1/auth/me", session, { onTokensRefreshed: vi.fn() }),
    ).rejects.toBeInstanceOf(SessionExpiredError);
    // no retry beyond the single refresh attempt
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("throws SessionExpiredError when the retry after refresh is still 401", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse(401, { error: { code: "token_invalid", message: "nope" } }),
      )
      .mockResolvedValueOnce(jsonResponse(200, rotated))
      .mockResolvedValueOnce(
        jsonResponse(401, { error: { code: "unauthorized", message: "still no" } }),
      );

    await expect(
      authedRequest("/v1/auth/me", session, { onTokensRefreshed: vi.fn() }),
    ).rejects.toBeInstanceOf(SessionExpiredError);
    expect(fetchMock).toHaveBeenCalledTimes(3);
  });

  it("does not refresh for non-401 errors", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(500, { error: { code: "server_error", message: "boom" } }),
    );

    await expect(
      authedRequest("/v1/auth/me", session, { onTokensRefreshed: vi.fn() }),
    ).rejects.toMatchObject({ status: 500, code: "server_error" });
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  // A refresh token is one-time and the API revokes the whole session family
  // when a spent one comes back. Every portal screen mounts more than one
  // authenticated read, so simultaneous expiry is the normal case, not a rare
  // one — three reads must still cost exactly one rotation.
  it("rotates once for simultaneous 401s and retries all of them", async () => {
    fetchMock.mockImplementation((url: string, init?: RequestInit) => {
      if (String(url).endsWith("/v1/auth/refresh")) {
        expect(JSON.parse(String(init?.body))).toEqual({ refreshToken: "refresh-1" });
        return Promise.resolve(jsonResponse(200, rotated));
      }
      const authorization = (init?.headers as Record<string, string> | undefined)?.Authorization;
      return Promise.resolve(
        authorization === "Bearer new-access"
          ? jsonResponse(200, { ok: String(url) })
          : jsonResponse(401, { error: { code: "token_expired", message: "expired" } }),
      );
    });

    const onTokensRefreshed = vi.fn();
    const results = await Promise.all([
      authedRequest<{ ok: string }>("/v1/bookings/me", session, { onTokensRefreshed }),
      authedRequest<{ ok: string }>("/v1/forms/me", session, { onTokensRefreshed }),
      authedRequest<{ ok: string }>("/v1/documents/me", session, { onTokensRefreshed }),
    ]);

    expect(results.map((r) => r.ok)).toEqual([
      `${API_BASE_URL}/v1/bookings/me`,
      `${API_BASE_URL}/v1/forms/me`,
      `${API_BASE_URL}/v1/documents/me`,
    ]);
    const refreshCalls = fetchMock.mock.calls.filter(([url]) =>
      String(url).endsWith("/v1/auth/refresh"),
    );
    expect(refreshCalls).toHaveLength(1);
    expect(onTokensRefreshed).toHaveBeenCalledTimes(1);
  });

  // A component holding a snapshot from before the rotation must not present
  // the spent token afterwards: that is the replay the server revokes for.
  it("hands the current tokens to a caller whose session has already rotated", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse(401, { error: { code: "token_expired", message: "expired" } }),
      )
      .mockResolvedValueOnce(jsonResponse(200, rotated))
      .mockResolvedValueOnce(jsonResponse(200, { first: true }))
      .mockResolvedValueOnce(
        jsonResponse(401, { error: { code: "token_expired", message: "expired" } }),
      )
      .mockResolvedValueOnce(jsonResponse(200, { second: true }));

    const onTokensRefreshed = vi.fn();
    await authedRequest("/v1/bookings/me", session, { onTokensRefreshed });
    // Same stale `session` object a mounted component would still be holding.
    await authedRequest("/v1/forms/me", session, { onTokensRefreshed });

    const refreshCalls = fetchMock.mock.calls.filter(([url]) =>
      String(url).endsWith("/v1/auth/refresh"),
    );
    expect(refreshCalls).toHaveLength(1);
    expect(fetchMock).toHaveBeenLastCalledWith(`${API_BASE_URL}/v1/forms/me`, {
      method: "GET",
      headers: { Authorization: "Bearer new-access" },
    });
  });

  // Sign out, sign in as someone else: the rotation state must not carry the
  // previous account's tokens into the new session.
  it("forgets rotated tokens when the session is reset", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse(401, { error: { code: "token_expired", message: "expired" } }),
      )
      .mockResolvedValueOnce(jsonResponse(200, rotated))
      .mockResolvedValueOnce(jsonResponse(200, { ok: true }));
    await authedRequest("/v1/bookings/me", session, { onTokensRefreshed: vi.fn() });

    resetTokenRotation(null);

    fetchMock
      .mockResolvedValueOnce(
        jsonResponse(401, { error: { code: "token_expired", message: "expired" } }),
      )
      .mockResolvedValueOnce(jsonResponse(200, rotated))
      .mockResolvedValueOnce(jsonResponse(200, { ok: true }));
    await authedRequest("/v1/forms/me", session, { onTokensRefreshed: vi.fn() });

    const refreshCalls = fetchMock.mock.calls.filter(([url]) =>
      String(url).endsWith("/v1/auth/refresh"),
    );
    expect(refreshCalls).toHaveLength(2);
  });
});
