"use client";

import { CircleAlert } from "lucide-react";
import { useState, type FormEvent } from "react";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { TextArea } from "@/components/ui/TextArea";
import { TextInput } from "@/components/ui/TextInput";
import { ApiError } from "@/lib/api";
import { minorToMajorString, parseMajorToMinor } from "@/lib/format";
import type { Service, ServiceDraft } from "@/lib/services";

/**
 * Create/edit service form in a Modal (design-system §3.14 form width, §3.29
 * field wrapper). Custom validation only — the form is noValidate and errors
 * render below each field; API failures surface in a banner at the top.
 *
 * Price is edited in major units (cedis) and converted to integer kobo on
 * submit; on edit the stored priceKobo is shown divided by 100.
 */

interface FieldErrors {
  name?: string;
  duration?: string;
  price?: string;
}

export function ServiceFormModal({
  service,
  onClose,
  onSubmit,
}: {
  /** null → create; a Service → edit (fields pre-filled). */
  service: Service | null;
  onClose: () => void;
  /** Parent performs the API call and throws on failure. */
  onSubmit: (draft: ServiceDraft, service: Service | null) => Promise<void>;
}) {
  const editing = service !== null;
  const initial = {
    name: service?.name ?? "",
    description: service?.description ?? "",
    duration: service ? String(service.durationMinutes) : "",
    price: service ? minorToMajorString(service.priceKobo) : "",
  };

  const [name, setName] = useState(initial.name);
  const [description, setDescription] = useState(initial.description);
  const [duration, setDuration] = useState(initial.duration);
  const [price, setPrice] = useState(initial.price);
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const dirty =
    name !== initial.name ||
    description !== initial.description ||
    duration !== initial.duration ||
    price !== initial.price;

  function validate(): boolean {
    const errors: FieldErrors = {};
    const trimmedName = name.trim();
    if (!trimmedName) {
      errors.name = "Give the service a name";
    } else if (trimmedName.length > 200) {
      errors.name = "Keep the name under 200 characters";
    }
    const durationMinutes = Number(duration.trim());
    if (!/^\d+$/.test(duration.trim()) || durationMinutes < 5 || durationMinutes > 480) {
      errors.duration = "Enter a duration between 5 and 480 minutes";
    }
    if (parseMajorToMinor(price) === null) {
      errors.price = "Enter a price in cedis, e.g. 250 or 250.50";
    }
    setFieldErrors(errors);
    return Object.keys(errors).length === 0;
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setFormError(null);
    if (!validate()) return;

    const draft: ServiceDraft = {
      name: name.trim(),
      description: description.trim(),
      durationMinutes: Number(duration.trim()),
      priceKobo: parseMajorToMinor(price)!,
      // Currency is fixed to GHS in v1; on edit the stored currency is kept.
      ...(editing ? {} : { currency: "GHS" }),
    };

    setSubmitting(true);
    try {
      await onSubmit(draft, service);
      onClose();
    } catch (error) {
      setFormError(
        error instanceof ApiError ? error.message : "Something went wrong. Try again.",
      );
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open
      onClose={onClose}
      title={editing ? "Edit service" : "New service"}
      description={
        editing
          ? "Changes apply to future bookings."
          : "It appears on your booking page once it's active."
      }
      size="form"
      dirty={dirty}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" form="service-form" loading={submitting}>
            {editing ? "Save changes" : "Add service"}
          </Button>
        </>
      }
    >
      {/* noValidate: native validation bubbles are forbidden — errors are custom per §3.29 */}
      <form id="service-form" noValidate onSubmit={handleSubmit} className="flex flex-col gap-4">
        {formError ? (
          <div
            role="alert"
            className="flex items-start gap-2 rounded-md bg-danger-bg px-4 py-3 text-sm leading-[1.55] text-danger-ink"
          >
            <CircleAlert size={16} aria-hidden="true" className="mt-0.5 shrink-0" />
            {formError}
          </div>
        ) : null}

        <TextInput
          label="Name"
          required
          placeholder="Aromatherapy massage"
          value={name}
          error={fieldErrors.name}
          data-autofocus
          onChange={(event) => {
            setName(event.target.value);
            setFieldErrors((errors) => ({ ...errors, name: undefined }));
          }}
        />
        <TextArea
          label="Description"
          placeholder="What the client gets, in a sentence or two"
          value={description}
          onChange={(event) => setDescription(event.target.value)}
        />
        <div className="grid grid-cols-2 gap-4">
          <TextInput
            label="Duration (minutes)"
            required
            type="number"
            inputMode="numeric"
            min={5}
            max={480}
            placeholder="60"
            value={duration}
            error={fieldErrors.duration}
            onChange={(event) => {
              setDuration(event.target.value);
              setFieldErrors((errors) => ({ ...errors, duration: undefined }));
            }}
          />
          <TextInput
            label="Price"
            required
            inputMode="decimal"
            placeholder="250"
            hint="In cedis (GH₵) — 250.50 is stored as 25050 kobo."
            value={price}
            error={fieldErrors.price}
            onChange={(event) => {
              setPrice(event.target.value);
              setFieldErrors((errors) => ({ ...errors, price: undefined }));
            }}
          />
        </div>
      </form>
    </Modal>
  );
}
