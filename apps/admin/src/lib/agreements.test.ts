import { describe, expect, it, vi } from "vitest";
import { agreementsApi } from "./agreements";

const fetchMock = vi.fn();
vi.stubGlobal("fetch", fetchMock);

describe("agreementsApi.countersign", () => {
  it("posts the signedName field the API expects", async () => {
    fetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        signature: {
          id: "sig-1",
          agreementId: "agr-1",
          agreementTitle: "Holistic Coaching Agreement",
          agreementVersion: 1,
          clientId: "client-1",
          clientName: "Daniel Baah",
          signedName: "Daniel Baah",
          signedAt: "2026-09-17T10:00:00Z",
          requiresCountersignature: true,
          practitionerSignedName: "Dr. Stanley Hayes",
          practitionerSignedAt: "2026-09-17T10:05:00Z",
        },
      }),
    });

    await agreementsApi.countersign(
      { accessToken: "token", refreshToken: "refresh" },
      { onTokensRefreshed: vi.fn() },
      "sig-1",
      "Dr. Stanley Hayes",
    );

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining("/v1/admin/signatures/sig-1/countersign"),
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ signedName: "Dr. Stanley Hayes" }),
      }),
    );
  });
});
