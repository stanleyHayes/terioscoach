"use client";

import { useRef, type ClipboardEvent, type KeyboardEvent } from "react";

const LENGTH = 6;

export function OtpInput({ value, onChange, disabled = false, label = "Authenticator code" }: { value: string; onChange: (value: string) => void; disabled?: boolean; label?: string }) {
  const refs = useRef<Array<HTMLInputElement | null>>([]);
  const digits = Array.from({ length: LENGTH }, (_, index) => value[index] ?? "");
  function setDigit(index: number, raw: string) {
    const digit = raw.replace(/\D/g, "").slice(-1);
    const next = [...digits]; next[index] = digit; onChange(next.join("").slice(0, LENGTH));
    if (digit && index < LENGTH - 1) refs.current[index + 1]?.focus();
  }
  function onKeyDown(index: number, event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === "Backspace" && !digits[index] && index > 0) refs.current[index - 1]?.focus();
    if (event.key === "ArrowLeft" && index > 0) refs.current[index - 1]?.focus();
    if (event.key === "ArrowRight" && index < LENGTH - 1) refs.current[index + 1]?.focus();
  }
  function onPaste(event: ClipboardEvent<HTMLInputElement>) {
    const pasted = event.clipboardData.getData("text").replace(/\D/g, "").slice(0, LENGTH);
    if (!pasted) return; event.preventDefault(); onChange(pasted); refs.current[Math.min(pasted.length, LENGTH) - 1]?.focus();
  }
  return <fieldset disabled={disabled} className="min-w-0"><legend className="mb-2 text-sm font-medium text-ink">{label}</legend><div className="grid grid-cols-6 gap-2" onPaste={onPaste}>{digits.map((digit, index) => <input key={index} ref={(node) => { refs.current[index] = node; }} value={digit} onChange={(event) => setDigit(index, event.target.value)} onKeyDown={(event) => onKeyDown(index, event)} onFocus={(event) => event.currentTarget.select()} inputMode="numeric" pattern="[0-9]*" autoComplete={index === 0 ? "one-time-code" : "off"} aria-label={`${label} digit ${index + 1}`} maxLength={1} className="aspect-square min-w-0 rounded-xl border border-border bg-surface-raised text-center font-display text-xl font-semibold text-ink outline-none transition-[border-color,box-shadow] focus:border-primary focus:ring-2 focus:ring-primary/20 disabled:opacity-50" />)}</div></fieldset>;
}
