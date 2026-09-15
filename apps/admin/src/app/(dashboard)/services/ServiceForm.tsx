"use client";

import { CircleAlert } from "lucide-react";
import { useEffect, useState, type FormEvent } from "react";
import { Button } from "@/components/ui/Button";
import { BrandedSelect } from "@/components/ui/ChoiceControls";
import { ImagePicker } from "@/components/content/ImagePicker";
import { AdminPageHeader } from "@/components/layout/AdminPageHeader";
import { Modal } from "@/components/ui/Modal";
import { TextArea } from "@/components/ui/TextArea";
import { TextInput } from "@/components/ui/TextInput";
import { ApiError } from "@/lib/api";
import { minorToMajorString, parseMajorToMinor } from "@/lib/format";
import type { Agreement } from "@/lib/agreements";
import type { Service, ServiceDraft } from "@/lib/services";

/**
 * Create/edit service form on a dedicated page (design-system §3.14 form width, §3.29
 * field wrapper). Custom validation only — the form is noValidate and errors
 * render below each field; API failures surface in a banner at the top.
 *
 * Price is edited in major units (US dollars) and converted to integer minor units on
 * submit; on edit the stored priceKobo is shown divided by 100.
 */

interface FieldErrors {
  name?: string;
  duration?: string;
  price?: string;
}

