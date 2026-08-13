import type { ReactNode } from "react";

export function AdminPageHeader({ eyebrow, title, description, actions }: { eyebrow?: string; title: string; description?: string; actions?: ReactNode }) {
  return (
    <header className="relative overflow-hidden rounded-[1.75rem] border border-border/70 bg-surface-raised/75 px-6 py-7 shadow-[0_28px_90px_rgba(0,0,0,.1)] backdrop-blur-xl sm:px-8 sm:py-9">
      <div aria-hidden="true" className="absolute inset-y-0 left-0 w-1 bg-primary" />
      <div aria-hidden="true" className="absolute -right-16 -top-24 size-64 rounded-full bg-primary/10 blur-3xl" />
      <div className="relative flex flex-col justify-between gap-6 md:flex-row md:items-end">
        <div>
          {eyebrow ? <p className="text-[11px] font-semibold tracking-[0.12em] text-primary uppercase">{eyebrow}</p> : null}
          <h1 className="mt-2 font-display text-[2rem] leading-tight font-semibold tracking-[-0.035em] text-ink sm:text-[2.5rem]">{title}</h1>
          {description ? <p className="mt-3 max-w-[58ch] text-sm leading-relaxed text-ink-muted">{description}</p> : null}
        </div>
        {actions ? <div className="flex shrink-0 flex-wrap gap-2">{actions}</div> : null}
      </div>
    </header>
  );
}
