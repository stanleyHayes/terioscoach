"use client";

import { Download, FolderOpen } from "lucide-react";
import { Card } from "@/components/ui/Card";
import {
  PortalEmpty,
  PortalError,
  PortalLoading,
  PortalPage,
} from "@/components/portal/PortalPage";
import { documentsApi, formatBytes, type ClientDocument } from "@/lib/portal";
import { usePortalAction, usePortalData } from "@/lib/use-portal-data";

/**
 * Documents library (CX-09).
 *
 * Only files the practitioner has shared appear here — the API will not
 * return anything else, so there is no "hidden" state to render. Download
 * links are fetched one at a time, on click: a signed URL is short-lived
 * and there is no reason for one to sit in the page waiting to be copied.
 */
export default function DocumentsPage() {
  const documents = usePortalData<ClientDocument[]>(
    (session, callbacks) => documentsApi.listMine(session, callbacks),
    [],
  );
  const action = usePortalAction();

  async function download(document: ClientDocument) {
    const url = await action.run(document.id, (session, callbacks) =>
      documentsApi.downloadUrl(session, callbacks, document.id),
    );
    if (url) {
      // A new tab rather than a navigation: the portal stays where it was,
      // and the signed link never becomes this page's address.
      window.open(url, "_blank", "noopener,noreferrer");
    }
  }

  return (
    <PortalPage
      title="Your documents"
      intro="Everything your practitioner has shared with you — plans, handouts and anything else worth keeping."
    >
      {action.error ? (
        <Card>
          <p role="alert" className="text-sm text-danger-ink">
            {action.error}
          </p>
        </Card>
      ) : null}

      {documents.error ? (
        <PortalError message={documents.error} onRetry={documents.refresh} />
      ) : documents.data === null ? (
        <PortalLoading label="Loading your documents…" />
      ) : documents.data.length === 0 ? (
        <PortalEmpty
          icon={<FolderOpen size={32} />}
          title="Nothing here yet"
          body="When your practitioner shares a document with you, it appears here for you to keep."
        />
      ) : (
        <ul className="flex flex-col gap-3">
          {documents.data.map((document) => (
            <li key={document.id}>
              <Card className="terios-record-card">
                <div className="flex flex-wrap items-center justify-between gap-4">
                  <div className="min-w-0">
                    <h2 className="text-base font-semibold leading-[1.4] text-ink">
                      {document.title}
                    </h2>
                    <p className="mt-1 text-[13px] text-ink-muted">
                      {document.format ? document.format.toUpperCase() : "File"}
                      <span aria-hidden="true" className="mx-2 text-ink-faint">
                        ·
                      </span>
                      {formatBytes(document.bytes)}
                      <span aria-hidden="true" className="mx-2 text-ink-faint">
                        ·
                      </span>
                      <time dateTime={document.createdAt}>
                        {new Date(document.createdAt).toLocaleDateString("en-GB", {
                          day: "numeric",
                          month: "short",
                          year: "numeric",
                        })}
                      </time>
                    </p>
                  </div>
                  <button
                    type="button"
                    disabled={action.pending === document.id}
                    onClick={() => void download(document)}
                    className="inline-flex shrink-0 items-center gap-2 rounded-lg border border-border px-4 py-2 text-sm font-medium text-ink transition-colors duration-instant ease-out hover:bg-surface-sunken disabled:opacity-50"
                  >
                    <Download size={16} aria-hidden="true" />
                    {action.pending === document.id ? "Preparing…" : "Download"}
                  </button>
                </div>
              </Card>
            </li>
          ))}
        </ul>
      )}
    </PortalPage>
  );
}
