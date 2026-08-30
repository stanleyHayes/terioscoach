"use client";

import {
  ChevronDown,
  ChevronUp,
  CircleAlert,
  Plus,
  Sparkles,
} from "lucide-react";
import { EmptyState } from "@/components/content/states";
import { KpiStrip } from "@/components/insights/KpiStrip";
import { useCallback, useEffect, useState } from "react";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { AdminPageHeader } from "@/components/layout/AdminPageHeader";
import { IconButton } from "@/components/ui/IconButton";
import { Switch } from "@/components/ui/Switch";
import { ApiError, SessionExpiredError } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { formatDuration, formatMoney } from "@/lib/format";
import {
  servicesApi,
  sortServices,
  type Service,
  type ServiceDraft,
} from "@/lib/services";
import { DeleteServiceModal } from "./DeleteServiceModal";
import { ServiceFormModal } from "./ServiceFormModal";

/**
 * Services & pricing manager (ADM-05).
 * DataTable per design-system §3.22: header row (title + primary action) →
 * table (name, duration, price, status, reorder, actions). The active Switch
 * PATCHes instantly with an optimistic update (reverts on error); reorder is
 * up/down IconButtons PATCHing sortOrder (no drag-drop in v1).
 */

function errorMessage(error: unknown): string {
  return error instanceof ApiError
    ? error.message
    : "Something went wrong. Try again.";
}

