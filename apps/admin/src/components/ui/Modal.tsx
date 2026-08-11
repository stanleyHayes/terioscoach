"use client";

import { X } from "lucide-react";
import { useEffect, useId, useRef, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { IconButton } from "@/components/ui/IconButton";
import { cn } from "@/lib/cn";

/**
 * Modal — design-system §3.14.
 * Custom chrome throughout (no native <dialog>): overlay scrim (250ms fade),
 * panel surface-raised radius-xl shadow-lg, entrance fade + scale .96→1 +
 * translateY 8px→0 (duration-slow ease-out). Width 480px default / 560px form,
 * max-height 85vh with internal scroll. Below 640px it docks as a bottom sheet
 * with a decorative drag-handle pill.
 *
 * A11y: role="dialog" + aria-modal + aria-labelledby, focus trapped inside the
 * panel, initial focus on the element marked `data-autofocus` (else the first
 * focusable), focus restored to the trigger on close. Esc closes unless the
 * content marks itself dirty (an open form keeps its data).
 */

export interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: string;
  /** 480px default, 560px for forms (per spec). */
  size?: "default" | "form";
  /** When true, Esc will not close the modal (dirty form protection). */
  dirty?: boolean;
  children: ReactNode;
  footer?: ReactNode;
}

const FOCUSABLE =
  'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])';

export function Modal({
  open,
  onClose,
  title,
  description,
  size = "default",
  dirty = false,
  children,
  footer,
}: ModalProps) {
  const titleId = useId();
  const panelRef = useRef<HTMLDivElement>(null);
  const dirtyRef = useRef(dirty);
  const onCloseRef = useRef(onClose);

  // Keep the latest dirty/onClose available to the document-level keydown
  // handler without re-subscribing (and re-focusing) on every render.
  useEffect(() => {
    dirtyRef.current = dirty;
    onCloseRef.current = onClose;
  }, [dirty, onClose]);

  useEffect(() => {
    if (!open) return;
    const panel = panelRef.current;
    if (!panel) return;

    const previouslyFocused = document.activeElement as HTMLElement | null;
    const initial =
      panel.querySelector<HTMLElement>("[data-autofocus]") ??
      panel.querySelector<HTMLElement>(FOCUSABLE) ??
      panel;
    initial.focus();

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        // Dirty forms keep their data — Esc is swallowed instead of confirming.
        if (!dirtyRef.current) onCloseRef.current();
        return;
      }
      if (event.key !== "Tab") return;

      const items = [...panel!.querySelectorAll<HTMLElement>(FOCUSABLE)].filter(
        (el) => el.getClientRects().length > 0,
      );
      if (items.length === 0) {
        event.preventDefault();
        return;
      }
      const first = items[0]!;
      const last = items[items.length - 1]!;
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    }

    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("keydown", handleKeyDown);
      previouslyFocused?.focus?.();
    };
  }, [open]);

  if (!open) return null;

  return createPortal(
    <div className="fixed inset-0 z-modal flex items-end justify-center sm:items-center sm:p-4">
      {/* scrim */}
      <div
        aria-hidden="true"
        className="animate-fade-in absolute inset-0 bg-overlay"
        onClick={() => {
          if (!dirtyRef.current) onCloseRef.current();
        }}
      />
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        className={cn(
          "animate-modal-enter relative flex max-h-[85vh] w-full flex-col overflow-y-auto rounded-t-xl bg-surface-raised p-6 shadow-lg sm:rounded-xl",
          size === "form" ? "sm:max-w-[560px]" : "sm:max-w-[480px]",
        )}
      >
        {/* mobile bottom-sheet drag handle (decorative) */}
        <span
          aria-hidden="true"
          className="mx-auto mb-4 h-1 w-8 shrink-0 rounded-full bg-border-strong sm:hidden"
        />

        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <h2
              id={titleId}
              className="font-display text-2xl leading-[1.2] tracking-[-0.01em] text-ink"
            >
              {title}
            </h2>
            {description ? (
              <p className="mt-1 text-sm leading-[1.55] text-ink-muted">{description}</p>
            ) : null}
          </div>
          <IconButton aria-label="Close" size="sm" onClick={() => onCloseRef.current()}>
            <X aria-hidden="true" />
          </IconButton>
        </div>

        <div className="mt-5">{children}</div>

        {footer ? <div className="mt-6 flex justify-end gap-3">{footer}</div> : null}
      </div>
    </div>,
    document.body,
  );
}
