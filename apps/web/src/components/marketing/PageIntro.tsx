import { Leaf } from "lucide-react";

export function PageIntro({ eyebrow, title, description }: { eyebrow: string; title: string; description: string }) {
  return (
    <header className="relative overflow-hidden border-b border-eucalyptus-800 bg-eucalyptus-900 text-sand-0">
      <div aria-hidden="true" className="absolute inset-0 opacity-50 [background-image:radial-gradient(circle_at_82%_16%,rgba(157,195,174,.22),transparent_24rem),radial-gradient(circle_at_8%_110%,rgba(222,166,132,.14),transparent_28rem)]" />
      <svg aria-hidden="true" viewBox="0 0 1440 220" preserveAspectRatio="none" className="absolute inset-x-0 bottom-0 h-32 w-full text-eucalyptus-400/20">
        <path d="M-40 170C160 40 310 230 520 118S880 18 1030 116s250 82 470-18" fill="none" stroke="currentColor" strokeWidth="2" />
      </svg>
      <Leaf aria-hidden="true" strokeWidth={0.65} className="botanical-drift absolute -right-14 -top-12 size-72 rotate-[-24deg] text-eucalyptus-300/10" />
      <div className="relative mx-auto grid max-w-[1200px] gap-10 px-6 pb-20 pt-20 lg:grid-cols-[minmax(0,1fr)_18rem] lg:px-12 lg:pb-28 lg:pt-28">
        <div className="max-w-[52rem]">
          <p className="text-xs font-semibold tracking-[0.14em] text-eucalyptus-300 uppercase">{eyebrow}</p>
          <h1 className="mt-5 font-display text-[clamp(3rem,7vw,6.75rem)] leading-[.9] font-semibold tracking-[-0.055em] text-sand-0">{title}</h1>
          <p className="mt-8 max-w-[58ch] text-lg leading-[1.7] text-eucalyptus-200">{description}</p>
        </div>
        <div className="hidden self-end border-l border-sand-0/15 pl-6 lg:block">
          <p className="text-xs leading-relaxed text-eucalyptus-300">Terios field note</p>
          <p className="mt-2 text-sm leading-relaxed text-sand-0/75">Clinical care, recorded with warmth and kept intentionally personal.</p>
        </div>
      </div>
    </header>
  );
}
