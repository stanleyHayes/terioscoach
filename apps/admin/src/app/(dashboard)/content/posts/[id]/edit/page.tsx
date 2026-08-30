import { PostEditorPage } from "@/components/content/PostEditorPage";

export default async function EditPostPage({ params }: PageProps<"/content/posts/[id]/edit">) {
  const { id } = await params;
  return <PostEditorPage postId={id} />;
}
