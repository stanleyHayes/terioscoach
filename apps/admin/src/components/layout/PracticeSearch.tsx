"use client";

import { CircleAlert, Loader2, Search, UserRound } from "lucide-react";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useRef, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { useAuth } from "@/lib/auth";
import { clientsApi, type ClientSummary } from "@/lib/clients";
import { describe } from "@/lib/use-resource";

function normalize(value: string) {
  return value.normalize("NFD").replace(/[\u0300-\u036f]/g, "").toLowerCase().trim();
}

export function filterPracticeClients(clients: ClientSummary[], query: string) {
  const needle = normalize(query);
  if (!needle) return clients.slice(0, 8);
  return clients.filter((client) =>
    normalize([client.name, client.email, client.phone ?? "", ...client.tags].join(" ")).includes(needle),
  ).slice(0, 12);
}

export function PracticeSearch() {
  const router = useRouter();
  const { session, refreshCallbacks } = useAuth();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [clients, setClients] = useState<ClientSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const loaded = useRef(false);

  function openSearch() {
    setOpen(true);
    setQuery("");
    setError(null);
    if (!loaded.current) setLoading(true);
  }

  useEffect(() => {
    function handleShortcut(event: KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        openSearch();
      }
    }
    document.addEventListener("keydown", handleShortcut);
    return () => document.removeEventListener("keydown", handleShortcut);
  }, []);

  useEffect(() => {
    if (!open || !session || loaded.current) return;
    clientsApi.list(session, refreshCallbacks)
      .then((items) => {
        setClients(items);
        loaded.current = true;
      })
      .catch((failure) => setError(describe(failure)))
      .finally(() => setLoading(false));
  }, [open, refreshCallbacks, session]);

  const results = useMemo(() => filterPracticeClients(clients, query), [clients, query]);

  function choose(client: ClientSummary) {
    setOpen(false);
    router.push(`/clients/${client.id}`);
  }

  return (
    <>
      <button
        type="button"
        onClick={openSearch}
        className="flex h-10 w-full items-center gap-3 rounded-xl border border-border bg-surface-sunken px-3 text-sm text-ink-muted transition-colors hover:border-border-strong hover:text-ink"
      >
        <Search size={16} aria-hidden="true" />
        <span className="flex-1 text-left">Find clients and records</span>
        <kbd className="rounded border border-border bg-surface-raised px-1.5 py-0.5 text-[10px]">
          ⌘K
        </kbd>
      </button>

      <Modal open={open} onClose={() => setOpen(false)} title="Find clients and records" description="Search by name, email, phone number, or care tag." size="form">
        <label className="relative block">
          <span className="sr-only">Search client records</span>
          <Search aria-hidden="true" className="absolute left-4 top-1/2 size-5 -translate-y-1/2 text-ink-faint" />
          <input
            data-autofocus
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter" && results[0]) choose(results[0]);
            }}
            placeholder="Start typing a client name…"
            autoComplete="off"
            className="h-12 w-full rounded-xl border border-border bg-surface pl-12 pr-4 text-base text-ink outline-none placeholder:text-ink-faint focus:border-primary focus:ring-2 focus:ring-primary/15"
          />
        </label>

        <div className="mt-4 min-h-36 overflow-hidden rounded-xl border border-border bg-surface">
          {loading ? (
            <div role="status" className="flex min-h-36 items-center justify-center text-sm text-ink-muted"><Loader2 className="mr-2 size-4 animate-spin" aria-hidden="true" />Loading client records…</div>
          ) : error ? (
            <div role="alert" className="flex min-h-36 items-center justify-center gap-2 px-6 text-sm text-danger-ink"><CircleAlert size={16} aria-hidden="true" />{error}</div>
          ) : results.length ? (
            <ul className="divide-y divide-border" aria-label="Client records">
              {results.map((client) => (
                <li key={client.id}>
                  <button type="button" onClick={() => choose(client)} className="flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-surface-sunken focus-visible:bg-surface-sunken focus-visible:outline-none">
                    <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-eucalyptus-100 text-eucalyptus-800"><UserRound size={16} aria-hidden="true" /></span>
                    <span className="min-w-0 flex-1"><span className="block truncate text-sm font-semibold text-ink">{client.name}</span><span className="block truncate text-xs text-ink-muted">{client.email}</span></span>
                    <span className="hidden text-xs text-ink-faint sm:block">{client.totalSessions} {client.totalSessions === 1 ? "session" : "sessions"}</span>
                  </button>
                </li>
              ))}
            </ul>
          ) : (
            <div className="grid min-h-36 place-items-center px-6 text-center text-sm text-ink-muted">{clients.length ? "No client record matches that search." : "No client records are available yet."}</div>
          )}
        </div>
        <p className="mt-3 text-xs text-ink-faint">Press Enter to open the first result, or select any client.</p>
      </Modal>
    </>
  );
}
