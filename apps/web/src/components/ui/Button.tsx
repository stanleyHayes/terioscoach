import { forwardRef } from "react";
import type { ButtonHTMLAttributes, ReactNode } from "react";
import { LoaderCircle } from "lucide-react";
import { cn } from "@/lib/cn";

export type ButtonVariant = "primary" | "secondary" | "ghost" | "danger";
export type ButtonSize = "sm" | "md" | "lg";

/* Design-system §3.1. All state changes animate duration-fast ease-out;
 * the focus ring comes from the global :focus-visible rule (brand §6). */
const baseClasses = cn(
  "relative inline-flex items-center justify-center gap-2 whitespace-nowrap",
  "rounded-md font-sans font-semibold tracking-[0.005em]",
  "transition-colors duration-fast ease-out",
  "disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50",
);

const variantClasses: Record<ButtonVariant, string> = {
  primary:
    "bg-primary text-on-primary shadow-xs hover:bg-primary-hover active:bg-primary-active",
  secondary:
    "border border-border-strong bg-transparent text-ink hover:bg-surface-sunken hover:border-ink-faint active:bg-eucalyptus-100",
  ghost: "bg-transparent text-primary hover:bg-eucalyptus-50 active:bg-eucalyptus-100",
  danger: cn(
    "bg-danger text-on-primary hover:bg-danger-hover",
    "active:bg-[color-mix(in_srgb,var(--danger),black_5%)]",
    "focus-visible:[outline-color:color-mix(in_srgb,var(--danger)_40%,transparent)]",
  ),
};

const sizeClasses: Record<ButtonSize, string> = {
  sm: "h-8 px-4 text-sm",
  md: "h-10 px-4 text-sm",
  lg: "h-12 px-4 text-base",
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
