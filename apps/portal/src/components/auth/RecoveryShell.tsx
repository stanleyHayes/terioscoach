import { LockKeyhole } from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import type { ReactNode } from "react";

export function RecoveryShell({ eyebrow, title, description, children }: { eyebrow: string; title: string; description: string; children: ReactNode }) {
  return <main className="grid min-h-[100dvh] flex-1 bg-eucalyptus-900 lg:grid-cols-[minmax(22rem,.9fr)_1.1fr]">
    <aside className="relative hidden overflow-hidden border-r border-sand-0/10 p-12 text-sand-0 lg:flex lg:flex-col lg:justify-between">
      <div aria-hidden="true" className="absolute inset-0 [background-image:radial-gradient(circle_at_20%_15%,rgba(157,195,174,.2),transparent_25rem),radial-gradient(circle_at_80%_90%,rgba(222,166,132,.14),transparent_28rem)]" />
      <Link href="/" className="relative inline-flex items-center gap-3 font-display text-2xl font-semibold tracking-[-0.03em]"><span className="flex size-10 items-center justify-center rounded-xl bg-sand-0 p-1.5"><Image src="/images/brand/identity/terios-mark.svg" alt="" width={24} height={36} className="h-full w-auto" priority /></span>Terios Wellness</Link>
      <div className="relative max-w-md"><p className="font-display text-5xl leading-[.98] font-semibold tracking-[-0.045em]">A quiet way back into your care.</p><p className="mt-6 max-w-[40ch] text-base leading-relaxed text-eucalyptus-200">Recovery links expire quickly and can only be used once.</p></div>
      <p className="relative flex items-center gap-2 text-xs text-eucalyptus-300"><LockKeyhole size={14} />Private and encrypted</p>
    </aside>
    <section className="flex items-start justify-center rounded-t-[2.5rem] bg-surface px-6 pb-12 pt-20 lg:items-center lg:rounded-l-[3rem] lg:rounded-tr-none lg:py-12">
      <div className="w-full max-w-[440px]">
        <div className="mb-9"><p className="text-xs font-semibold tracking-[0.12em] text-primary uppercase">{eyebrow}</p><h1 className="mt-3 font-display text-[2.75rem] leading-none font-semibold tracking-[-0.04em] text-ink">{title}</h1><p className="mt-3 text-sm leading-relaxed text-ink-muted">{description}</p></div>
        {children}
      </div>
    </section>
  </main>;
}
