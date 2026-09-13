"use client";

import { useRef } from "react";
import { useRouter } from "next/navigation";
import {
  ArticleEditor,
  toArticleBody,
  type ArticleValues,
} from "@/components/content/ArticleEditor";
import { LoadFailure, Skeletons } from "@/components/content/states";
import { ApiError } from "@/lib/api";
import { pagesApi, type Page, type PagePatch } from "@/lib/content";
import { useAction, useResource } from "@/lib/use-resource";

export function PageEditorPage({ pageId }: { pageId?: string }) {
  const router = useRouter();
  const createdId = useRef<string | null>(null);
  const action = useAction();
  const pages = useResource<Page[]>(
    (session, callbacks) => pagesApi.list(session, callbacks),
    [],
  );
  const page = pageId ? pages.data?.find((item) => item.id === pageId) : null;

  async function save(values: ArticleValues) {
    const body = toArticleBody("page", values) as PagePatch & {
      slug: string;
      title: string;
      body: string;
    };
    const saved = await action.run<Page>(
      "page-editor",
      async (session, callbacks) => {
        if (page) return pagesApi.update(session, callbacks, page.id, body);
        if (createdId.current)
          return pagesApi.update(session, callbacks, createdId.current, body);
        const created = await pagesApi.create(session, callbacks, {
          slug: body.slug,
          title: body.title,
          body: body.body,
        });
        createdId.current = created.id;
        return pagesApi.update(session, callbacks, created.id, body);
      },
    );
    if (!saved)
      throw new ApiError(
        500,
        "write_failed",
        action.error || "The page did not save. Try again.",
      );
  }

  if (pages.error)
    return <LoadFailure message={pages.error} onRetry={pages.refresh} />;
  if (pages.data === null)
    return <Skeletons label="Opening the writing desk…" />;
  if (pageId && !page)
    return (
      <LoadFailure
        message="That page could not be found."
        onRetry={() => router.push("/content?tab=pages")}
      />
    );

  return (
    <ArticleEditor
      kind="page"
      article={page ?? null}
      presentation="page"
      onSaved={() => router.push("/content?tab=pages&saved=1")}
      onClose={() => router.push("/content?tab=pages")}
      onSubmit={save}
    />
  );
}
