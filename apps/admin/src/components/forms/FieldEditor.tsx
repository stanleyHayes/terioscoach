"use client";

import { ChevronDown, GripVertical, Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { BrandedSelect } from "@/components/ui/ChoiceControls";
import { IconButton } from "@/components/ui/IconButton";
import { Switch } from "@/components/ui/Switch";
import { TextInput } from "@/components/ui/TextInput";
import { CHOICE_TYPES, FIELD_TYPES, fieldProblem, type FieldType, type FormField } from "@/lib/forms";

/**
 * One question in the form builder (ADM-08).
 *
 * The type control is a custom listbox, not a `<select>` — native selects
 * are forbidden throughout, and this one has to show a hint per option
 * anyway, which a native option cannot do.
 */
export function FieldEditor({
  field,
  index,
  count,
  showErrors = false,
  onChange,
  onMove,
  onRemove,
}: {
  field: FormField;
  index: number;
  count: number;
  /** Held back until the builder has been submitted once — a question added
   * a second ago is not yet a mistake. */
  showErrors?: boolean;
  onChange: (next: FormField) => void;
  onMove: (direction: -1 | 1) => void;
  onRemove: () => void;
}) {
  const problem = showErrors ? fieldProblem(field) : null;
  const choices = CHOICE_TYPES.includes(field.type);

  function set<K extends keyof FormField>(key: K, value: FormField[K]) {
    onChange({ ...field, [key]: value });
  }

  function setType(type: FieldType) {
    // Options only exist on choice fields, and the server rejects them
    // anywhere else — so switching away from a choice type drops them
    // rather than carrying dead data the save would fail on.
    onChange({
      ...field,
      type,
      options: CHOICE_TYPES.includes(type) ? (field.options.length ? field.options : [""]) : [],
      // A signature field is the act of signing, not a question with an
      // answer to skip; it is always required.
      required: type === "signature" ? true : field.required,
    });
  }

  return (
    <li className="rounded-lg border border-border bg-surface-raised p-4">
      <div className="flex items-start gap-3">
        <div className="flex shrink-0 flex-col items-center gap-1 pt-2.5">
          <GripVertical size={16} aria-hidden="true" className="text-ink-faint" />
          <span className="text-[11px] tabular-nums text-ink-faint">{index + 1}</span>
        </div>

        <div className="flex min-w-0 flex-1 flex-col gap-3">
          <TextInput
            label={`Question ${index + 1}`}
            value={field.label}
            error={problem === "Every question needs a label" ? problem : undefined}
            placeholder="Do you have any allergies?"
            onChange={(event) => set("label", event.target.value)}
          />

          <TypeListbox value={field.type} onChange={setType} />

          <TextInput
            label="Help text"
            value={field.helpText ?? ""}
            hint="Optional. Shown under the question."
            onChange={(event) => set("helpText", event.target.value)}
          />

          {choices ? (
            <OptionsEditor
              options={field.options}
              error={problem === "Add at least one option" ? problem : undefined}
              onChange={(options) => set("options", options)}
            />
          ) : null}

          <div className="flex flex-wrap items-center gap-4">
            <Switch
              checked={field.required}
              disabled={field.type === "signature"}
              label={`"${field.label || `Question ${index + 1}`}" must be answered`}
              onChange={(next) => set("required", next)}
            />
            {field.type === "signature" ? (
              <span className="text-[13px] text-ink-faint">A signature is always required.</span>
            ) : null}
          </div>
        </div>

        <div className="flex shrink-0 flex-col gap-1">
          <IconButton
            size="sm"
            aria-label={`Move question ${index + 1} up`}
            disabled={index === 0}
            onClick={() => onMove(-1)}
          >
            <ChevronDown aria-hidden="true" className="rotate-180" />
          </IconButton>
          <IconButton
            size="sm"
            aria-label={`Move question ${index + 1} down`}
            disabled={index === count - 1}
            onClick={() => onMove(1)}
          >
            <ChevronDown aria-hidden="true" />
          </IconButton>
          <IconButton
            size="sm"
            aria-label={`Remove question ${index + 1}`}
            onClick={onRemove}
          >
            <Trash2 aria-hidden="true" />
          </IconButton>
        </div>
      </div>
    </li>
  );
}

function TypeListbox({
  value,
  onChange,
}: {
  value: FieldType;
  onChange: (type: FieldType) => void;
}) {
  return (
    <BrandedSelect
      label="Answer type"
      value={value}
      onChange={(next) => onChange(next as FieldType)}
      options={FIELD_TYPES.map((option) => ({
        value: option.value,
        label: option.label,
        description: option.hint,
      }))}
    />
  );
}

function OptionsEditor({
  options,
  error,
  onChange,
}: {
  options: string[];
  error?: string;
  onChange: (options: string[]) => void;
}) {
  const list = options.length > 0 ? options : [""];

  return (
    <fieldset className="flex flex-col gap-2 rounded-md border border-border p-3">
      <legend className="px-1.5 text-[13px] font-medium text-ink-muted">Options</legend>
      {list.map((option, index) => (
        <div key={index} className="flex items-center gap-2">
          <TextInput
            label={`Option ${index + 1}`}
            labelHidden
            size="sm"
            value={option}
            placeholder={`Option ${index + 1}`}
            wrapperClassName="flex-1"
            onChange={(event) => {
              const next = [...list];
              next[index] = event.target.value;
              onChange(next);
            }}
          />
          <IconButton
            size="sm"
            aria-label={`Remove option ${index + 1}`}
            disabled={list.length === 1}
            onClick={() => onChange(list.filter((_, i) => i !== index))}
          >
            <Trash2 aria-hidden="true" />
          </IconButton>
        </div>
      ))}
      {error ? (
        <p role="alert" className="text-[13px] text-danger-ink">
          {error}
        </p>
      ) : null}
      <div>
        <Button variant="ghost" size="sm" onClick={() => onChange([...list, ""])}>
          <Plus size={14} aria-hidden="true" className="mr-1.5" />
          Add option
        </Button>
      </div>
    </fieldset>
  );
}
