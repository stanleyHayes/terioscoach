import { ArrowLeft, Compass } from "lucide-react";
import Link from "next/link";
import { buttonClasses } from "@/components/ui/Button";

export default function NotFound() {
  return (
    <main id="main-content" className="terios-grain flex min-h-[100dvh] items-center justify-center overflow-hidden px-6 py-20">
      <div className="relative max-w-xl text-center">
        <span className="mx-auto flex size-16 items-center justify-center rounded-[1.25rem] bg-eucalyptus-100 text-primary"><Compass size={26} aria-hidden="true" /></span>
        <p className="mt-8 text-xs font-semibold tracking-[0.1em] text-primary uppercase">Page not found</p>
        <h1 className="mt-3 font-display text-5xl font-semibold tracking-[-0.035em] text-ink">This path has gone quiet.</h1>
        <p className="mx-auto mt-5 max-w-[48ch] text-base leading-relaxed text-ink-muted">The page may have moved or no longer exists. Your next step is still close by.</p>
        <Link href="/" className={buttonClasses({ size: "lg", className: "mt-8" })}><ArrowLeft size={17} aria-hidden="true" />Return home</Link>
      </div>
    </main>
  );
}
