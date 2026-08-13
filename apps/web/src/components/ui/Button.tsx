import { forwardRef } from "react";
import type { ButtonHTMLAttributes, ReactNode } from "react";
import { LoaderCircle } from "lucide-react";
import { cn } from "@/lib/cn";

export type ButtonVariant = "primary" | "secondary" | "ghost" | "danger";
export type ButtonSize = "sm" | "md" | "lg";

/* Design-system §3.1. All state changes animate duration-fast ease-out;
 * the focus ring comes from the global :focus-visible rule (brand §6). */
const baseClasses = cn(
  "terios-button relative isolate inline-flex items-center justify-center gap-2 overflow-hidden whitespace-nowrap",
  "rounded-full font-sans font-semibold tracking-[0.005em]",
  "transition-[color,background-color,border-color,box-shadow,transform] duration-fast ease-out hover:-translate-y-0.5 active:scale-[.975] active:translate-y-0",
  "disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50",
);

const variantClasses: Record<ButtonVariant, string> = {
  primary:
    "terios-button-primary bg-primary text-on-primary shadow-[0_8px_24px_color-mix(in_srgb,var(--primary)_22%,transparent)] hover:bg-primary-hover hover:shadow-[0_12px_30px_color-mix(in_srgb,var(--primary)_26%,transparent)] active:bg-primary-active",
  secondary:
    "terios-button-secondary border border-border-strong bg-transparent text-ink hover:bg-surface-sunken hover:border-primary active:bg-eucalyptus-100",
  ghost: "terios-button-ghost bg-transparent text-primary hover:bg-eucalyptus-50 active:bg-eucalyptus-100",
  danger: cn(
    "bg-danger text-on-primary hover:bg-danger-hover",
    "active:bg-[color-mix(in_srgb,var(--danger),black_5%)]",
    "focus-visible:[outline-color:color-mix(in_srgb,var(--danger)_40%,transparent)]",
  ),
};

const sizeClasses: Record<ButtonSize, string> = {
  sm: "h-9 px-4 text-sm",
  md: "h-11 px-5 text-sm",
  lg: "h-[3.25rem] px-6 text-base",
};

export interface ButtonClassOptions {
  variant?: ButtonVariant;
  size?: ButtonSize;
  fullWidth?: boolean;
  className?: string;
}

/** Class composition for anything that must look like a Button
 * (e.g. a Next <Link> used as a CTA). Keeps one source of truth. */
export function buttonClasses({
  variant = "primary",
  size = "md",
  fullWidth = false,
  className,
}: ButtonClassOptions = {}): string {
  return cn(
    baseClasses,
    variantClasses[variant],
    sizeClasses[size],
    fullWidth && "w-full",
    className,
  );
}

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  /** Replaces the visible label with a 16px spinner; width is locked and the
   * original label stays available to screen readers (`aria-busy`). */
  loading?: boolean;
  fullWidth?: boolean;
  children: ReactNode;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  {
    variant = "primary",
    size = "md",
    loading = false,
    fullWidth = false,
    disabled,
    className,
    type = "button",
    children,
    ...props
  },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      className={buttonClasses({ variant, size, fullWidth, className })}
      {...props}
    >
      <span
        aria-hidden={loading || undefined}
        className={cn(
          "inline-flex items-center justify-center gap-2",
          loading && "invisible",
        )}
      >
        {children}
      </span>
      {loading && (
        <>
          <span className="sr-only">{children}</span>
          <LoaderCircle
            aria-hidden="true"
            className="absolute inset-0 m-auto size-4 animate-spin"
          />
        </>
      )}
    </button>
  );
});
