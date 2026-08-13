"use client";

import { FileText, Pencil, Trash2 } from "lucide-react";
import { useState } from "react";
import {
  ArticleEditor,
  toArticleBody,
  type ArticleValues,
} from "@/components/content/ArticleEditor";
import { EmptyState, ErrorBanner, LoadFailure, Skeletons } from "@/components/content/states";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { ApiError, type RefreshCallbacks, type Session } from "@/lib/api";
import { pagesApi, postsApi, type Page, type Post, type PostPatch } from "@/lib/content";
import { useAction, useResource } from "@/lib/use-resource";

/**
 * The list behind both the Pages and Blog tabs (ADM-07).
 *
 * Publish is its own button, separate from Save, all the way down to the
 * API. That is the whole design: an editor can leave a half-finished draft
 * open for a week, and a live page can be revised without the revision
 * going out until it is meant to.
 */
export function ArticleSection({ kind }: { kind: "page" | "post" }) {
  const api = kind === "page" ? pagesApi : postsApi;
  const noun = kind === "page" ? "page" : "post";

  const articles = useResource<(Page | Post)[]>(
    (session, callbacks) =>
      kind === "page" ? pagesApi.list(session, callbacks) : postsApi.list(session, callbacks),
    [kind],
  );
  const action = useAction();
  const [editing, setEditing] = useState<Page | Post | null | undefined>(undefined);
  const [confirming, setConfirming] = useState<Page | Post | null>(null);

  const items = articles.data ?? [];
  const liveCount = items.filter((a) => a.status === "published").length;

  async function save(values: ArticleValues) {
    const existing = editing ?? null;
    const body = toArticleBody(kind, values);

    // The create route takes slug/title/body only; everything else arrives
    // as a follow-up patch. That keeps one form shape serving both routes
    // without the create endpoint having to learn every optional field.
    const patch = (session: Session, callbacks: RefreshCallbacks, id: string) =>
      kind === "page"
        ? pagesApi.update(session, callbacks, id, body)
        : postsApi.update(session, callbacks, id, body as PostPatch);

    const saved = await action.run<Page | Post>("form", async (session, callbacks) => {
      if (existing) return patch(session, callbacks, existing.id);
      const created = await api.create(session, callbacks, {
        slug: body.slug,
        title: body.title,
        body: body.body,
      });
      return patch(session, callbacks, created.id);
    });
    if (!saved) throw new ApiError(500, "write_failed", "It didn't save. Try again.");

    articles.set((current) => {
      const list = current ?? [];
      return existing
        ? list.map((a) => (a.id === saved.id ? saved : a))
        : [saved, ...list];
    });
  }

  async function setPublished(article: Page | Post, published: boolean) {
    const updated = await action.run(article.id, (session, callbacks) =>
      api.setPublished(session, callbacks, article.id, published),
    );
    if (updated) {
      articles.set((current) => (current ?? []).map((a) => (a.id === updated.id ? updated : a)));
    }
  }

  async function remove(article: Page | Post) {
    const done = await action.run(article.id, (session, callbacks) =>
      api.remove(session, callbacks, article.id).then(() => true),
    );
    if (done) {
      articles.set((current) => (current ?? []).filter((a) => a.id !== article.id));
      setConfirming(null);
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-ink-muted">
          {items.length === 0
            ? kind === "page"
              ? "Your site's standing pages live here."
              : "Write something your clients would find useful."
            : `${liveCount} of ${items.length} live on the site`}
        </p>
        <Button size="sm" onClick={() => setEditing(null)}>
          {kind === "page" ? "New page" : "Write a post"}
        </Button>
      </div>

      <ErrorBanner message={action.error} />

      {articles.error ? (
        <LoadFailure message={articles.error} onRetry={articles.refresh} />
      ) : articles.data === null ? (
        <Skeletons label={`Loading ${noun}s…`} />
      ) : items.length === 0 ? (
        <EmptyState
          icon={<FileText size={26} aria-hidden="true" className="text-ink-faint" />}
          title={kind === "page" ? "No pages yet" : "No posts yet"}
          body={
            kind === "page"
              ? "Pages are the parts of the site that don't change often — your story, your approach, your policies."
              : "Posts appear on the blog newest first. Nothing goes out until you publish it."
          }
          action={
            <Button size="sm" onClick={() => setEditing(null)}>
              {kind === "page" ? "New page" : "Write a post"}
            </Button>
          }
        />
      ) : (
        <ul className="flex flex-col gap-3">
          {items.map((article) => {
            const busy = action.pending === article.id;
            const live = article.status === "published";
            return (
              <li
                key={article.id}
                className="flex flex-wrap items-start justify-between gap-4 rounded-lg border border-border bg-surface-raised p-5"
              >
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <h3 className="text-sm font-medium text-ink">{article.title}</h3>
                    <Badge variant={live ? "success" : "neutral"}>
                      {live ? "Live" : "Draft"}
                    </Badge>
                  </div>
                  <p className="mt-1 font-mono text-[12px] text-ink-faint">
                    {kind === "page" ? `/${article.slug}` : `/blog/${article.slug}`}
                  </p>
                  <p className="mt-2 max-w-[68ch] text-[13px] leading-[1.55] text-ink-muted">
                    {summarize(article)}
                  </p>
                  <p className="mt-2 text-[12px] tabular-nums text-ink-faint">
                    {live && article.publishedAt
                      ? `Published ${formatDate(article.publishedAt)}`
                      : `Last edited ${formatDate(article.updatedAt)}`}
                  </p>
                </div>

                <div className="flex flex-wrap gap-2">
                  <Button
                    variant={live ? "secondary" : "primary"}
                    size="sm"
                    disabled={busy}
                    onClick={() => void setPublished(article, !live)}
                  >
                    {live ? "Unpublish" : "Publish"}
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={busy}
                    onClick={() => setEditing(article)}
                  >
                    <Pencil size={14} aria-hidden="true" className="mr-1.5" />
                    Edit
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={busy}
                    onClick={() => setConfirming(article)}
                  >
                    <Trash2 size={14} aria-hidden="true" className="mr-1.5" />
                    Delete
                  </Button>
                </div>
              </li>
            );
          })}
        </ul>
      )}

      {editing !== undefined ? (
        <ArticleEditor
          kind={kind}
          article={editing}
          onClose={() => setEditing(undefined)}
          onSubmit={save}
        />
      ) : null}

      {confirming ? (
        <Modal
          open
          onClose={() => setConfirming(null)}
          title={`Delete this ${noun}?`}
          description={
            confirming.status === "published"
              ? "It's live right now. Deleting takes it off the site immediately and can't be undone."
              : "This can't be undone."
          }
          footer={
            <>
              <Button variant="secondary" onClick={() => setConfirming(null)}>
                Keep it
              </Button>
              <Button
                variant="danger"
                loading={action.pending === confirming.id}
                onClick={() => void remove(confirming)}
              >
                Delete
              </Button>
            </>
          }
        >
          <p className="text-sm leading-[1.55] text-ink-muted">{confirming.title}</p>
        </Modal>
      ) : null}
    </div>
  );
}

/** A post's own excerpt if it has one, otherwise the opening of the body. */
function summarize(article: Page | Post): string {
  const excerpt = (article as Post).excerpt;
  const source = excerpt?.trim() || article.body;
  const flat = source.replace(/\s+/g, " ").trim();
  return flat.length > 160 ? `${flat.slice(0, 159)}…` : flat;
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString("en-GB", {
    day: "numeric",
    month: "short",
    year: "numeric",
  });
}