export default function ServicesPage() {
  const { session, refreshCallbacks, logout } = useAuth();
  const [services, setServices] = useState<Service[] | null>(null);
  const [listError, setListError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<Service | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Service | null>(null);
  const [pendingToggleId, setPendingToggleId] = useState<string | null>(null);

  // A session that can no longer be refreshed ends here — sign out with the
  // branded message handed to the login screen.
  const handleSessionExpiry = useCallback(
    (error: unknown) => {
      if (error instanceof SessionExpiredError) {
        void logout(error.message);
      }
    },
    [logout],
  );

  const load = useCallback(() => {
    if (!session) return;
    let cancelled = false;
    servicesApi
      .listAll(session, refreshCallbacks)
      .then((items) => {
        if (!cancelled) {
          setListError(null);
          setServices(sortServices(items));
        }
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        setListError(errorMessage(error));
        handleSessionExpiry(error);
      });
    return () => {
      cancelled = true;
    };
  }, [session, refreshCallbacks, handleSessionExpiry]);

  useEffect(() => load(), [load]);

  function replaceService(updated: Service) {
    setServices((prev) =>
      prev
        ? sortServices(prev.map((s) => (s.id === updated.id ? updated : s)))
        : prev,
    );
  }

  async function handleToggle(service: Service, active: boolean) {
    if (!session) return;
    setActionError(null);
    setPendingToggleId(service.id);
    // Optimistic: flip immediately, revert on failure.
    setServices((prev) =>
      prev
        ? prev.map((s) => (s.id === service.id ? { ...s, active } : s))
        : prev,
    );
    try {
      replaceService(
        await servicesApi.update(session, refreshCallbacks, service.id, {
          active,
        }),
      );
    } catch (error) {
      setServices((prev) =>
        prev ? prev.map((s) => (s.id === service.id ? service : s)) : prev,
      );
      setActionError(errorMessage(error));
      handleSessionExpiry(error);
    } finally {
      setPendingToggleId(null);
    }
  }

  async function handleMove(index: number, direction: -1 | 1) {
    if (!session || !services) return;
    const target = index + direction;
    if (target < 0 || target >= services.length) return;

    const moved = services[index]!;
    const displaced = services[target]!;
    // Swap sortOrder values; equal values get a bump so the move still lands.
    let movedOrder = displaced.sortOrder;
    let displacedOrder = moved.sortOrder;
    if (movedOrder === displacedOrder) {
      if (direction === 1) movedOrder = displacedOrder + 1;
      else displacedOrder = movedOrder + 1;
    }

    const reordered = [...services];
    reordered[index] = { ...displaced, sortOrder: displacedOrder };
    reordered[target] = { ...moved, sortOrder: movedOrder };
    setActionError(null);
    setServices(reordered);

    try {
      const [updatedDisplaced, updatedMoved] = await Promise.all([
        servicesApi.update(session, refreshCallbacks, displaced.id, {
          sortOrder: displacedOrder,
        }),
        servicesApi.update(session, refreshCallbacks, moved.id, {
          sortOrder: movedOrder,
        }),
      ]);
      setServices((prev) =>
        prev
          ? sortServices(
              prev.map((s) =>
                s.id === updatedMoved.id
                  ? updatedMoved
                  : s.id === updatedDisplaced.id
                    ? updatedDisplaced
                    : s,
              ),
            )
          : prev,
      );
    } catch (error) {
      setServices(services);
      setActionError(errorMessage(error));
      handleSessionExpiry(error);
    }
  }

  async function handleFormSubmit(draft: ServiceDraft, target: Service | null) {
    if (!session) return;
    if (target) {
      replaceService(
        await servicesApi.update(session, refreshCallbacks, target.id, draft),
      );
    } else {
      const created = await servicesApi.create(
        session,
        refreshCallbacks,
        draft,
      );
      setServices((prev) =>
        prev ? sortServices([...prev, created]) : [created],
      );
    }
  }

  async function handleDelete(service: Service) {
    if (!session) return;
    await servicesApi.remove(session, refreshCallbacks, service.id);
    setServices((prev) =>
      prev ? prev.filter((s) => s.id !== service.id) : prev,
    );
  }

  const columnCount = 6;

  return (
    <div data-admin-page="services" className="flex flex-col gap-6">
      <AdminPageHeader
        eyebrow="Practice menu"
        title="Services"
        description="Shape what you offer, what it costs, and the order clients see it in."
        actions={
          <Button
            onClick={() => {
              setEditTarget(null);
              setFormOpen(true);
            }}
          >
            <Plus size={16} aria-hidden="true" />
            New service
          </Button>
        }
      />

      {services ? (
        <KpiStrip
          label="Service summary"
          items={[
            {
              label: "Services",
              value: String(services.length),
              detail: "in the practice menu",
            },
            {
              label: "Published",
              value: String(
                services.filter((service) => service.active).length,
              ),
              detail: "available to book",
            },
            {
              label: "Average duration",
              value: services.length
                ? `${Math.round(services.reduce((sum, service) => sum + service.durationMinutes, 0) / services.length)} min`
                : "—",
              detail: "per appointment",
            },
            {
              label: "Average price",
              value: services.length
                ? formatMoney(
                    Math.round(
                      services.reduce(
                        (sum, service) => sum + service.priceKobo,
                        0,
                      ) / services.length,
                    ),
                    services[0]?.currency || "GHS",
                  )
                : "—",
              detail: "across the menu",
            },
          ]}
        />
      ) : null}

      {actionError ? (
        <div
          role="alert"
          className="flex items-start gap-2 rounded-md bg-danger-bg px-4 py-3 text-sm leading-[1.55] text-danger-ink"
        >
          <CircleAlert
            size={16}
            aria-hidden="true"
            className="mt-0.5 shrink-0"
          />
          {actionError}
        </div>
      ) : null}

      <div className="overflow-x-auto rounded-lg border border-border bg-surface-raised">
        <table className="w-full text-left text-sm leading-[1.55]">
          <thead className="bg-surface-sunken">
            <tr className="border-b border-border">
              <th
                scope="col"
                className="px-4 py-3 text-[13px] font-semibold tracking-[0.01em] text-ink-muted"
              >
                Name
              </th>
              <th
                scope="col"
                className="px-4 py-3 text-[13px] font-semibold tracking-[0.01em] text-ink-muted"
              >
                Duration
              </th>
              <th
                scope="col"
                className="px-4 py-3 text-right text-[13px] font-semibold tracking-[0.01em] text-ink-muted"
              >
                Price
              </th>
              <th
                scope="col"
                className="px-4 py-3 text-[13px] font-semibold tracking-[0.01em] text-ink-muted"
              >
                Status
              </th>
              <th
                scope="col"
                className="px-4 py-3 text-[13px] font-semibold tracking-[0.01em] text-ink-muted"
              >
                <span className="sr-only">Reorder</span>
              </th>
              <th
                scope="col"
                className="px-4 py-3 text-right text-[13px] font-semibold tracking-[0.01em] text-ink-muted"
              >
                <span className="sr-only">Actions</span>
              </th>
            </tr>
          </thead>
          <tbody aria-busy={services === null || undefined}>
            {services === null && !listError ? (
              // Loading: 5 skeleton rows matching the cell shapes (§3.22/§3.28)
              Array.from({ length: 5 }, (_, row) => (
                <tr
                  key={row}
                  className="h-[52px] border-b border-border last:border-0"
                >
                  <td className="px-4">
                    <div
                      className="skeleton-shimmer h-4 w-40 rounded-sm"
                      aria-hidden="true"
                    />
                  </td>
                  <td className="px-4">
                    <div
                      className="skeleton-shimmer h-4 w-16 rounded-sm"
                      aria-hidden="true"
                    />
                  </td>
                  <td className="px-4">
                    <div
                      className="skeleton-shimmer ml-auto h-4 w-20 rounded-sm"
                      aria-hidden="true"
                    />
                  </td>
                  <td className="px-4">
                    <div
                      className="skeleton-shimmer h-5 w-24 rounded-full"
                      aria-hidden="true"
                    />
                  </td>
                  <td className="px-4" />
                  <td className="px-4" />
                </tr>
              ))
            ) : listError ? (
              <tr>
                <td colSpan={columnCount} className="px-6 py-12 text-center">
                  <p className="flex items-center justify-center gap-2 text-sm text-danger-ink">
                    <CircleAlert
                      size={16}
                      aria-hidden="true"
                      className="shrink-0"
                    />
                    {listError}
                  </p>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="mt-3"
                    onClick={() => {
                      setListError(null);
                      load();
                    }}
                  >
                    Try again
                  </Button>
                </td>
              </tr>
            ) : services && services.length === 0 ? (
              // EmptyState table variant (§3.27): reduced padding, no icon well
              <tr>
                <td colSpan={columnCount} className="px-6 py-12 text-center">
                  <EmptyState
                    compact
                    className="border-0 bg-transparent"
                    icon={<Sparkles size={24} />}
                    title="No services yet"
                    body="Add your first service and it will appear on the booking page."
                  />
                </td>
              </tr>
            ) : (
              services?.map((service, index) => (
                <tr
                  key={service.id}
                  className="h-[52px] border-b border-border transition-colors duration-fast ease-out last:border-0 hover:bg-eucalyptus-50"
                >
                  <td className="max-w-[320px] px-4 py-2">
                    <p className="truncate font-medium text-ink">
                      {service.name}
                    </p>
                    {service.description ? (
                      <p className="truncate text-[13px] leading-[1.45] text-ink-faint">
                        {service.description}
                      </p>
                    ) : null}
                  </td>
                  <td className="px-4 tabular-nums text-ink-muted">
                    {formatDuration(service.durationMinutes)}
                  </td>
                  <td className="px-4 text-right tabular-nums text-ink">
                    {formatMoney(service.priceKobo, service.currency)}
                  </td>
                  <td className="px-4">
                    <div className="flex items-center gap-2">
                      <Switch
                        checked={service.active}
                        loading={pendingToggleId === service.id}
                        label={
                          service.active
                            ? `Deactivate ${service.name}`
                            : `Activate ${service.name}`
                        }
                        onChange={(active) =>
                          void handleToggle(service, active)
                        }
                      />
                      <Badge
                        variant={service.active ? "success" : "neutral"}
                        dot
                      >
                        {service.active ? "Active" : "Inactive"}
                      </Badge>
                    </div>
                  </td>
                  <td className="px-4">
                    <div className="flex items-center gap-1">
                      <IconButton
                        variant="ghost"
                        size="sm"
                        aria-label={`Move ${service.name} up`}
                        disabled={index === 0}
                        onClick={() => void handleMove(index, -1)}
                      >
                        <ChevronUp aria-hidden="true" />
                      </IconButton>
                      <IconButton
                        variant="ghost"
                        size="sm"
                        aria-label={`Move ${service.name} down`}
                        disabled={index === services.length - 1}
                        onClick={() => void handleMove(index, 1)}
                      >
                        <ChevronDown aria-hidden="true" />
                      </IconButton>
                    </div>
                  </td>
                  <td className="px-4">
                    <div className="flex items-center justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="sm"
                        aria-label={`Edit ${service.name}`}
                        onClick={() => {
                          setEditTarget(service);
                          setFormOpen(true);
                        }}
                      >
                        Edit
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        aria-label={`Delete ${service.name}`}
                        className="text-danger hover:bg-danger-bg"
                        onClick={() => setDeleteTarget(service)}
                      >
                        Delete
                      </Button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {formOpen ? (
        <ServiceFormModal
          service={editTarget}
          onClose={() => {
            setFormOpen(false);
            setEditTarget(null);
          }}
          onSubmit={async (draft, target) => {
            try {
              await handleFormSubmit(draft, target);
            } catch (error) {
              handleSessionExpiry(error);
              throw error;
            }
          }}
        />
      ) : null}

      {deleteTarget ? (
        <DeleteServiceModal
          service={deleteTarget}
          onClose={() => setDeleteTarget(null)}
          onConfirm={async (service) => {
            try {
              await handleDelete(service);
            } catch (error) {
              handleSessionExpiry(error);
              throw error;
            }
          }}
        />
      ) : null}
    </div>
  );
}
