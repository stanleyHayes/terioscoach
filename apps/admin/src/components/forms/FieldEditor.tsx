"use client";

import { ChevronDown, GripVertical, Plus, Trash2 } from "lucide-react";
import { useEffect, useId, useRef, useState } from "react";
import { Button } from "@/components/ui/Button";
import { IconButton } from "@/components/ui/IconButton";
import { Switch } from "@/components/ui/Switch";
import { TextInput } from "@/components/ui/TextInput";
import { cn } from "@/lib/cn";
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

/**
 * A custom listbox for the field type.
 *
 * Written out rather than reached for from a library because the app has no
 * form library and native `<select>` is forbidden. It implements the parts
 * that matter: a labelled button, `role="listbox"` with `aria-activedescendant`,
 * arrow/Home/End/Escape keys, and a click-away that closes.
 */
function TypeListbox({
  value,
  onChange,
}: {
  value: FieldType;
  onChange: (type: FieldType) => void;
}) {
  const labelId = useId();
  const listId = useId();
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(() => FIELD_TYPES.findIndex((t) => t.value === value));
  const containerRef = useRef<HTMLDivElement>(null);

  const selected = FIELD_TYPES.find((t) => t.value === value) ?? FIELD_TYPES[0]!;

  useEffect(() => {
    if (!open) return;
    function handleClick(event: MouseEvent) {
      if (!containerRef.current?.contains(event.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [open]);

  function choose(index: number) {
    const option = FIELD_TYPES[index];
    if (!option) return;
    onChange(option.value);
    setActive(index);
    setOpen(false);
  }

  function handleKeyDown(event: React.KeyboardEvent) {
    switch (event.key) {
      case "ArrowDown":
      case "ArrowUp": {
        event.preventDefault();
        if (!open) {
          setOpen(true);
          return;
        }
        const step = event.key === "ArrowDown" ? 1 : -1;
        setActive((current) => (current + step + FIELD_TYPES.length) % FIELD_TYPES.length);
        return;
      }
      case "Home":
        if (open) {
          event.preventDefault();
          setActive(0);
        }
        return;
      case "End":
        if (open) {
          event.preventDefault();
          setActive(FIELD_TYPES.length - 1);
        }
        return;
      case "Enter":
      case " ":
        event.preventDefault();
        if (open) choose(active);
        else setOpen(true);
        return;
      case "Escape":
        if (open) {
          event.preventDefault();
          setOpen(false);
        }
        return;
      default:
    }
  }

  return (
    <div ref={containerRef} className="flex flex-col gap-1.5">
      <span id={labelId} className="text-sm font-medium text-ink">
        Answer type
      </span>
      <div className="relative">
        <button
          type="button"
          role="combobox"
          aria-haspopup="listbox"
          aria-expanded={open}
          aria-controls={listId}
          aria-labelledby={labelId}
          aria-activedescendant={open ? `${listId}-${active}` : undefined}
          onClick={() => setOpen((current) => !current)}
          onKeyDown={handleKeyDown}
          className="flex h-10 w-full items-center justify-between gap-2 rounded-md border border-border bg-surface px-3 text-left text-sm text-ink transition-colors duration-instant ease-out hover:border-border-strong focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
        >
          <span>{selected.label}</span>
          <ChevronDown
            size={16}
            aria-hidden="true"
            className={cn("shrink-0 text-ink-faint transition-transform", open && "rotate-180")}
          />
        </button>

        {open ? (
          <ul
            role="listbox"
            id={listId}
            aria-labelledby={labelId}
            className="absolute z-dropdown mt-1 w-full overflow-hidden rounded-md border border-border bg-surface-raised py-1 shadow-lg"
          >
            {FIELD_TYPES.map((option, index) => (
              <li
                key={option.value}
                role="option"
                id={`${listId}-${index}`}
                aria-selected={option.value === value}
                onMouseEnter={() => setActive(index)}
                onClick={() => choose(index)}
                className={cn(
                  "cursor-pointer px-3 py-2",
                  index === active ? "bg-surface-sunken" : "",
                )}
              >
                <span className="block text-sm text-ink">{option.label}</span>
                <span className="block text-[12px] text-ink-muted">{option.hint}</span>
              </li>
            ))}
          </ul>
        ) : null}
      </div>
    </div>
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
