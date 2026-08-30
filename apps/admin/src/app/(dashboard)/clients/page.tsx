"use client";

import Link from "next/link";
import { Search, Users } from "lucide-react";
import { useMemo, useState } from "react";
import { Button } from "@/components/ui/Button";
import { EmptyState } from "@/components/content/states";
import { AdminPageHeader } from "@/components/layout/AdminPageHeader";
import { cn } from "@/lib/cn";
import { clientsApi, type ClientSummary } from "@/lib/clients";
import { useResource } from "@/lib/use-resource";

/**
 * Client list (ADM-03).
 *
 * Ordered by the API as last-seen-first, which is the order a practitioner
 * actually thinks in — who was here recently, then everyone else. Search is
 * local because a single practice's list is small: filtering in the browser
 * is instant and costs no round trip.
 */
export default function ClientsPage() {
  const [query, setQuery] = useState("");

  const clients = useResource<ClientSummary[]>(
    (session, callbacks) => clientsApi.list(session, callbacks),
    [],
  );

  // Memoized, not `?? []` inline: a fresh empty array every render would
  // make every useMemo below it recompute, which is the opposite of why
  // they are there.
  const items = useMemo(() => clients.data ?? [], [clients.data]);
  const visible = useMemo(() => filterClients(items, query), [items, query]);

  return (
    <div data-admin-page="clients" className="flex flex-col gap-6">
      <AdminPageHeader
        eyebrow="Care directory"
        title="Clients"
        description={
          items.length === 0
            ? "Everyone who books becomes a client here."
            : `${items.length} ${items.length === 1 ? "client" : "clients"} in your practice.`
        }
        actions={
          <div className="relative w-full max-w-[280px]">
            <label htmlFor="client-search" className="sr-only">
              Search clients
            </label>
            <Search
              size={16}
              aria-hidden="true"
              className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-faint"
            />
            <input
              id="client-search"
              type="text"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Name, email or tag"
              autoComplete="off"
              className={cn(
                "h-9 w-full rounded-md border border-border bg-surface-raised pl-9 pr-3 text-sm text-ink",
                "placeholder:text-ink-faint transition-colors duration-instant ease-out",
                "focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/20",
              )}
            />
          </div>
        }
      />

      {clients.error ? (
        <div
          role="alert"
          className="rounded-lg border border-border bg-surface-raised p-8 text-center"
        >
          <p className="text-sm text-ink-muted">{clients.error}</p>
          <div className="mt-4">
            <Button variant="secondary" size="sm" onClick={clients.refresh}>
              Try again
            </Button>
          </div>
        </div>
      ) : clients.data === null ? (
        <div role="status" aria-busy="true" className="flex flex-col gap-2">
          <span className="sr-only">Loading clients…</span>
          {[0, 1, 2, 3].map((i) => (
            <span
              key={i}
              aria-hidden="true"
              className="h-14 rounded-lg bg-surface-sunken"
            />
          ))}
        </div>
      ) : visible.length === 0 ? (
        <EmptyState
          icon={<Users size={26} />}
          title={items.length === 0 ? "No clients yet" : "Nobody matches that"}
          body={
            items.length === 0
              ? "A client appears here as soon as they book their first session."
              : "Try part of a name, an email address, or a tag you have used."
          }
        />
      ) : (
        <div className="overflow-hidden rounded-lg border border-border bg-surface-raised">
          <table className="w-full border-collapse text-left">
            <caption className="sr-only">
              Clients, most recently seen first
            </caption>
            <thead>
              <tr className="border-b border-border">
                <th
                  scope="col"
                  className="px-5 py-3 text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-muted"
                >
                  Client
                </th>
                <th
                  scope="col"
                  className="px-5 py-3 text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-muted"
                >
                  Sessions
                </th>
                <th
                  scope="col"
                  className="px-5 py-3 text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-muted"
                >
                  Last seen
                </th>
                <th
                  scope="col"
                  className="px-5 py-3 text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-muted"
                >
                  Tags
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {visible.map((client) => (
                <tr
                  key={client.id}
                  className="transition-colors duration-instant ease-out hover:bg-surface-sunken"
                >
                  <td className="px-5 py-4">
                    <Link
                      href={`/clients/${client.id}`}
                      className="text-sm font-semibold text-ink transition-colors duration-instant ease-out hover:text-primary"
                    >
                      {client.name}
                    </Link>
                    <span className="mt-0.5 block text-[13px] text-ink-muted">
                      {client.email}
                    </span>
                  </td>
                  <td className="px-5 py-4 text-sm tabular-nums text-ink">
                    {client.totalSessions}
                  </td>
                  <td className="px-5 py-4 text-sm tabular-nums text-ink-muted">
                    {client.lastSessionAt ? (
                      <time dateTime={client.lastSessionAt}>
                        {new Date(client.lastSessionAt).toLocaleDateString(
                          "en-GB",
                          {
                            day: "numeric",
                            month: "short",
                            year: "numeric",
                          },
                        )}
                      </time>
                    ) : (
                      <span className="text-ink-faint">Not yet</span>
                    )}
                  </td>
                  <td className="px-5 py-4">
                    {client.tags.length > 0 ? (
                      <span className="flex flex-wrap gap-1.5">
                        {client.tags.map((tag) => (
                          <span
                            key={tag}
                            className="rounded-full bg-surface-sunken px-2 py-0.5 text-[11px] font-medium text-ink-muted"
                          >
                            {tag}
                          </span>
                        ))}
                      </span>
                    ) : (
                      <span className="text-[13px] text-ink-faint">—</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

/** Local filter across name, email and tags — case- and accent-insensitive
 * so "Sero" finds "Séro". */
export function filterClients(
  clients: ClientSummary[],
  query: string,
): ClientSummary[] {
  const needle = normalize(query);
  if (!needle) return clients;
  return clients.filter(
    (client) =>
      normalize(client.name).includes(needle) ||
      normalize(client.email).includes(needle) ||
      client.tags.some((tag) => normalize(tag).includes(needle)),
  );
}

function normalize(value: string): string {
  return value
    .toLowerCase()
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .trim();
}
