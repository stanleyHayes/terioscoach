import { ArrowLeft, CalendarDays, SearchX } from "lucide-react";
import Link from "next/link";
import { buttonClasses } from "@/components/ui/Button";

export default function NotFound() {
  return (
    <main className="terios-404 min-h-[100dvh] overflow-hidden px-6 py-12 sm:px-10">
      <div className="terios-404-number" aria-hidden="true">
        404
      </div>
      <div className="relative mx-auto grid min-h-[calc(100dvh-6rem)] max-w-6xl items-center gap-12 lg:grid-cols-[1.05fr_.95fr]">
        <div className="max-w-2xl">
          <div className="flex items-center gap-3 text-primary">
            <span className="flex size-11 items-center justify-center rounded-xl border border-primary/20 bg-eucalyptus-50">
              <SearchX size={20} aria-hidden="true" />
            </span>
            <p className="text-xs font-semibold tracking-[0.12em] uppercase">
              Workspace not found
            </p>
          </div>
          <h1 className="mt-8 font-display text-[clamp(3.25rem,8vw,6.75rem)] leading-[.9] font-semibold tracking-[-0.06em] text-ink">
            That workspace
            <br />
            <em className="font-medium text-primary">is off the map.</em>
          </h1>
          <p className="mt-7 max-w-[48ch] text-base leading-[1.7] text-ink-muted">
            The link may be out of date or your role may not include this area.
            Return to the overview to continue with the tools available to you.
          </p>
          <div className="mt-9 flex flex-wrap gap-3">
            <Link href="/" className={buttonClasses({ size: "lg" })}>
              <ArrowLeft size={17} aria-hidden="true" />
              Back to overview
            </Link>
            <Link
              href="/calendar"
              className={buttonClasses({ variant: "secondary", size: "lg" })}
            >
              <CalendarDays size={17} aria-hidden="true" />
              Open schedule
            </Link>
          </div>
        </div>
        <div className="terios-404-garden" aria-hidden="true">
          <span className="terios-404-path" />
          <span className="terios-404-stone terios-404-stone-one" />
          <span className="terios-404-stone terios-404-stone-two" />
          <span className="terios-404-stone terios-404-stone-three" />
          <span className="terios-404-leaf" />
        </div>
      </div>
    </main>
  );
}
