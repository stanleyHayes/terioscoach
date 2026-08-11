import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  ApiError,
  API_BASE_URL,
  authApi,
  authedRequest,
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
});
