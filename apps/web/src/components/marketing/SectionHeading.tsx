import { cn } from "@/lib/cn";

export interface SectionHeadingProps {
  /** Micro-label overline (type scale `micro`): 11px, uppercase, +0.08em. */
  eyebrow?: string;
  title: string;
  /** Optional lead paragraph (type scale `body-lg`). */
  description?: string;
  align?: "left" | "center";
  /** Set when a parent Section needs `aria-labelledby`. */
  id?: string;
  className?: string;
}

/** Marketing section header (brand.md §4): micro eyebrow, display-lg Fraunces
 * title, body-lg muted description. Measure-capped, text-wrap pretty. */
export function SectionHeading({
  eyebrow,
  title,
  description,
  align = "left",
  id,
  className,
}: SectionHeadingProps) {
  return (
    <div
      className={cn(
        "max-w-[68ch]",
        align === "center" && "mx-auto text-center",
        className,
      )}
    >
      {eyebrow && (
        <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-muted">
          {eyebrow}
        </p>
      )}
      <h2
        id={id}
        className={cn(
          "mt-3 font-display text-[2.5rem] leading-[1.1] font-medium tracking-[-0.015em] text-ink",
          "[text-wrap:pretty]",
          !eyebrow && "mt-0",
        )}
      >
        {title}
      </h2>
      {description && (
        <p
          className={cn(
            "mt-5 text-lg leading-[1.6] text-ink-muted [text-wrap:pretty]",
            align === "center" && "mx-auto",
          )}
        >
          {description}
        </p>
      )}
    </div>
  );
}
