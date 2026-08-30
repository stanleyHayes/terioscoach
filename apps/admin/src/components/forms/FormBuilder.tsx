"use client";

import { CircleAlert, ListPlus, Plus } from "lucide-react";
import { EmptyState } from "@/components/content/states";
import { useState, type FormEvent } from "react";
import { FieldEditor } from "@/components/forms/FieldEditor";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { TextArea } from "@/components/ui/TextArea";
import { TextInput } from "@/components/ui/TextInput";
import { ApiError } from "@/lib/api";
import {
  fieldProblem,
  uniqueFieldKey,
  type FormDefinition,
  type FormDraft,
  type FormField,
} from "@/lib/forms";

/**
 * The form builder (ADM-08).
 *
 * Editing an existing form edits the definition, not the submissions
 * already made against it. A submission stores the answers it was given
 * and is read back against the definition as it was — so a question
 * reworded today does not silently rewrite what a client agreed to last
 * month. The warning below says so, because it is not obvious.
 */

function newField(taken: string[]): FormField {
  return {
    key: uniqueFieldKey("question", taken),
    label: "",
    type: "text",
    required: false,
    options: [],
  };
}

export function FormBuilder({
  form,
  hasSubmissions,
  onClose,
  onSubmit,
}: {
  form: FormDefinition | null;
  /** Whether anyone has already filled this in — changes the warning shown. */
  hasSubmissions: boolean;
  onClose: () => void;
  onSubmit: (draft: FormDraft) => Promise<void>;
}) {
  const editing = form !== null;
  const initial = {
    title: form?.title ?? "",
    description: form?.description ?? "",
    fields: form?.fields ?? [],
  };

  const [title, setTitle] = useState(initial.title);
  const [description, setDescription] = useState(initial.description);
  const [fields, setFields] = useState<FormField[]>(initial.fields);
  const [showErrors, setShowErrors] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const dirty =
    title !== initial.title ||
    description !== initial.description ||
    JSON.stringify(fields) !== JSON.stringify(initial.fields);

  const titleProblem =
    showErrors && !title.trim() ? "Give the form a title" : undefined;
  const fieldsProblem =
    showErrors && fields.length === 0
      ? "A form needs at least one question"
      : undefined;

  function addField() {
    setFields((current) => [...current, newField(current.map((f) => f.key))]);
  }

  function changeField(index: number, next: FormField) {
    setFields((current) =>
      current.map((field, i) => {
        if (i !== index) return field;
        // The key is assigned once and kept. An answer is stored under it,
        // so rewording a label must not move where the answer lives.
        return { ...next, key: field.key };
      }),
    );
  }

  function moveField(index: number, direction: -1 | 1) {
    setFields((current) => {
      const target = index + direction;
      if (target < 0 || target >= current.length) return current;
      const next = [...current];
      [next[index], next[target]] = [next[target]!, next[index]!];
      return next;
    });
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setShowErrors(true);
    setFormError(null);

    if (
      !title.trim() ||
      fields.length === 0 ||
      fields.some((f) => fieldProblem(f))
    )
      return;

    setSubmitting(true);
    try {
      await onSubmit({
        title: title.trim(),
        description: description.trim() || undefined,
        fields: fields.map((field) => ({
          ...field,
          key: field.key || uniqueFieldKey(field.label, []),
          label: field.label.trim(),
          helpText: field.helpText?.trim() || undefined,
          options: field.options.map((o) => o.trim()).filter(Boolean),
        })),
      });
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
    <Modal
      open
      onClose={onClose}
      title={editing ? "Edit form" : "New form"}
      size="form"
      dirty={dirty}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" form="form-builder" loading={submitting}>
            {editing ? "Save changes" : "Create form"}
          </Button>
        </>
      }
    >
      <form
        id="form-builder"
        noValidate
        onSubmit={handleSubmit}
        className="flex flex-col gap-5"
      >
        {formError ? (
          <div
            role="alert"
            className="flex items-start gap-2 rounded-md bg-danger-bg px-4 py-3 text-sm leading-[1.55] text-danger-ink"
          >
            <CircleAlert
              size={16}
              aria-hidden="true"
              className="mt-0.5 shrink-0"
            />
            {formError}
          </div>
        ) : null}

        {editing && hasSubmissions ? (
          <p className="rounded-md bg-info-bg px-4 py-3 text-[13px] leading-[1.55] text-info-ink">
            Forms already signed keep the wording they were signed under. Your
            changes apply to the next person who fills this in.
          </p>
        ) : null}

        <TextInput
          label="Title"
          required
          data-autofocus
          value={title}
          error={titleProblem}
          placeholder="Health intake"
          onChange={(event) => setTitle(event.target.value)}
        />
        <TextArea
          label="Description"
          rows={2}
          value={description}
          hint="Shown at the top of the form. What it's for, and why you're asking."
          onChange={(event) => setDescription(event.target.value)}
        />

        <div className="flex flex-col gap-3">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-medium text-ink">
              Questions{fields.length > 0 ? ` (${fields.length})` : ""}
            </h3>
            <Button variant="secondary" size="sm" onClick={addField}>
              <Plus size={14} aria-hidden="true" className="mr-1.5" />
              Add question
            </Button>
          </div>

          {fieldsProblem ? (
            <p role="alert" className="text-[13px] text-danger-ink">
              {fieldsProblem}
            </p>
          ) : null}

          {fields.length > 0 ? (
            <ul className="flex flex-col gap-3">
              {fields.map((field, index) => (
                <FieldEditor
                  key={field.key}
                  field={field}
                  // A blank question that has only just been added isn't a
                  // mistake yet; problems appear once Save has been tried.
                  showErrors={showErrors}
                  index={index}
                  count={fields.length}
                  onChange={(next) => changeField(index, next)}
                  onMove={(direction) => moveField(index, direction)}
                  onRemove={() =>
                    setFields((current) =>
                      current.filter((_, i) => i !== index),
                    )
                  }
                />
              ))}
            </ul>
          ) : (
            <EmptyState
              compact
              icon={<ListPlus size={24} />}
              title="No questions yet"
              body="Use the Add question button above to begin building this form."
            />
          )}
        </div>
      </form>
    </Modal>
  );
}
