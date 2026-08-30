import type { ReactNode } from "react";

export function EmptyState({
  icon,
  title,
  description,
  action,
  compact = false,
  className = "",
}: {
  icon: ReactNode;
  title: string;
  description: string;
  action?: ReactNode;
  compact?: boolean;
  className?: string;
}) {
  return (
    <div
      className={`mx-auto flex max-w-[390px] flex-col items-center px-6 text-center ${compact ? "py-7" : "py-12"} ${className}`}
    >
      <span
        aria-hidden="true"
        className="terios-empty-icon flex size-16 items-center justify-center rounded-full bg-surface-sunken text-ink-faint"
      >
        {icon}
      </span>
      <h3 className="mt-6 font-display text-[1.5rem] leading-[1.2] font-medium tracking-[-0.01em] text-ink">
        {title}
      </h3>
      <p className="mt-2 text-sm leading-[1.55] text-ink-muted">
        {description}
      </p>
      {action ? (
        <div className="mt-6 flex flex-wrap justify-center gap-3">{action}</div>
      ) : null}
    </div>
  );
}
