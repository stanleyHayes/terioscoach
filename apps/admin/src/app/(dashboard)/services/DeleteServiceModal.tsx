"use client";

import { CircleAlert } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { ApiError } from "@/lib/api";
import type { Service } from "@/lib/services";

/**
 * Delete confirmation (design-system §3.14 destructive variant): names the
 * service, states the consequence (past bookings keep their record — the API
 * soft-deletes when bookings exist), danger Button, initial focus on Cancel.
 */
export function DeleteServiceModal({
  service,
  onClose,
  onConfirm,
}: {
  service: Service;
  onClose: () => void;
  /** Parent performs the API call and throws on failure. */
  onConfirm: (service: Service) => Promise<void>;
}) {
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleConfirm() {
    setDeleting(true);
    setError(null);
    try {
      await onConfirm(service);
      onClose();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Something went wrong. Try again.");
    } finally {
      setDeleting(false);
    }
  }

  return (
    <Modal
      open
      onClose={onClose}
      title="Delete this service?"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} data-autofocus>
            Cancel
          </Button>
          <Button variant="danger" loading={deleting} onClick={handleConfirm}>
            Delete service
          </Button>
        </>
      }
    >
      <p className="text-base leading-[1.6] text-ink">
        <span className="font-semibold">{service.name}</span> will be removed from your
        catalog and can no longer be booked. Past bookings keep their record of this
        service.
      </p>
      {error ? (
        <div
          role="alert"
          className="mt-4 flex items-start gap-2 rounded-md bg-danger-bg px-4 py-3 text-sm leading-[1.55] text-danger-ink"
        >
          <CircleAlert size={16} aria-hidden="true" className="mt-0.5 shrink-0" />
          {error}
        </div>
      ) : null}
    </Modal>
  );
}
