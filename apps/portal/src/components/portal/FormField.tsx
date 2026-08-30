"use client";

import { Check, ChevronDown } from "lucide-react";
import { useEffect, useId, useRef, useState } from "react";
import { TextInput } from "@/components/ui/TextInput";
import { cn } from "@/lib/cn";
import type { Answer, FormField as Field } from "@/lib/portal";

/**
 * One form field, rendered from the practice's own definition (CX-07).
 *
 * Every control here is built rather than borrowed — no native `<select>`,
 * no `<input type="date">`, no default checkbox or radio. That is the
 * platform rule, and it is also the only way these look like the rest of
 * the practice on every browser a client might arrive with.
 */
export interface FormFieldProps {
  field: Field;
  answer: Answer;
  error?: string;
  disabled?: boolean;
  onChange: (answer: Answer) => void;
}

export function FormField({ field, answer, error, disabled, onChange }: FormFieldProps) {
  switch (field.type) {
    case "textarea":
      return (
        <FieldShell field={field} error={error}>
          {(id, describedBy) => (
            <textarea
              id={id}
              rows={5}
              value={answer.value ?? ""}
              disabled={disabled}
              aria-describedby={describedBy}
              aria-invalid={error ? true : undefined}
              onChange={(event) => onChange({ value: event.target.value })}
              className={controlClasses(error, "min-h-[120px] py-2.5")}
            />
          )}
        </FieldShell>
      );

    case "select":
    case "radio":
      return (
        <ChoiceField
          field={field}
          answer={answer}
          error={error}
          disabled={disabled}
          onChange={onChange}
        />
      );

    case "checkbox":
      return (
        <CheckboxField
          field={field}
          answer={answer}
          error={error}
          disabled={disabled}
          onChange={onChange}
        />
      );

    case "number":
    case "date":
    case "text":
    default:
      return (
        <TextInput
          label={field.label}
          hint={field.helpText}
          error={error}
          required={field.required}
          disabled={disabled}
          // A date field asks for YYYY-MM-DD as text: the native date
          // picker is browser chrome we do not control, and the API wants
          // exactly this format anyway.
          inputMode={field.type === "number" ? "decimal" : undefined}
          placeholder={field.type === "date" ? "YYYY-MM-DD" : undefined}
          value={answer.value ?? ""}
          onChange={(event) => onChange({ value: event.target.value })}
        />
      );
  }
}

/** Label → control → hint/error wrapper, matching TextInput's anatomy so a
 * mixed form lines up. */
function FieldShell({
  field,
  error,
  children,
}: {
  field: Field;
  error?: string;
  children: (id: string, describedBy: string | undefined) => React.ReactNode;
}) {
  const id = useId();
  const hintId = `${id}-hint`;
  const errorId = `${id}-error`;
  const describedBy = error ? errorId : field.helpText ? hintId : undefined;

  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-sm font-medium tracking-[0.005em] text-ink">
        {field.label}
        {field.required ? (
          <>
            <span aria-hidden="true" className="text-accent"> *</span>
            <span className="sr-only"> (required)</span>
          </>
        ) : null}
      </label>
      {children(id, describedBy)}
      {error ? (
        <p id={errorId} role="alert" className="text-[13px] text-danger-ink">
          {error}
        </p>
      ) : field.helpText ? (
        <p id={hintId} className="text-[13px] text-ink-muted">
          {field.helpText}
        </p>
      ) : null}
    </div>
  );
}

/** A single-choice field. Short lists render as radio cards; longer ones as
 * a custom listbox, because twelve stacked cards is a worse experience than
 * a menu — and neither is a native `<select>`. */
function ChoiceField({ field, answer, error, disabled, onChange }: FormFieldProps) {
  const asMenu = field.type === "select" && field.options.length > 4;
  return asMenu ? (
    <SelectField field={field} answer={answer} error={error} disabled={disabled} onChange={onChange} />
  ) : (
    <fieldset className="border-0 p-0" disabled={disabled}>
      <legend className="text-sm font-medium tracking-[0.005em] text-ink">
        {field.label}
        {field.required ? (
          <>
            <span aria-hidden="true" className="text-accent"> *</span>
            <span className="sr-only"> (required)</span>
          </>
        ) : null}
      </legend>
      {field.helpText ? (
        <p className="mt-1 text-[13px] text-ink-muted">{field.helpText}</p>
      ) : null}

      <div className="mt-2 flex flex-col gap-2">
        {field.options.map((option) => {
          const selected = answer.value === option;
          return (
            <button
              key={option}
              type="button"
              role="radio"
              aria-checked={selected}
              disabled={disabled}
              onClick={() => onChange({ value: option })}
              className={cn(
                "flex items-center gap-3 rounded-lg border px-4 py-3 text-left text-base transition-colors duration-instant ease-out",
                selected
                  ? "border-primary bg-primary/5 text-ink"
                  : "border-border bg-surface-raised text-ink hover:border-border-strong",
                disabled && "opacity-50",
              )}
            >
              <span
                aria-hidden="true"
                className={cn(
                  "flex size-5 shrink-0 items-center justify-center rounded-full border-2",
                  selected ? "border-primary" : "border-border-strong",
                )}
              >
                {selected ? <span className="size-2.5 rounded-full bg-primary" /> : null}
              </span>
              {option}
            </button>
          );
        })}
      </div>

      {error ? (
        <p role="alert" className="mt-1.5 text-[13px] text-danger-ink">
          {error}
        </p>
      ) : null}
    </fieldset>
  );
}

