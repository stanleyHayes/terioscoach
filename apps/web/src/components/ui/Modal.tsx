"use client";

import { X } from "lucide-react";
import { useEffect, useId, useRef, type ReactNode } from "react";
import { cn } from "@/lib/cn";

/**
 * Modal — design-system §3.14.
 * Built on the native <dialog> element only as an a11y primitive (focus trap,
 * Esc, ::backdrop); all chrome is custom. Scrim `overlay`; panel
 * `surface-raised radius-xl shadow-lg`, width 480px, max 85vh, padding
 * space-6; entrance fade + scale .96→1 + translateY 8px→0, duration-slow
 * ease-out. Mobile (<640px) becomes a bottom sheet with a decorative drag
 * handle pill. Esc and scrim clicks close.
 */

/* Panel entrance + exit live here (not globals.css — that file is the token
 * arbiter shared verbatim with apps/admin and must not drift). */
const modalKeyframes = `
@keyframes terios-modal-in {
  from { opacity: 0; transform: scale(0.96) translateY(8px); }
  to { opacity: 1; transform: scale(1) translateY(0); }
}
@media (prefers-reduced-motion: reduce) {
  .terios-modal-panel { animation-duration: 150ms !important; }
}
`;

export interface ModalProps {
  open: boolean;
  onClose: () => void;
  /** Rendered as display-sm; wired via aria-labelledby. */
  title: string;
  description?: string;
  /** Body content (body-md). */
  children: ReactNode;
  /** Right-aligned action row, gap space-3, primary last. */
  footer?: ReactNode;
  /** Panel width preset; default 480px. */
  size?: "sm" | "md" | "lg";
  labelId?: string;
}

const widthClasses: Record<NonNullable<ModalProps["size"]>, string> = {
  sm: "sm:w-[400px]",
  md: "sm:w-[480px]",
  lg: "sm:w-[640px]",
};

export function Modal({
  open,
  onClose,
  title,
  description,
  children,
  footer,
  size = "md",
}: ModalProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const titleId = useId();

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    if (open && !dialog.open) {
      // jsdom (tests) predates full dialog support — fall back to the open
      // attribute so the panel is simply visible there.
      if (typeof dialog.showModal === "function") {
        dialog.showModal();
      } else {
        dialog.setAttribute("open", "");
      }
    } else if (!open && dialog.open) {
      // Guarded to match the branch above. An environment without
      // showModal has no close either, and an unguarded call there throws
      // on the way out of a modal that opened perfectly well.
      if (typeof dialog.close === "function") {
        dialog.close();
      } else {
        dialog.removeAttribute("open");
      }
    }
  }, [open]);

  return (
    <dialog
      ref={dialogRef}
      aria-modal="true"
      aria-labelledby={titleId}
      onCancel={(event) => {
        event.preventDefault();
        onClose();
      }}
      onClick={(event) => {
        // Clicks on the dialog element itself land on the scrim (the panel
        // fills the rest) — a scrim click closes.
        if (event.target === dialogRef.current) onClose();
      }}
      className={cn(
        "fixed inset-0 m-auto h-fit w-full max-w-[calc(100vw-32px)] border-none bg-transparent p-0",
        "backdrop:bg-overlay",
        // Bottom sheet on mobile: pin to the bottom edge, square off the
        // bottom corners.
        "max-sm:mt-auto max-sm:mb-0",
        widthClasses[size],
      )}
    >
      <style>{modalKeyframes}</style>
      <div
        className={cn(
          "terios-modal-panel max-h-[88vh] overflow-y-auto rounded-[2rem] border border-border bg-surface-raised p-6 shadow-[0_35px_100px_rgba(28,51,40,.22)] sm:p-8",
          "max-sm:rounded-b-none",
          open && "[animation:terios-modal-in_var(--duration-slow)_var(--ease-out)_both]",
        )}
      >
        {/* Decorative drag handle, bottom-sheet mode only. */}
        <div
          aria-hidden="true"
          className="mx-auto mb-4 hidden h-1 w-8 rounded-full bg-border-strong max-sm:block"
        />
        <div className="flex items-start justify-between gap-4">
          <div>
            <h2
              id={titleId}
              className="font-display text-[1.75rem] leading-[1.08] font-semibold tracking-[-0.03em] text-ink"
            >
              {title}
            </h2>
            {description ? (
              <p className="mt-1 text-sm leading-[1.55] text-ink-muted">{description}</p>
            ) : null}
          </div>
          <button
            type="button"
            aria-label="Close"
            onClick={onClose}
            className={cn(
              "-mt-1 -mr-1 flex size-8 shrink-0 items-center justify-center rounded-sm text-ink-faint",
              "transition-colors duration-instant ease-out hover:bg-surface-sunken hover:text-ink-muted",
            )}
          >
            <X size={20} aria-hidden="true" />
          </button>
        </div>

        <div className="mt-4 text-base leading-[1.6] text-ink">{children}</div>

        {footer ? <div className="mt-6 flex justify-end gap-3">{footer}</div> : null}
      </div>
    </dialog>
  );
}
