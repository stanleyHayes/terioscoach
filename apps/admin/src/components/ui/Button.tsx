import { LoaderCircle } from "lucide-react";
import type { ButtonHTMLAttributes, ReactNode } from "react";
import { cn } from "@/lib/cn";

/**
 * Button — design-system §3.1.
 * Variants: primary / secondary / ghost / danger. Sizes: sm 32px, md 40px (default), lg 48px.
 * Loading: label width is locked, a 16px LoaderCircle spins (900ms linear), the
 * original label stays available to screen readers, aria-busy + disabled.
 */

export type ButtonVariant = "primary" | "secondary" | "ghost" | "danger";
export type ButtonSize = "sm" | "md" | "lg";

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  loading?: boolean;
  fullWidth?: boolean;
  children: ReactNode;
}

const base =
  "inline-flex select-none items-center justify-center gap-2 rounded-md font-semibold transition-colors duration-fast ease-out disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50";

const variants: Record<ButtonVariant, string> = {
  primary:
    "bg-primary text-on-primary shadow-xs hover:bg-primary-hover active:bg-primary-active",
  secondary:
    "border border-border-strong bg-transparent text-ink hover:bg-surface-sunken hover:border-ink-faint active:bg-eucalyptus-100",
  ghost: "bg-transparent text-primary hover:bg-eucalyptus-50 active:bg-eucalyptus-100",
  danger:
    "bg-danger text-on-primary hover:bg-danger-hover active:brightness-95 focus-visible:outline-[color-mix(in_srgb,var(--danger)_40%,transparent)]",
};

const sizes: Record<ButtonSize, string> = {
  sm: "h-8 px-4 text-sm",
  md: "h-10 px-4 text-sm",
  lg: "h-12 px-4 text-base",
};

export function Button({
  variant = "primary",
  size = "md",
  loading = false,
  fullWidth = false,
  disabled,
  className,
  children,
  type = "button",
  ...rest
}: ButtonProps) {
  return (
    <button
      type={type}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      className={cn(
        base,
        variants[variant],
        sizes[size],
        fullWidth && "w-full",
        className,
      )}
      {...rest}
    >
      {loading ? (
        <>
          {/* spinner overlays the invisible label, locking the button width */}
          <span className="relative inline-flex items-center justify-center">
            <span aria-hidden="true" className="invisible">
              {children}
            </span>
            <LoaderCircle
              size={16}
              aria-hidden="true"
              className="animate-loading absolute"
            />
          </span>
          {/* original label kept for screen readers while the spinner shows */}
          <span className="sr-only">{children}</span>
        </>
      ) : (
        children
      )}
    </button>
  );
}
