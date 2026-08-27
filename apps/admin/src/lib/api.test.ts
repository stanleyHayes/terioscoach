import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  ApiError,
  API_BASE_URL,
  authApi,
  authedRequest,
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

beforeEach(() => {
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  // note: no vi.unstubAllGlobals() — it would undo the storage stubs from vitest.setup.ts
  fetchMock.mockReset();
});

describe("api client", () => {
  it("supports practitioner password recovery", async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));
    await expect(authApi.forgotPassword("practice@example.com")).resolves.toBeUndefined();
    expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body))).toEqual({ email: "practice@example.com" });
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));
    await expect(authApi.resetPassword("one-time-token", "a safer new password")).resolves.toBeUndefined();
  });

  it("posts JSON to the login endpoint and parses the auth response", async () => {
    const payload = {
      accessToken: "a1",
      refreshToken: "r1",
      user: { id: "u1", email: "a@b.com", role: "practitioner", name: "Akosua" },
    };
    fetchMock.mockResolvedValueOnce(jsonResponse(200, payload));

    const result = await authApi.login("a@b.com", "secret");

    expect(fetchMock).toHaveBeenCalledWith(`${API_BASE_URL}/v1/auth/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email: "a@b.com", password: "secret" }),
    });
    expect(result).toEqual(payload);
  });

  it("maps the {error:{code,message}} shape to a typed ApiError", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(401, { error: { code: "invalid_credentials", message: "Nope" } }),
    );

    const error = await authApi.login("a@b.com", "wrong").catch((e) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect(error.status).toBe(401);
    expect(error.code).toBe("invalid_credentials");
    expect(error.message).toBe("Nope");
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

    const error = await authApi.login("a@b.com", "secret").catch((e) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect(error.status).toBe(0);
    expect(error.code).toBe("network_error");
  });

  it("sends the bearer token on /v1/auth/me", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(200, { user: { id: "u1", email: "a@b.com", role: "practitioner", name: "A" } }),
    );

    await authApi.me("access-123");

    expect(fetchMock).toHaveBeenCalledWith(`${API_BASE_URL}/v1/auth/me`, {
      method: "GET",
      headers: { Authorization: "Bearer access-123" },
    });
  });

  it("returns undefined for 204 No Content (logout)", async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));

    await expect(authApi.logout("r1")).resolves.toBeUndefined();
  });
});

describe("authedRequest 401 recovery", () => {
  const session = { accessToken: "old-access", refreshToken: "refresh-1" };
  const rotated = { accessToken: "new-access", refreshToken: "refresh-2" };

  // The rotation state is shared across calls by design; each test starts
  // from a clean one, exactly as a fresh page load would.
  beforeEach(() => {
    resetTokenRotation(null);
  });

  it("refreshes once after a 401, retries once, and reports the rotated tokens", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse(401, { error: { code: "unauthorized", message: "expired" } }),
      )
      .mockResolvedValueOnce(
        jsonResponse(200, { accessToken: "new-access", refreshToken: "refresh-2" }),
      )
      .mockResolvedValueOnce(jsonResponse(200, { user: { id: "u1" } }));

    const onTokensRefreshed = vi.fn();
    const result = await authedRequest<{ user: { id: string } }>(
      "/v1/auth/me",
      session,
      { onTokensRefreshed },
    );

    expect(result.user.id).toBe("u1");
    expect(onTokensRefreshed).toHaveBeenCalledWith({
      accessToken: "new-access",
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
        jsonResponse(401, { error: { code: "unauthorized", message: "expired" } }),
      )
      .mockResolvedValueOnce(
        jsonResponse(401, { error: { code: "invalid_token", message: "bad refresh" } }),
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
        jsonResponse(401, { error: { code: "unauthorized", message: "expired" } }),
      )
      .mockResolvedValueOnce(
        jsonResponse(200, { accessToken: "new-access", refreshToken: "refresh-2" }),
      )
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
  // when a spent one comes back. /forms mounts two authenticated reads and
  // /content mounts four, so simultaneous expiry is the normal case, not a
  // rare one — they must still cost exactly one rotation.
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
          : jsonResponse(401, { error: { code: "unauthorized", message: "expired" } }),
      );
    });

    const onTokensRefreshed = vi.fn();
    const results = await Promise.all([
      authedRequest<{ ok: string }>("/v1/forms", session, { onTokensRefreshed }),
      authedRequest<{ ok: string }>("/v1/forms/submissions", session, { onTokensRefreshed }),
      authedRequest<{ ok: string }>("/v1/content/faqs", session, { onTokensRefreshed }),
      authedRequest<{ ok: string }>("/v1/content/testimonials", session, { onTokensRefreshed }),
    ]);

    expect(results.map((r) => r.ok)).toEqual([
      `${API_BASE_URL}/v1/forms`,
      `${API_BASE_URL}/v1/forms/submissions`,
      `${API_BASE_URL}/v1/content/faqs`,
      `${API_BASE_URL}/v1/content/testimonials`,
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
        jsonResponse(401, { error: { code: "unauthorized", message: "expired" } }),
      )
      .mockResolvedValueOnce(jsonResponse(200, rotated))
      .mockResolvedValueOnce(jsonResponse(200, { first: true }))
      .mockResolvedValueOnce(
        jsonResponse(401, { error: { code: "unauthorized", message: "expired" } }),
      )
      .mockResolvedValueOnce(jsonResponse(200, { second: true }));

    const onTokensRefreshed = vi.fn();
    await authedRequest("/v1/forms", session, { onTokensRefreshed });
    // Same stale `session` object a mounted component would still be holding.
    await authedRequest("/v1/clients", session, { onTokensRefreshed });

    const refreshCalls = fetchMock.mock.calls.filter(([url]) =>
      String(url).endsWith("/v1/auth/refresh"),
    );
    expect(refreshCalls).toHaveLength(1);
    expect(fetchMock).toHaveBeenLastCalledWith(`${API_BASE_URL}/v1/clients`, {
      method: "GET",
      headers: { Authorization: "Bearer new-access" },
    });
  });

  // Sign out, sign in as someone else: the rotation state must not carry the
  // previous account's tokens into the new session.
  it("forgets rotated tokens when the session is reset", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse(401, { error: { code: "unauthorized", message: "expired" } }),
      )
      .mockResolvedValueOnce(jsonResponse(200, rotated))
      .mockResolvedValueOnce(jsonResponse(200, { ok: true }));
    await authedRequest("/v1/forms", session, { onTokensRefreshed: vi.fn() });

    resetTokenRotation(null);

    fetchMock
      .mockResolvedValueOnce(
        jsonResponse(401, { error: { code: "unauthorized", message: "expired" } }),
      )
      .mockResolvedValueOnce(jsonResponse(200, rotated))
      .mockResolvedValueOnce(jsonResponse(200, { ok: true }));
    await authedRequest("/v1/clients", session, { onTokensRefreshed: vi.fn() });

    const refreshCalls = fetchMock.mock.calls.filter(([url]) =>
      String(url).endsWith("/v1/auth/refresh"),
    );
    expect(refreshCalls).toHaveLength(2);
  });
});
