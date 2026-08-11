"use client";

import { LoaderCircle } from "lucide-react";
import { cn } from "@/lib/cn";

/**
 * Switch — design-system §3.9.
 * Track 40×22px radius-full, thumb 18px sand-0 with shadow-xs, 2px inset.
 * Off: track sand-300 (hover ink-faint). On: track primary, thumb slides 18px
 * (duration-base ease-out). Loading: thumb replaced by a 12px spinner.
 * `role="switch"` + `aria-checked`; the visual 22px track sits inside a 40px
 * hit target. Never used for instantaneous destructive actions.
 */

export interface SwitchProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  /** Accessible name — required because there is no visible label here. */
  label: string;
  disabled?: boolean;
  loading?: boolean;
}

export function Switch({ checked, onChange, label, disabled = false, loading = false }: SwitchProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-busy={loading || undefined}
      aria-label={label}
      disabled={disabled || loading}
      onClick={() => onChange(!checked)}
      className={cn(
        "inline-flex size-10 shrink-0 items-center justify-center rounded-full transition-colors duration-fast ease-out disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50",
      )}
    >
      <span
        aria-hidden="true"
        className={cn(
          "relative h-[22px] w-10 rounded-full transition-colors duration-fast ease-out",
          checked ? "bg-primary" : "bg-sand-300 hover:bg-ink-faint",
        )}
      >
        {loading ? (
          <LoaderCircle
            size={12}
            className={cn(
              "animate-loading absolute top-[5px] left-[5px]",
              checked ? "text-on-primary" : "text-ink-muted",
            )}
          />
        ) : (
          <span
            className={cn(
              "absolute top-[2px] left-[2px] size-[18px] rounded-full bg-sand-0 shadow-xs transition-transform duration-base ease-out",
              checked && "translate-x-[18px]",
            )}
          />
        )}
      </span>
    </button>
  );
}
