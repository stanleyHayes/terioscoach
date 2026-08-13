"use client";

import { Undo2 } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { cn } from "@/lib/cn";

/**
 * Signature pad (design-system §3, CX-07).
 *
 * Built on a canvas with pointer events, which covers mouse, touch and
 * stylus with one code path — no native control does this, and the design
 * system forbids borrowing one anyway.
 *
 * The canvas is sized to its own displayed box multiplied by the device
 * pixel ratio, so a signature drawn on a phone is not a blurry
 * approximation of itself when the practitioner opens the record later.
 *
 * The exported value is a PNG data URL, which is exactly what the API
 * accepts — it refuses remote URLs and every other scheme, because a
 * consent record whose mark lives on someone else's server is not a record.
 */
export interface SignaturePadProps {
  /** Called with a PNG data URL, or null when the pad is cleared. */
  onChange: (dataUrl: string | null) => void;
  label: string;
  describedBy?: string;
  disabled?: boolean;
}

export function SignaturePad({ onChange, label, describedBy, disabled }: SignaturePadProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const drawing = useRef(false);
  const hasInk = useRef(false);
  const [signed, setSigned] = useState(false);

  /** Matches the backing store to the displayed size and the pixel ratio.
   * Resizing clears the canvas, so a signature is not silently distorted by
   * a rotated phone — the client is told to sign again instead. */
  const resize = useCallback(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const ratio = window.devicePixelRatio || 1;
    const { width, height } = canvas.getBoundingClientRect();
    canvas.width = Math.max(1, Math.floor(width * ratio));
    canvas.height = Math.max(1, Math.floor(height * ratio));

    const context = canvas.getContext("2d");
    if (!context) return;
    context.scale(ratio, ratio);
    context.lineWidth = 2;
    context.lineCap = "round";
    context.lineJoin = "round";
    // Ink colour is read from the rendered element so the pad follows the
    // brand tokens rather than hard-coding one of them.
    context.strokeStyle = getComputedStyle(canvas).color || "#1F2922";

    if (hasInk.current) {
      hasInk.current = false;
      setSigned(false);
      onChange(null);
    }
  }, [onChange]);

  useEffect(() => {
    resize();
    window.addEventListener("resize", resize);
    return () => window.removeEventListener("resize", resize);
  }, [resize]);

  function pointAt(event: React.PointerEvent<HTMLCanvasElement>) {
    const canvas = canvasRef.current!;
    const rect = canvas.getBoundingClientRect();
    return { x: event.clientX - rect.left, y: event.clientY - rect.top };
  }

  function start(event: React.PointerEvent<HTMLCanvasElement>) {
    if (disabled) return;
    const canvas = canvasRef.current;
    const context = canvas?.getContext("2d");
    if (!canvas || !context) return;

    // Capturing the pointer keeps the stroke going if a finger strays
    // outside the box mid-signature.
    canvas.setPointerCapture(event.pointerId);
    drawing.current = true;

    const { x, y } = pointAt(event);
    context.beginPath();
    context.moveTo(x, y);
  }

  function move(event: React.PointerEvent<HTMLCanvasElement>) {
    if (!drawing.current || disabled) return;
    const context = canvasRef.current?.getContext("2d");
    if (!context) return;

    const { x, y } = pointAt(event);
    context.lineTo(x, y);
    context.stroke();
    hasInk.current = true;
  }

  function end(event: React.PointerEvent<HTMLCanvasElement>) {
    if (!drawing.current) return;
    drawing.current = false;
    canvasRef.current?.releasePointerCapture(event.pointerId);

    if (hasInk.current) {
      setSigned(true);
      onChange(canvasRef.current?.toDataURL("image/png") ?? null);
    }
  }

  function clear() {
    const canvas = canvasRef.current;
    const context = canvas?.getContext("2d");
    if (!canvas || !context) return;

    context.clearRect(0, 0, canvas.width, canvas.height);
    hasInk.current = false;
    setSigned(false);
    onChange(null);
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center justify-between gap-3">
        <span id="signature-pad-label" className="text-sm font-medium text-ink">
          {label}
        </span>
        <button
          type="button"
          onClick={clear}
          disabled={disabled || !signed}
          className="inline-flex items-center gap-1.5 text-[13px] font-medium text-ink-muted transition-colors duration-instant ease-out hover:text-ink disabled:opacity-40"
        >
          <Undo2 size={14} aria-hidden="true" />
          Clear
        </button>
      </div>

      <canvas
        ref={canvasRef}
        role="img"
        aria-label={signed ? `${label} — signed` : `${label} — draw your signature here`}
        aria-labelledby="signature-pad-label"
        aria-describedby={describedBy}
        onPointerDown={start}
        onPointerMove={move}
        onPointerUp={end}
        onPointerCancel={end}
        className={cn(
          "h-40 w-full rounded-lg border bg-surface-raised text-ink",
          // touch-none stops the browser scrolling the page while a finger
          // is drawing — without it a signature on a phone is impossible.
          "touch-none",
          signed ? "border-primary" : "border-border",
          disabled && "opacity-50",
        )}
      />

      <p aria-live="polite" className="min-h-[1.25rem] text-[13px] text-ink-faint">
        {signed ? "Signed. Clear it if you would like to sign again." : ""}
      </p>
    </div>
  );
}
