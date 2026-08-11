"use client";

import { CircleAlert } from "lucide-react";
import { useId, type TextareaHTMLAttributes } from "react";
import { cn } from "@/lib/cn";

/**
 * TextArea — design-system §3.4 (+ field wrapper §3.29).
 * TextInput rules; min-height 96px, padding space-3, vertical resize only.
 * Anatomy: label → 6px → control → 6px → hint, or error (CircleAlert + danger-ink).
 */

export interface TextAreaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  label: string;
  hint?: string;
  error?: string;
}

export function TextArea({
  label,
  hint,
  error,
  id,
  required,
  disabled,
  readOnly,
  className,
  ...rest
}: TextAreaProps) {
  const autoId = useId();
  const inputId = id ?? autoId;
  const hintId = `${inputId}-hint`;
  const errorId = `${inputId}-error`;

  return (
    <div className="flex flex-col gap-1.5">
      <label
        htmlFor={inputId}
        className="text-sm font-medium tracking-[0.005em] text-ink"
      >
        {label}
        {required ? (
          <>
            <span aria-hidden="true" className="text-accent">
              {" "}
              *
            </span>
            <span className="sr-only"> (required)</span>
          </>
        ) : null}
      </label>

      <textarea
        id={inputId}
        required={required}
        aria-required={required || undefined}
        disabled={disabled}
        readOnly={readOnly}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? errorId : hint ? hintId : undefined}
        className={cn(
          "min-h-24 w-full resize-y rounded-md border bg-surface-raised px-3 py-3 text-base text-ink caret-primary transition-colors duration-fast ease-out placeholder:text-ink-faint",
          error
            ? "border-danger focus-visible:outline-[color-mix(in_srgb,var(--danger)_40%,transparent)]"
            : "border-border-strong hover:border-ink-faint focus:border-primary",
          disabled && "cursor-not-allowed border-border bg-surface-sunken text-ink-faint",
          readOnly && !disabled && "border-border bg-transparent",
          className,
        )}
        {...rest}
      />

      {error ? (
        <p
          id={errorId}
          role="alert"
          className="flex items-center gap-1.5 text-[13px] leading-[1.45] font-medium tracking-[0.01em] text-danger-ink"
        >
          <CircleAlert size={14} aria-hidden="true" className="shrink-0" />
          {error}
        </p>
      ) : hint ? (
        <p
          id={hintId}
          className="text-[13px] leading-[1.45] font-medium tracking-[0.01em] text-ink-faint"
        >
          {hint}
        </p>
      ) : null}
    </div>
  );
}
