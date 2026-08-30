"use client";

import {
  Bold,
  Code2,
  Eye,
  Heading2,
  Italic,
  Link2,
  List,
  ListOrdered,
  Quote,
  Redo2,
  Undo2,
  WrapText,
} from "lucide-react";
import { useId, useRef, useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { cn } from "@/lib/cn";

type Mode = "write" | "preview";

const TOOLS = [
  { label: "Heading", icon: Heading2, before: "## ", after: "", fallback: "Section heading" },
  { label: "Bold", icon: Bold, before: "**", after: "**", fallback: "bold text" },
  { label: "Italic", icon: Italic, before: "_", after: "_", fallback: "emphasized text" },
  { label: "Link", icon: Link2, before: "[", after: "](https://)", fallback: "link text" },
  { label: "Bulleted list", icon: List, before: "- ", after: "", fallback: "List item" },
  { label: "Numbered list", icon: ListOrdered, before: "1. ", after: "", fallback: "List item" },
  { label: "Quote", icon: Quote, before: "> ", after: "", fallback: "A useful thought" },
  { label: "Inline code", icon: Code2, before: "`", after: "`", fallback: "code" },
] as const;

export function MarkdownEditor({
  value,
  onChange,
  error,
}: {
  value: string;
  onChange: (value: string) => void;
  error?: string;
}) {
  const id = useId();
  const textarea = useRef<HTMLTextAreaElement>(null);
  const [mode, setMode] = useState<Mode>("write");

  function insert(before: string, after = "", fallback = "text") {
    const field = textarea.current;
    if (!field) return;
    const start = field.selectionStart;
    const end = field.selectionEnd;
    const selected = value.slice(start, end) || fallback;
    const next = `${value.slice(0, start)}${before}${selected}${after}${value.slice(end)}`;
    onChange(next);
    requestAnimationFrame(() => {
      field.focus();
      field.setSelectionRange(start + before.length, start + before.length + selected.length);
    });
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <label htmlFor={id} className="text-sm font-medium text-ink">
            Body <span className="text-accent">*</span>
          </label>
          <p className="mt-1 text-xs text-ink-faint">Markdown supported · preview matches the public article.</p>
        </div>
        <div className="flex rounded-xl border border-border bg-surface-sunken p-1" role="tablist" aria-label="Body editor mode">
          {(["write", "preview"] as const).map((tab) => (
            <button
              key={tab}
              type="button"
              role="tab"
              aria-selected={mode === tab}
              onClick={() => setMode(tab)}
              className={cn(
                "inline-flex h-8 items-center gap-2 rounded-lg px-3 text-xs font-semibold capitalize",
                mode === tab ? "bg-surface-raised text-ink shadow-xs" : "text-ink-muted hover:text-ink",
              )}
            >
              {tab === "write" ? <WrapText size={14} /> : <Eye size={14} />}
              {tab}
            </button>
          ))}
        </div>
      </div>

      <div className={cn("overflow-hidden rounded-2xl border bg-surface-raised", error ? "border-danger" : "border-border-strong focus-within:border-primary")}>
        <div className="flex min-h-11 flex-wrap items-center gap-0.5 border-b border-border bg-surface-sunken/70 px-2 py-1.5">
          {mode === "write" ? TOOLS.map(({ label, icon: Icon, before, after, fallback }) => (
            <button key={label} type="button" aria-label={label} title={label} onClick={() => insert(before, after, fallback)} className="flex size-8 items-center justify-center rounded-lg text-ink-muted hover:bg-surface-raised hover:text-ink">
              <Icon size={15} />
            </button>
          )) : <p className="px-2 text-xs font-medium text-ink-faint">Rendered preview</p>}
          {mode === "write" ? <span className="ml-auto flex items-center gap-0.5 border-l border-border pl-2">
            <button type="button" aria-label="Undo" onClick={() => document.execCommand("undo")} className="flex size-8 items-center justify-center rounded-lg text-ink-muted hover:bg-surface-raised"><Undo2 size={15} /></button>
            <button type="button" aria-label="Redo" onClick={() => document.execCommand("redo")} className="flex size-8 items-center justify-center rounded-lg text-ink-muted hover:bg-surface-raised"><Redo2 size={15} /></button>
          </span> : null}
        </div>
        {mode === "write" ? (
          <textarea
            ref={textarea}
            id={id}
            value={value}
            aria-invalid={Boolean(error)}
            aria-describedby={error ? `${id}-error` : undefined}
            onChange={(event) => onChange(event.target.value)}
            placeholder="Begin with the idea you want your reader to carry away…"
            className="min-h-[26rem] w-full resize-y bg-transparent px-5 py-4 font-mono text-sm leading-7 text-ink outline-none placeholder:text-ink-faint"
          />
        ) : (
          <article className="terios-markdown min-h-[26rem] px-6 py-5">
            {value.trim() ? <ReactMarkdown remarkPlugins={[remarkGfm]}>{value}</ReactMarkdown> : <p className="text-sm text-ink-faint">Nothing to preview yet.</p>}
          </article>
        )}
      </div>
      {error ? <p id={`${id}-error`} role="alert" className="text-[13px] font-medium text-danger-ink">{error}</p> : null}
    </div>
  );
}
