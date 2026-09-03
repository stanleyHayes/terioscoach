import { HeartPulse, ShieldCheck } from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import type { ReactNode } from "react";

export function RecoveryShell({ eyebrow, title, description, children }: { eyebrow: string; title: string; description: string; children: ReactNode }) {
  return <main className="grid min-h-[100dvh] bg-eucalyptus-900 lg:grid-cols-[1.08fr_.92fr]">
    <aside className="relative hidden overflow-hidden p-12 text-sand-0 lg:flex lg:flex-col lg:justify-between">
      <div aria-hidden="true" className="absolute inset-0 [background-image:radial-gradient(circle_at_20%_15%,rgba(157,195,174,.18),transparent_26rem),radial-gradient(circle_at_85%_90%,rgba(222,166,132,.14),transparent_28rem)]" />
      <Link href="/login" className="relative inline-flex items-center gap-3 font-display text-2xl font-semibold tracking-[-0.035em]"><span className="flex size-10 items-center justify-center rounded-xl bg-sand-0 p-1.5"><Image src="/images/brand/identity/terios-mark.svg" alt="" width={24} height={36} className="h-full w-auto" priority /></span><span>Terios</span><span className="-ml-2 font-medium text-eucalyptus-300">Practice</span></Link>
      <div className="relative max-w-xl"><HeartPulse className="mb-8 size-8 text-eucalyptus-300" /><p className="font-display text-[4rem] leading-[.92] font-semibold tracking-[-0.055em]">Secure access, restored quietly.</p><p className="mt-7 max-w-[42ch] text-base leading-relaxed text-eucalyptus-200">Recovery links expire after one hour, work once, and close every previous session when your password changes.</p></div>
      <p className="relative flex items-center gap-2 text-xs text-eucalyptus-300"><ShieldCheck size={14} />Practitioner access only</p>
    </aside>
    <section className="flex items-start justify-center rounded-t-[2.5rem] bg-surface px-6 pb-12 pt-20 lg:items-center lg:rounded-l-[3rem] lg:rounded-tr-none lg:py-12"><div className="w-full max-w-[440px]"><div className="mb-9"><p className="text-xs font-semibold tracking-[0.12em] text-primary uppercase">{eyebrow}</p><h1 className="mt-3 font-display text-[2.75rem] leading-none font-semibold tracking-[-0.045em] text-ink">{title}</h1><p className="mt-3 text-sm leading-relaxed text-ink-muted">{description}</p></div>{children}</div></section>
  </main>;
}
