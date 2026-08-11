import type { ButtonHTMLAttributes, ReactNode } from "react";
import { cn } from "@/lib/cn";

/**
 * IconButton — design-system §3.2.
 * Square button, icon centered. Sizes sm 32 / md 40 / lg 48; variants ghost
 * (default), outline, filled. `aria-label` is mandatory (enforced by the type).
 */

export type IconButtonVariant = "ghost" | "outline" | "filled";
export type IconButtonSize = "sm" | "md" | "lg";

export interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: IconButtonVariant;
  size?: IconButtonSize;
  "aria-label": string;
  children: ReactNode;
}

const base =
  "inline-flex shrink-0 select-none items-center justify-center rounded-md transition-colors duration-fast ease-out disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50";

const variants: Record<IconButtonVariant, string> = {
  ghost: "bg-transparent text-ink-muted hover:bg-surface-sunken hover:text-ink",
  outline:
    "border border-border-strong bg-transparent text-ink-muted hover:bg-surface-sunken hover:text-ink",
  filled: "bg-primary text-on-primary hover:bg-primary-hover active:bg-primary-active",
};

const sizes: Record<IconButtonSize, string> = {
  sm: "size-8 [&_svg]:size-4",
  md: "size-10 [&_svg]:size-5",
  lg: "size-12 [&_svg]:size-5",
};

export function IconButton({
  variant = "ghost",
  size = "md",
  className,
  children,
  type = "button",
  ...rest
}: IconButtonProps) {
  return (
    <button
      type={type}
      className={cn(base, variants[variant], sizes[size], className)}
      {...rest}
    >
      {children}
    </button>
  );
}
