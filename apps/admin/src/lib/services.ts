/**
 * Typed client for the Services slice of the API contract (BE-03):
 *
 *   GET    /v1/services/all    → 200 {items}   (all non-deleted, incl. inactive)
 *   POST   /v1/services        → 201 {service}
 *   PATCH  /v1/services/{id}   → 200 {service} (partial, incl. active + sortOrder)
 *   DELETE /v1/services/{id}   → 204           (soft-deletes invisibly if booked)
 *
 * All calls go through authedRequest (single 401 → refresh → retry).
 */

import { authedRequest, type RefreshCallbacks, type Session } from "@/lib/api";

export interface Service {
  id: string;
  practitionerId: string;
  name: string;
  description: string;
  imageUrl?: string;
  durationMinutes: number;
  priceKobo: number;
  currency: string;
  active: boolean;
  sortOrder: number;
  createdAt: string;
  updatedAt: string;
}

/** Fields accepted on create. `currency` defaults to "USD" server-side when omitted. */
export interface ServiceDraft {
  name: string;
  description: string;
  imageUrl?: string;
  durationMinutes: number;
  priceKobo: number;
  currency?: string;
  sortOrder?: number;
}

/** Any subset on PATCH; `active` toggles visibility on the public list. */
export type ServicePatch = Partial<ServiceDraft> & { active?: boolean };

export const servicesApi = {
  async listAll(session: Session, callbacks: RefreshCallbacks): Promise<Service[]> {
    const data = await authedRequest<{ items: Service[] }>(
      "/v1/services/all",
      session,
      callbacks,
    );
    return data.items;
  },

  async create(
    session: Session,
    callbacks: RefreshCallbacks,
    draft: ServiceDraft,
  ): Promise<Service> {
    const data = await authedRequest<{ service: Service }>(
      "/v1/services",
      session,
      callbacks,
      { method: "POST", body: draft },
    );
    return data.service;
  },

  async update(
    session: Session,
    callbacks: RefreshCallbacks,
    id: string,
    patch: ServicePatch,
  ): Promise<Service> {
    const data = await authedRequest<{ service: Service }>(
      `/v1/services/${id}`,
      session,
      callbacks,
      { method: "PATCH", body: patch },
    );
    return data.service;
  },

  remove(session: Session, callbacks: RefreshCallbacks, id: string): Promise<void> {
    return authedRequest<void>(`/v1/services/${id}`, session, callbacks, {
      method: "DELETE",
    });
  },
};

/** Display ordering, mirroring the backend: sortOrder, then createdAt. */
export function sortServices(services: Service[]): Service[] {
  return [...services].sort(
    (a, b) => a.sortOrder - b.sortOrder || a.createdAt.localeCompare(b.createdAt),
  );
}