/** Custom listbox for longer choice lists. */
function SelectField({ field, answer, error, disabled, onChange }: FormFieldProps) {
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const id = useId();

  // Clicking away closes the menu — the behaviour a native select gives
  // free, and the one thing people notice immediately when it is missing.
  useEffect(() => {
    if (!open) return;
    function onPointerDown(event: PointerEvent) {
      if (!containerRef.current?.contains(event.target as Node)) setOpen(false);
    }
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") setOpen(false);
    }
    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [open]);

  return (
    <div className="flex flex-col gap-1.5" ref={containerRef}>
      <span id={`${id}-label`} className="text-sm font-medium tracking-[0.005em] text-ink">
        {field.label}
        {field.required ? (
          <>
            <span aria-hidden="true" className="text-accent"> *</span>
            <span className="sr-only"> (required)</span>
          </>
        ) : null}
      </span>

      <div className="relative">
        <button
          type="button"
          aria-haspopup="listbox"
          aria-expanded={open}
          aria-labelledby={`${id}-label`}
          disabled={disabled}
          onClick={() => setOpen((current) => !current)}
          className={cn(controlClasses(error, "flex items-center justify-between text-left"))}
        >
          <span className={answer.value ? "text-ink" : "text-ink-faint"}>
            {answer.value || "Choose one"}
          </span>
          <ChevronDown size={16} aria-hidden="true" className="text-ink-faint" />
        </button>

        {open ? (
          <ul
            role="listbox"
            aria-labelledby={`${id}-label`}
            className="absolute z-10 mt-1 max-h-60 w-full overflow-auto rounded-lg border border-border bg-surface-raised py-1 shadow-lg"
          >
            {field.options.map((option) => (
              <li key={option}>
                <button
                  type="button"
                  role="option"
                  aria-selected={answer.value === option}
                  onClick={() => {
                    onChange({ value: option });
                    setOpen(false);
                  }}
                  className="flex w-full items-center justify-between gap-3 px-4 py-2.5 text-left text-base text-ink transition-colors duration-instant ease-out hover:bg-surface-sunken"
                >
                  {option}
                  {answer.value === option ? (
                    <Check size={16} aria-hidden="true" className="text-primary" />
                  ) : null}
                </button>
              </li>
            ))}
          </ul>
        ) : null}
      </div>

      {error ? (
        <p role="alert" className="text-[13px] text-danger-ink">
          {error}
        </p>
      ) : field.helpText ? (
        <p className="text-[13px] text-ink-muted">{field.helpText}</p>
      ) : null}
    </div>
  );
}

/** A checkbox group when the field has options, a single toggle otherwise. */
function CheckboxField({ field, answer, error, disabled, onChange }: FormFieldProps) {
  const values = answer.values ?? [];
  const single = field.options.length === 0;
  const checked = single ? answer.value === "true" : false;

  function toggle(option: string) {
    const next = values.includes(option)
      ? values.filter((value) => value !== option)
      : [...values, option];
    onChange({ values: next });
  }

  return (
    <fieldset className="border-0 p-0" disabled={disabled}>
      <legend className="text-sm font-medium tracking-[0.005em] text-ink">
        {field.label}
        {field.required ? (
          <>
            <span aria-hidden="true" className="text-accent"> *</span>
            <span className="sr-only"> (required)</span>
          </>
        ) : null}
      </legend>
      {field.helpText ? (
        <p className="mt-1 text-[13px] text-ink-muted">{field.helpText}</p>
      ) : null}

      <div className="mt-2 flex flex-col gap-2">
        {single ? (
          <Box
            label={field.label}
            checked={checked}
            disabled={disabled}
            onToggle={() => onChange({ value: checked ? "false" : "true" })}
          />
        ) : (
          field.options.map((option) => (
            <Box
              key={option}
              label={option}
              checked={values.includes(option)}
              disabled={disabled}
              onToggle={() => toggle(option)}
            />
          ))
        )}
      </div>

      {error ? (
        <p role="alert" className="mt-1.5 text-[13px] text-danger-ink">
          {error}
        </p>
      ) : null}
    </fieldset>
  );
}

function Box({
  label,
  checked,
  disabled,
  onToggle,
}: {
  label: string;
  checked: boolean;
  disabled?: boolean;
  onToggle: () => void;
}) {
  return (
    <button
      type="button"
      role="checkbox"
      aria-checked={checked}
      disabled={disabled}
      onClick={onToggle}
      className={cn(
        "flex items-center gap-3 rounded-lg border px-4 py-3 text-left text-base transition-colors duration-instant ease-out",
        checked
          ? "border-primary bg-primary/5 text-ink"
          : "border-border bg-surface-raised text-ink hover:border-border-strong",
        disabled && "opacity-50",
      )}
    >
      <span
        aria-hidden="true"
        className={cn(
          "flex size-5 shrink-0 items-center justify-center rounded-[5px] border-2",
          checked ? "border-primary bg-primary text-on-primary" : "border-border-strong",
        )}
      >
        {checked ? <Check size={12} strokeWidth={3} /> : null}
      </span>
      {label}
    </button>
  );
}

function controlClasses(error: string | undefined, extra = ""): string {
  return cn(
    "w-full rounded-lg border bg-surface-raised px-3 text-base leading-[1.6] text-ink",
    "h-10 transition-colors duration-instant ease-out placeholder:text-ink-faint",
    "focus:outline-none focus:ring-2 disabled:opacity-50",
    error
      ? "border-danger focus:border-danger focus:ring-danger/20"
      : "border-border focus:border-primary focus:ring-primary/20",
    extra,
  );
}
