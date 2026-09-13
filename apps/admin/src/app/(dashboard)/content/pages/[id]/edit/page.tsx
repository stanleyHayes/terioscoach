import { PageEditorPage } from "@/components/content/PageEditorPage";

export default async function EditPage({ params }: PageProps<"/content/pages/[id]/edit">) { const { id } = await params; return <PageEditorPage pageId={id} />; }
