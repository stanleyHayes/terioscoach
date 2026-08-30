"use client";

import { useRouter } from "next/navigation";
import { ArticleEditor, toArticleBody, type ArticleValues } from "@/components/content/ArticleEditor";
import { LoadFailure, Skeletons } from "@/components/content/states";
import { ApiError } from "@/lib/api";
import { postsApi, type Post, type PostPatch } from "@/lib/content";
import { useAction, useResource } from "@/lib/use-resource";

export function PostEditorPage({ postId }: { postId?: string }) {
  const router = useRouter();
  const action = useAction();
  const posts = useResource<Post[]>((session, callbacks) => postsApi.list(session, callbacks), []);
  const post = postId ? posts.data?.find((item) => item.id === postId) : null;

  async function save(values: ArticleValues) {
    const body = toArticleBody("post", values) as PostPatch & { slug: string; title: string; body: string };
    const saved = await action.run<Post>("post-editor", async (session, callbacks) => {
      if (post) return postsApi.update(session, callbacks, post.id, body);
      const created = await postsApi.create(session, callbacks, { slug: body.slug, title: body.title, body: body.body });
      return postsApi.update(session, callbacks, created.id, body);
    });
    if (!saved) throw new ApiError(500, "write_failed", action.error || "The post did not save. Try again.");
  }

  if (posts.error) return <LoadFailure message={posts.error} onRetry={posts.refresh} />;
  if (posts.data === null) return <Skeletons label="Opening the writing desk…" />;
  if (postId && !post) return <LoadFailure message="That post could not be found." onRetry={() => router.push("/content?tab=blog")} />;

  return <ArticleEditor kind="post" article={post ?? null} presentation="page" onClose={() => router.push("/content?tab=blog")} onSubmit={save} />;
}
