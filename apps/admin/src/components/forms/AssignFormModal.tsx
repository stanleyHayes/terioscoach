"use client";

import { CircleAlert, Search, UserRoundSearch } from "lucide-react";
import { EmptyState } from "@/components/content/states";
import { useMemo, useState } from "react";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { TextInput } from "@/components/ui/TextInput";
import { ApiError } from "@/lib/api";
import { clientsApi, type ClientSummary } from "@/lib/clients";
import { cn } from "@/lib/cn";
import type { FormDefinition } from "@/lib/forms";
import { useResource } from "@/lib/use-resource";

/**
 * Assigning a form to a client (ADM-08).
 *
 * The client list is searched in the browser rather than round-tripping per
 * keystroke: a single practice's client list is small, it is already loaded
 * for the dashboard, and a search that answers instantly is worth more than
 * one that scales to a list this will never have.
 */
export function AssignFormModal({
  form,
  onClose,
  onAssign,
}: {
  form: FormDefinition;
  onClose: () => void;
  onAssign: (clientId: string) => Promise<void>;
}) {
  const clients = useResource<ClientSummary[]>(
    (session, callbacks) => clientsApi.list(session, callbacks),
    [],
  );
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const matches = useMemo(() => {
    const items = clients.data ?? [];
    const needle = query.trim().toLowerCase();
    if (!needle) return items;
    return items.filter(
      (client) =>
        client.name.toLowerCase().includes(needle) ||
        client.email.toLowerCase().includes(needle),
    );
  }, [clients.data, query]);

  async function handleAssign() {
    if (!selected) return;
    setError(null);
    setSubmitting(true);
    try {
      await onAssign(selected);
      onClose();
    } catch (failure) {
      setError(
        failure instanceof ApiError
          ? failure.message
          : "Something went wrong. Try again.",
      );
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open
      onClose={onClose}
      title={`Send "${form.title}"`}
      description="They'll see it in their portal the next time they sign in."
      size="form"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button
            disabled={!selected}
            loading={submitting}
            onClick={() => void handleAssign()}
          >
            Send it
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-4">
        {error ? (
          <div
            role="alert"
            className="flex items-start gap-2 rounded-md bg-danger-bg px-4 py-3 text-sm leading-[1.55] text-danger-ink"
          >
            <CircleAlert
              size={16}
              aria-hidden="true"
              className="mt-0.5 shrink-0"
            />
            {error}
          </div>
        ) : null}

        <TextInput
          label="Find a client"
          data-autofocus
          value={query}
          leadingIcon={<Search aria-hidden="true" />}
          placeholder="Name or email"
          onChange={(event) => setQuery(event.target.value)}
        />

        {clients.error ? (
          <p role="alert" className="text-sm text-ink-muted">
            {clients.error}
          </p>
        ) : clients.data === null ? (
          <p role="status" aria-busy="true" className="text-sm text-ink-muted">
            Loading clients…
          </p>
        ) : matches.length === 0 ? (
          <EmptyState
            compact
            icon={<UserRoundSearch size={24} />}
            title={query ? "No one matches that" : "You have no clients yet"}
            body={
              query
                ? "Try a shorter name or email address."
                : "A client appears here after their first booking."
            }
            action={
              query ? (
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  onClick={() => setQuery("")}
                >
                  Clear search
                </Button>
              ) : undefined
            }
          />
        ) : (
          <ul
            role="radiogroup"
            aria-label="Client"
            className="flex max-h-72 flex-col gap-1 overflow-y-auto"
          >
            {matches.map((client) => (
              <li key={client.id}>
                {/* A custom radio row: native radios are forbidden, and the
                    hit area here is the whole row, not a 16px circle. */}
                <button
                  type="button"
                  role="radio"
                  aria-checked={selected === client.id}
                  onClick={() => setSelected(client.id)}
                  className={cn(
                    "flex w-full items-center justify-between gap-3 rounded-md border px-3 py-2.5 text-left transition-colors duration-instant ease-out",
                    selected === client.id
                      ? "border-primary bg-eucalyptus-50"
                      : "border-border hover:border-border-strong",
                  )}
                >
                  <span className="min-w-0">
                    <span className="block truncate text-sm text-ink">
                      {client.name}
                    </span>
                    <span className="block truncate text-[13px] text-ink-muted">
                      {client.email}
                    </span>
                  </span>
                  <span
                    aria-hidden="true"
                    className={cn(
                      "size-4 shrink-0 rounded-full border-2",
                      selected === client.id
                        ? "border-primary bg-primary ring-2 ring-surface-raised ring-inset"
                        : "border-border-strong",
                    )}
                  />
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </Modal>
  );
}
