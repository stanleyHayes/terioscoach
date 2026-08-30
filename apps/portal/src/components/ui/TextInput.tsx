"use client";

import { CircleAlert, Eye, EyeOff } from "lucide-react";
import { useId, useState, type InputHTMLAttributes, type ReactNode } from "react";
import { cn } from "@/lib/cn";

/**
 * TextInput + form field wrapper — design-system §3.3 + §3.29.
 * Fully restyled (native validation bubbles are forbidden — pair with
 * `noValidate` on the form). Anatomy: label → 6px → control → 6px → hint,
 * or error (CircleAlert 14px + danger-ink, announced via role="alert").
 * Password fields get an Eye/EyeOff visibility toggle in the trailing slot.
 */

export type TextInputSize = "sm" | "md" | "lg";

export interface TextInputProps
  extends Omit<InputHTMLAttributes<HTMLInputElement>, "size"> {
  label: string;
  hint?: string;
  error?: string;
  size?: TextInputSize;
  /** 16px icon rendered inside the control, left side. */
  leadingIcon?: ReactNode;
}

const controlSizes: Record<TextInputSize, string> = {
  sm: "h-8 text-sm",
  md: "h-10 text-base",
  lg: "h-12 text-base",
};

export function TextInput({
  label,
  hint,
  error,
  size = "md",
  leadingIcon,
  type = "text",
  id,
  required,
  disabled,
  readOnly,
  className,
  ...rest
}: TextInputProps) {
  const autoId = useId();
  const inputId = id ?? autoId;
  const hintId = `${inputId}-hint`;
  const errorId = `${inputId}-error`;
  const isPassword = type === "password";
  const [passwordVisible, setPasswordVisible] = useState(false);

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

      <div className="relative">
        {leadingIcon ? (
          <span
            aria-hidden="true"
            className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-ink-faint [&_svg]:size-4"
          >
            {leadingIcon}
          </span>
        ) : null}
        <input
          id={inputId}
          type={isPassword && passwordVisible ? "text" : type}
          required={required}
          aria-required={required || undefined}
          disabled={disabled}
          readOnly={readOnly}
          aria-invalid={error ? true : undefined}
          aria-describedby={error ? errorId : hint ? hintId : undefined}
          className={cn(
            "w-full rounded-xl border bg-surface-raised px-3.5 text-ink caret-primary transition-[border-color,background-color,box-shadow] duration-fast ease-out placeholder:text-ink-faint",
            controlSizes[size],
            leadingIcon ? "pl-9" : undefined,
            isPassword && "pr-11",
            error
              ? "border-danger focus-visible:outline-[color-mix(in_srgb,var(--danger)_40%,transparent)]"
              : "border-border-strong hover:border-ink-faint focus:border-primary",
            disabled && "cursor-not-allowed border-border bg-surface-sunken text-ink-faint",
            readOnly && !disabled && "border-border bg-transparent",
            className,
          )}
          {...rest}
        />
        {isPassword ? (
          <button
            type="button"
            tabIndex={-1}
            aria-label={passwordVisible ? "Hide password" : "Show password"}
            onClick={() => setPasswordVisible((visible) => !visible)}
            className="absolute top-1/2 right-1.5 flex size-8 -translate-y-1/2 items-center justify-center rounded-sm text-ink-faint transition-colors duration-fast ease-out hover:bg-surface-sunken hover:text-ink-muted"
          >
            {passwordVisible ? (
              <EyeOff size={16} aria-hidden="true" />
            ) : (
              <Eye size={16} aria-hidden="true" />
            )}
          </button>
        ) : null}
      </div>

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
