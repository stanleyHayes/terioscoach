import { beforeEach, describe, expect, it, vi } from "vitest";
import type { RefreshCallbacks, Session } from "@/lib/api";
import { servicesApi, sortServices, type Service } from "./services";

const authedRequestMock = vi.fn();

vi.mock("@/lib/api", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...original,
    authedRequest: (...args: unknown[]) => authedRequestMock(...args),
  };
});

const session: Session = { accessToken: "access", refreshToken: "refresh" };
const callbacks: RefreshCallbacks = { onTokensRefreshed: vi.fn() };

function service(overrides: Partial<Service> = {}): Service {
  return {
    id: "svc-1",
    practitionerId: "prac-1",
    name: "Aromatherapy massage",
    description: "Full body, 60 minutes",
    durationMinutes: 60,
    priceKobo: 25000,
    currency: "GHS",
    active: true,
    sortOrder: 0,
    createdAt: "2026-08-01T10:00:00Z",
    updatedAt: "2026-08-01T10:00:00Z",
    ...overrides,
  };
}

beforeEach(() => {
  authedRequestMock.mockReset();
});

describe("servicesApi", () => {
  it("listAll GETs /v1/services/all and unwraps items", async () => {
    const items = [service()];
    authedRequestMock.mockResolvedValueOnce({ items });

    await expect(servicesApi.listAll(session, callbacks)).resolves.toEqual(items);
    expect(authedRequestMock).toHaveBeenCalledWith("/v1/services/all", session, callbacks);
  });

  it("create POSTs the draft and unwraps service", async () => {
    const created = service();
    authedRequestMock.mockResolvedValueOnce({ service: created });

    const draft = {
      name: "Aromatherapy massage",
      description: "Full body, 60 minutes",
      durationMinutes: 60,
      priceKobo: 25000,
      currency: "GHS",
    };
    await expect(servicesApi.create(session, callbacks, draft)).resolves.toEqual(created);
    expect(authedRequestMock).toHaveBeenCalledWith("/v1/services", session, callbacks, {
      method: "POST",
      body: draft,
    });
  });

  it("update PATCHes a partial and unwraps service", async () => {
    const updated = service({ active: false });
    authedRequestMock.mockResolvedValueOnce({ service: updated });

    await expect(
      servicesApi.update(session, callbacks, "svc-1", { active: false }),
    ).resolves.toEqual(updated);
    expect(authedRequestMock).toHaveBeenCalledWith(
      "/v1/services/svc-1",
      session,
      callbacks,
      { method: "PATCH", body: { active: false } },
    );
  });

  it("remove DELETEs and resolves to nothing", async () => {
    authedRequestMock.mockResolvedValueOnce(undefined);

    await expect(servicesApi.remove(session, callbacks, "svc-1")).resolves.toBeUndefined();
    expect(authedRequestMock).toHaveBeenCalledWith(
      "/v1/services/svc-1",
      session,
      callbacks,
      { method: "DELETE" },
    );
  });
});

describe("sortServices", () => {
  it("orders by sortOrder, then createdAt, without mutating the input", () => {
    const a = service({ id: "a", sortOrder: 2, createdAt: "2026-08-01T10:00:00Z" });
    const b = service({ id: "b", sortOrder: 0, createdAt: "2026-08-03T10:00:00Z" });
    const c = service({ id: "c", sortOrder: 0, createdAt: "2026-08-02T10:00:00Z" });
    const input = [a, b, c];

    expect(sortServices(input).map((s) => s.id)).toEqual(["c", "b", "a"]);
    expect(input.map((s) => s.id)).toEqual(["a", "b", "c"]);
  });
});
