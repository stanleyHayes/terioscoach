import { ArrowLeft, SearchX } from "lucide-react";
import Link from "next/link";

export default function NotFound() {
  return (
    <main className="flex min-h-[100dvh] items-center justify-center px-6 py-16">
      <div className="max-w-md text-center">
        <span className="mx-auto flex size-14 items-center justify-center rounded-2xl bg-primary/10 text-primary"><SearchX size={24} aria-hidden="true" /></span>
        <p className="mt-7 text-xs font-semibold tracking-[0.08em] text-primary uppercase">Not found</p>
        <h1 className="mt-2 font-display text-4xl font-semibold tracking-[-0.03em] text-ink">That workspace does not exist.</h1>
        <p className="mt-4 text-sm leading-relaxed text-ink-muted">Return to the practice overview and continue from there.</p>
        <Link href="/" className="mt-7 inline-flex h-10 items-center gap-2 rounded-full bg-primary px-4 text-sm font-semibold text-on-primary"><ArrowLeft size={16} aria-hidden="true" />Back to overview</Link>
      </div>
    </main>
  );
}
