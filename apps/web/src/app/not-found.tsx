import { ArrowLeft, CalendarCheck, Compass } from "lucide-react";
import Link from "next/link";
import { buttonClasses } from "@/components/ui/Button";

export default function NotFound() {
  return (
    <main
      id="main-content"
      className="terios-404 terios-grain min-h-[100dvh] overflow-hidden px-6 py-12 sm:px-10"
    >
      <div className="terios-404-number" aria-hidden="true">
        404
      </div>
      <div className="relative mx-auto grid min-h-[calc(100dvh-6rem)] max-w-6xl items-center gap-12 lg:grid-cols-[1.05fr_.95fr]">
        <div className="max-w-2xl">
          <div className="flex items-center gap-3 text-primary">
            <span className="flex size-11 items-center justify-center rounded-full border border-primary/20 bg-eucalyptus-50">
              <Compass size={20} aria-hidden="true" />
            </span>
            <p className="text-xs font-semibold tracking-[0.12em] uppercase">
              Page not found
            </p>
          </div>
          <h1 className="mt-8 font-display text-[clamp(3.5rem,9vw,7.5rem)] leading-[.88] font-semibold tracking-[-0.065em] text-ink">
            This path has
            <br />
            <em className="font-medium text-primary">gone quiet.</em>
          </h1>
          <p className="mt-7 max-w-[48ch] text-base leading-[1.7] text-ink-muted">
            The address may have changed, but your way back to care is close.
            Return home or go straight to booking.
          </p>
          <div className="mt-9 flex flex-wrap gap-3">
            <Link href="/" className={buttonClasses({ size: "lg" })}>
              <ArrowLeft size={17} aria-hidden="true" />
              Return home
            </Link>
            <Link
              href="/work-with-me"
              className={buttonClasses({ variant: "secondary", size: "lg" })}
            >
              <CalendarCheck size={17} aria-hidden="true" />
              Find a session
            </Link>
          </div>
        </div>
        <Garden />
      </div>
    </main>
  );
}

function Garden() {
  return (
    <div className="terios-404-garden" aria-hidden="true">
      <span className="terios-404-path" />
      <span className="terios-404-stone terios-404-stone-one" />
      <span className="terios-404-stone terios-404-stone-two" />
      <span className="terios-404-stone terios-404-stone-three" />
      <span className="terios-404-leaf" />
    </div>
  );
}
