import type { ReactNode } from "react";

export interface KpiItem {
  label: string;
  value: string;
  detail?: string;
  icon?: ReactNode;
}

export function KpiStrip({
  items,
  label = "Quick summary",
}: {
  items: KpiItem[];
  label?: string;
}) {
  return (
    <dl
      aria-label={label}
      className="grid gap-px overflow-hidden rounded-2xl border border-border bg-border sm:grid-cols-2 xl:grid-cols-4"
    >
      {items.map((item) => (
        <div
          key={item.label}
          className="relative min-w-0 bg-surface-raised px-5 py-4"
        >
          <div className="flex items-center justify-between gap-3">
            <dt className="text-[10px] font-semibold tracking-[.1em] text-ink-faint uppercase">
              {item.label}
            </dt>
            {item.icon ? (
              <span aria-hidden="true" className="text-primary">
                {item.icon}
              </span>
            ) : null}
          </div>
          <dd className="mt-2 font-display text-2xl font-semibold tracking-[-.035em] tabular-nums text-ink">
            {item.value}
          </dd>
          {item.detail ? (
            <p className="mt-1 truncate text-xs text-ink-muted">
              {item.detail}
            </p>
          ) : null}
        </div>
      ))}
    </dl>
  );
}
