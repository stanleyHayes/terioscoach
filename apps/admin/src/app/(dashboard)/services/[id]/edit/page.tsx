import { ServiceEditorPage } from "../../ServiceEditorPage";
export default async function EditServicePage({ params }: { params: Promise<{ id: string }> }) {
  return <ServiceEditorPage serviceId={(await params).id} />;
}