export function ServiceForm({
  service,
  agreements,
  onClose,
  onSubmit,
}: {
  /** null → create; a Service → edit (fields pre-filled). */
  service: Service | null;
  /** The practice's agreements, for the "requires" field. */
  agreements: Agreement[];
  onClose: () => void;
  /** Parent performs the API call and throws on failure. */
  onSubmit: (draft: ServiceDraft, service: Service | null) => Promise<void>;
}) {
  const editing = service !== null;
  const initialAgreementIds =
    service?.agreementIds && service.agreementIds.length > 0
      ? service.agreementIds
      : service?.agreementId
        ? [service.agreementId]
        : [];

  const initial = {
    name: service?.name ?? "",
    description: service?.description ?? "",
    imageUrl: service?.imageUrl ?? "",
    duration: service ? String(service.durationMinutes) : "",
    price: service ? minorToMajorString(service.priceKobo) : "",
    agreementIds: initialAgreementIds,
  };

  const [name, setName] = useState(initial.name);
  const [description, setDescription] = useState(initial.description);
  const [imageUrl, setImageUrl] = useState(initial.imageUrl);
  const [duration, setDuration] = useState(initial.duration);
  const [price, setPrice] = useState(initial.price);
  const [agreementIds, setAgreementIds] = useState<string[]>(initial.agreementIds);
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const dirty =
    name !== initial.name ||
    description !== initial.description ||
    imageUrl !== initial.imageUrl ||
    duration !== initial.duration ||
    price !== initial.price ||
    JSON.stringify(agreementIds) !== JSON.stringify(initial.agreementIds);

  const [confirmLeave, setConfirmLeave] = useState(false);
  useEffect(() => {
    if (!dirty || submitting) return;
    const warn = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty, submitting]);
  function leave() {
    if (dirty) setConfirmLeave(true);
    else onClose();
  }

  function validate(): boolean {
    const errors: FieldErrors = {};
    const trimmedName = name.trim();
    if (!trimmedName) {
      errors.name = "Give the service a name";
    } else if (trimmedName.length > 200) {
      errors.name = "Keep the name under 200 characters";
    }
    const durationMinutes = Number(duration.trim());
    if (
      !/^\d+$/.test(duration.trim()) ||
      durationMinutes < 5 ||
      durationMinutes > 480
    ) {
      errors.duration = "Enter a duration between 5 and 480 minutes";
    }
    if (parseMajorToMinor(price) === null) {
      errors.price = "Enter a price in US dollars, e.g. 250 or 250.50";
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
      imageUrl: imageUrl.trim(),
      durationMinutes: Number(duration.trim()),
      priceKobo: parseMajorToMinor(price)!,
      // Currency is fixed to USD for this US-based practice.
      currency: "USD",
      agreementId: agreementIds[0] ?? "",
      agreementIds: agreementIds,
    };

    setSubmitting(true);
    try {
      await onSubmit(draft, service);
      onClose();
    } catch (error) {
      setFormError(
        error instanceof ApiError
          ? error.message
          : "Something went wrong. Try again.",
      );
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div data-admin-page="service-editor" className="flex flex-col gap-6">
      <AdminPageHeader
        eyebrow="Practice menu"
        title={editing ? "Edit service" : "New service"}
        description={
          editing
            ? "Update the details clients see and use for future bookings."
            : "Build a service for your practice. It becomes available for booking when you save it."
        }
      />
      <div className="sticky top-0 z-20 flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-border bg-surface-raised p-4 shadow-sm">
        <Button variant="ghost" disabled={submitting} onClick={leave}>
          Back to services
        </Button>
        <Button type="submit" form="service-form" loading={submitting}>
          {editing ? "Save changes" : "Add service"}
        </Button>
      </div>
      {/* noValidate: native validation bubbles are forbidden — errors are custom per §3.29 */}
      <form
        id="service-form"
        noValidate
        onSubmit={handleSubmit}
        className="grid items-start gap-6 lg:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)]"
      >
        {formError ? (
          <div
            role="alert"
            className="lg:col-span-2 flex items-start gap-2 rounded-md bg-danger-bg px-4 py-3 text-sm leading-[1.55] text-danger-ink"
          >
            <CircleAlert
              size={16}
              aria-hidden="true"
              className="mt-0.5 shrink-0"
            />
            {formError}
          </div>
        ) : null}

        <section
          aria-labelledby="service-details-heading"
          className="min-w-0 rounded-2xl border border-border bg-surface-raised p-5 sm:p-7 flex flex-col gap-5"
        >
          <h2
            id="service-details-heading"
            className="text-lg font-semibold text-ink"
          >
            Service details
          </h2>
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
          <ImagePicker
            label="Service image"
            value={imageUrl}
            disabled={submitting}
            onChange={setImageUrl}
          />
        </section>
        <section
          aria-labelledby="service-booking-heading"
          className="min-w-0 rounded-2xl border border-border bg-surface-raised p-5 sm:p-7 flex flex-col gap-5"
        >
          <h2
            id="service-booking-heading"
            className="text-lg font-semibold text-ink"
          >
            Booking details
          </h2>
          {/* Agreements required for this service: supports multiple documents. */}
          <div className="flex flex-col gap-2">
            <span className="text-sm font-medium text-ink">
              Required Agreements (Multi-document selection)
            </span>
            <div className="flex flex-col gap-2 rounded-xl border border-border p-3 bg-surface-sunken">
              {agreements.length === 0 ? (
                <span className="text-xs text-ink-muted">No agreements created yet.</span>
              ) : (
                agreements
                  .filter((item) => item.active || agreementIds.includes(item.id))
                  .map((item) => {
                    const checked = agreementIds.includes(item.id);
                    return (
                      <label
                        key={item.id}
                        className="flex items-center gap-2.5 text-sm text-ink cursor-pointer hover:text-primary transition-colors"
                      >
                        <input
                          type="checkbox"
                          checked={checked}
                          onChange={(e) => {
                            if (e.target.checked) {
                              setAgreementIds((prev) => [...prev, item.id]);
                            } else {
                              setAgreementIds((prev) =>
                                prev.filter((id) => id !== item.id),
                              );
                            }
                          }}
                          className="rounded border-border text-primary focus:ring-primary size-4"
                        />
                        <span>
                          {item.title}{" "}
                          {!item.active ? (
                            <span className="text-xs text-ink-faint">(retired)</span>
                          ) : null}
                        </span>
                      </label>
                    );
                  })
              )}
            </div>
            <span className="text-xs leading-relaxed text-ink-muted">
              Clients will sequentially read and sign all selected agreements before their first booking.
            </span>
          </div>
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
                setFieldErrors((errors) => ({
                  ...errors,
                  duration: undefined,
                }));
              }}
            />
            <TextInput
              label="Price"
              required
              inputMode="decimal"
              placeholder="250"
              hint="In US dollars ($). Enter 0 for a free session."
              value={price}
              error={fieldErrors.price}
              onChange={(event) => {
                setPrice(event.target.value);
                setFieldErrors((errors) => ({ ...errors, price: undefined }));
              }}
            />
          </div>
        </section>
      </form>
      <Modal
        open={confirmLeave}
        onClose={() => setConfirmLeave(false)}
        title="Discard unsaved changes?"
        footer={
          <>
            <Button variant="ghost" onClick={() => setConfirmLeave(false)}>
              Keep editing
            </Button>
            <Button onClick={onClose}>Discard changes</Button>
          </>
        }
      >
        <p>Your service has not been saved yet.</p>
      </Modal>
    </div>
  );
}
