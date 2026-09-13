"use client";

import { useRouter } from "next/navigation";
import { ServiceForm } from "./ServiceForm";
import { LoadFailure, Skeletons } from "@/components/content/states";
import { ApiError } from "@/lib/api";
import { agreementsApi } from "@/lib/agreements";
import { servicesApi, type ServiceDraft } from "@/lib/services";
import { useAction, useResource } from "@/lib/use-resource";

export function ServiceEditorPage({ serviceId }: { serviceId?: string }) {
  const router = useRouter();
  const action = useAction();
  const data = useResource(
    async (session, callbacks) => {
      const [agreements, services] = await Promise.all([
        agreementsApi.list(session, callbacks),
        serviceId
          ? servicesApi.listAll(session, callbacks)
          : Promise.resolve([]),
      ]);
      return { agreements, services };
    },
    [serviceId],
  );
  const service = serviceId
    ? data.data?.services.find((item) => item.id === serviceId)
    : null;

  async function save(draft: ServiceDraft) {
    const saved = await action.run("service", (session, callbacks) =>
      service
        ? servicesApi.update(session, callbacks, service.id, draft)
        : servicesApi.create(session, callbacks, draft),
    );
    if (!saved)
      throw new ApiError(
        0,
        "save_failed",
        "The service could not be saved. Please try again.",
      );
  }

  if (data.error)
    return <LoadFailure message={data.error} onRetry={data.refresh} />;
  if (!data.data) return <Skeletons label="Opening service editor…" />;
  if (serviceId && !service)
    return (
      <LoadFailure
        message="This service was not found. It may have been deleted."
        onRetry={() => router.push("/services")}
      />
    );
  return (
    <>
      <ServiceForm
        key={serviceId ?? "new"}
        service={service ?? null}
        agreements={data.data.agreements}
        onClose={() => router.push("/services")}
        onSubmit={save}
      />
      {action.error ? (
        <p
          role="alert"
          className="sticky bottom-4 rounded-xl bg-danger-bg p-4 text-sm text-danger-ink"
        >
          {action.error}
        </p>
      ) : null}
    </>
  );
}
