"use client";

import { useEffect, useRef, useState } from "react";
import { Download, FileText, Trash2, Upload } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { useAuth } from "@/lib/auth";
import {
  DOCUMENT_ACCEPT_ATTRIBUTE,
  documentRejectionReason,
  documentsApi,
  type ClientDocument,
} from "@/lib/documents";
import { describe } from "@/lib/use-resource";

/**
 * ClientDocuments — sharing a file with one client, from the practice side.
 *
 * The API has supported this since BE-11 and the client portal has always
 * had a Documents page, but the dashboard had no way to put anything in it:
 * the client file only ever showed a document *count*. This is that missing
 * action, on the record the practitioner is already looking at.
 *
 * A shared document is visible to the client as soon as it uploads — there
 * is no separate publish step to forget. Hiding it again is one toggle, and
 * deleting removes the practice's copy too.
 */

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export function ClientDocuments({
  clientId,
  clientName,
}: {
  clientId: string;
  clientName: string;
}) {
  const { session, refreshCallbacks } = useAuth();
  const inputRef = useRef<HTMLInputElement>(null);
  const [documents, setDocuments] = useState<ClientDocument[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!session) return;
    let cancelled = false;
    documentsApi
      .listForClient(session, refreshCallbacks, clientId)
      .then((items) => {
        // Signed agreements are filed here too, by the API. They belong to
        // the Signed agreements card, which knows not to offer Delete on a
        // practice record.
        if (!cancelled) {
          setDocuments(items.filter((item) => item.kind === "client_document"));
        }
      })
      .catch(() => {
        if (!cancelled) setDocuments([]);
      });
    return () => {
      cancelled = true;
    };
  }, [clientId, refreshCallbacks, session]);

  async function share(file: File) {
    if (!session) return;
    const reason = documentRejectionReason(file);
    if (reason) {
      setError(reason);
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const created = await documentsApi.shareWithClient(
        session,
        refreshCallbacks,
        clientId,
        file,
      );
      setDocuments((current) => [created, ...(current ?? [])]);
    } catch (failure) {
      setError(describe(failure));
    } finally {
      setBusy(false);
      if (inputRef.current) inputRef.current.value = "";
    }
  }

  async function open(document: ClientDocument) {
    if (!session) return;
    setError(null);
    try {
      const url = await documentsApi.downloadUrl(session, refreshCallbacks, document.id);
      // Private documents are delivered through a short-lived signed URL, so
      // the address only exists at the moment it is asked for.
      window.open(url, "_blank", "noopener");
    } catch (failure) {
      setError(describe(failure));
    }
  }

  async function toggleVisibility(document: ClientDocument) {
    if (!session) return;
    setError(null);
    try {
      const updated = await documentsApi.setVisibility(
        session,
        refreshCallbacks,
        document.id,
        !document.visibleToClient,
      );
      setDocuments((current) =>
        (current ?? []).map((item) => (item.id === updated.id ? updated : item)),
      );
    } catch (failure) {
      setError(describe(failure));
    }
  }

  async function remove(document: ClientDocument) {
    if (!session) return;
    setError(null);
    try {
      await documentsApi.remove(session, refreshCallbacks, document.id);
      setDocuments((current) =>
        (current ?? []).filter((item) => item.id !== document.id),
      );
    } catch (failure) {
      setError(describe(failure));
    }
  }

  return (
    <div className="flex flex-col gap-3">
      <input
        ref={inputRef}
        type="file"
        accept={DOCUMENT_ACCEPT_ATTRIBUTE}
        className="sr-only"
        onChange={(event) => {
          const file = event.target.files?.[0];
          if (file) void share(file);
        }}
      />

      <Button
        size="sm"
        loading={busy}
        disabled={busy}
        onClick={() => inputRef.current?.click()}
      >
        <Upload size={15} aria-hidden="true" />
        Share a document
      </Button>
      <p className="text-xs leading-relaxed text-ink-muted">
        {clientName} sees it in their portal straight away. PDF, JPEG, PNG or
        WebP, up to 10 MB.
      </p>

      {error ? (
        <p role="alert" className="text-xs text-danger-ink">
          {error}
        </p>
      ) : null}

      {documents === null ? (
        <div className="h-10 animate-pulse rounded-lg bg-surface-sunken" />
      ) : documents.length === 0 ? (
        <p className="text-sm text-ink-muted">Nothing shared yet.</p>
      ) : (
        <ul className="flex flex-col gap-2">
          {documents.map((document) => (
            <li
              key={document.id}
              className="flex items-start gap-2 rounded-lg bg-surface-sunken p-2.5"
            >
              <FileText
                size={16}
                aria-hidden="true"
                className="mt-0.5 shrink-0 text-ink-faint"
              />
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium text-ink">
                  {document.title || document.filename}
                </p>
                <p className="mt-0.5 text-[11px] text-ink-muted">
                  {formatBytes(document.bytes)} ·{" "}
                  {document.visibleToClient ? "Visible to client" : "Hidden"}
                </p>
                <div className="mt-1.5 flex flex-wrap items-center gap-3">
                  <button
                    type="button"
                    onClick={() => void open(document)}
                    className="inline-flex items-center gap-1 text-[11px] font-semibold text-primary hover:text-primary-hover"
                  >
                    <Download size={12} aria-hidden="true" />
                    Open
                  </button>
                  <button
                    type="button"
                    onClick={() => void toggleVisibility(document)}
                    className="text-[11px] font-semibold text-ink-muted hover:text-ink"
                  >
                    {document.visibleToClient ? "Hide from client" : "Show to client"}
                  </button>
                  <button
                    type="button"
                    onClick={() => void remove(document)}
                    className="inline-flex items-center gap-1 text-[11px] font-semibold text-danger-ink hover:opacity-80"
                  >
                    <Trash2 size={12} aria-hidden="true" />
                    Delete
                  </button>
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
