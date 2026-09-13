"use client";
import { useSiteCopy } from "@/lib/site-copy-context";
import { ArrowUpRight, HeartPulse } from "lucide-react";
import { ContentImage as Image } from "@/components/content/ContentImage";
import Link from "next/link";
import { buttonClasses } from "@/components/ui/Button";





function FooterRail({ label, links }: { label: string; links: Array<{ href: string; label: string }> }) {
  return (
    <nav aria-label={label} className="border-t border-sand-0/12 py-5">
      <div className="grid gap-4 sm:grid-cols-[7rem_1fr] sm:items-start">
        <h2 className="text-[10px] font-semibold uppercase tracking-[0.16em] text-eucalyptus-300">{label}</h2>
        <ul className="flex flex-wrap gap-x-6 gap-y-3">
          {links.map((link) => (
            <li key={link.href}>
              <Link href={link.href} className="group inline-flex items-center gap-1.5 text-sm text-eucalyptus-100 transition-colors duration-fast hover:text-sand-0">
                {link.label}<ArrowUpRight aria-hidden="true" className="size-3 text-eucalyptus-400 transition-transform duration-fast group-hover:-translate-y-0.5 group-hover:translate-x-0.5" />
              </Link>
            </li>
          ))}
        </ul>
      </div>
    </nav>
  );
}

export function SiteFooter() {
  const copy = useSiteCopy("footer");

const practiceLinks = [
  { href: copy("field-008", "/services"), label: copy("field-009", "Services") },
  { href: copy("field-010", "/work-with-me"), label: copy("field-011", "Work with me") },
  { href: copy("field-012", "/contact"), label: copy("field-013", "Contact") },
];

const exploreLinks = [
  { href: "/", label: copy("field-001", "Home") },
  { href: copy("field-002", "/about"), label: copy("field-003", "About") },
  { href: copy("field-004", "/blog"), label: copy("field-005", "Journal") },
  { href: copy("field-006", "/faq"), label: copy("field-007", "FAQ") },
];

  const year = new Date().getFullYear();

  return (
    <footer className="relative overflow-hidden rounded-t-[2.75rem] bg-eucalyptus-900 text-sand-0 lg:rounded-t-[4rem]">
      <div aria-hidden="true" className="pointer-events-none absolute -right-24 top-24 size-80 rounded-full border border-eucalyptus-700/70" />
      <div aria-hidden="true" className="pointer-events-none absolute -right-10 top-38 size-40 rounded-full border border-clay-300/25" />

      <div className="mx-auto max-w-[1200px] px-6 pb-8 pt-8 lg:px-12 lg:pt-12">
        <section className="terios-footer-callout relative overflow-hidden rounded-[2rem] bg-sand-0 px-7 py-9 text-eucalyptus-900 sm:px-10 lg:grid lg:grid-cols-[1fr_auto] lg:items-end lg:gap-12 lg:px-12 lg:py-11">
          <div aria-hidden="true" className="absolute right-8 top-7 hidden size-16 items-center justify-center rounded-full border border-eucalyptus-200 text-eucalyptus-500 sm:flex"><HeartPulse className="size-6" /></div>
          <div>
            <p className="text-[10px] font-semibold uppercase tracking-[0.16em] text-eucalyptus-600">{copy("field-014", "Your next chapter")}</p>
            <h2 className="mt-4 max-w-[15ch] font-display text-[clamp(2.2rem,5vw,4.6rem)] font-semibold leading-[.96] tracking-[-0.055em]">{copy("field-015", "Care can begin with one calm conversation.")}</h2>
          </div>
          <Link href={copy("field-016", "/work-with-me")} className={buttonClasses({ size: "lg", className: "mt-8 lg:mt-0" })}>{copy("field-017", "Book a session")}</Link>
        </section>

        <div className="grid gap-12 pb-12 pt-16 lg:grid-cols-[1.1fr_.9fr] lg:gap-24 lg:pb-16 lg:pt-20">
          <div>
            <Link href="/" aria-label="Terios Wellness" className="group inline-flex items-center gap-4 text-sand-0">
              <span className="flex size-14 items-center justify-center rounded-[1.1rem] bg-sand-0 p-1.5 transition-transform duration-base group-hover:-rotate-2"><Image unoptimized src={copy("field-018", "/images/brand/identity/terios-mark.svg")} alt="" width={44} height={66} className="h-full w-auto" /></span>
              <span className="font-display text-3xl font-semibold tracking-[-0.04em]">{copy("field-019", "Terios ")}<span className="font-medium text-eucalyptus-300">{copy("field-020", "Wellness")}</span></span>
            </Link>
            <p className="mt-6 max-w-[39ch] text-base leading-[1.75] text-eucalyptus-200">{copy("field-021", "Registered nursing and wellness coaching, held in one private, video-first practice.")}</p>
            <address className="mt-5 flex flex-col gap-2 text-sm not-italic text-eucalyptus-100">
              <a href={copy("field-022", "mailto:Hello@terioscoach.com")} className="transition-colors hover:text-sand-0">
                {copy("field-023", "Hello@terioscoach.com ")}</a>
              <a href={copy("field-024", "tel:+19293748914")} className="transition-colors hover:text-sand-0">
                {copy("field-025", "+1 (929) 374-8914 ")}</a>
            </address>
            <ul aria-label="Practice qualities" className="mt-8 flex flex-wrap gap-2 text-[11px] font-semibold uppercase tracking-[0.1em] text-eucalyptus-200">
              {[copy("quality-0", "Nurse-led"), copy("quality-1", "Worldwide"), copy("quality-2", "Private by design")].map((item) => <li key={item} className="rounded-full border border-sand-0/14 px-3 py-2">{item}</li>)}
            </ul>
          </div>

          <div className="self-end">
            <FooterRail label={copy("field-026", "Explore")} links={exploreLinks} />
            <FooterRail label={copy("field-027", "Practice")} links={practiceLinks} />
          </div>
        </div>

        <div className="grid gap-4 border-t border-sand-0/12 pt-6 text-xs text-eucalyptus-300 sm:grid-cols-[1fr_auto_1fr] sm:items-center">
          <p>© {year} {copy("field-028", "Terios Wellness Spa")}</p>
          <nav aria-label="Legal" className="flex items-center gap-5"><Link href={copy("field-029", "/privacy")} className="transition-colors hover:text-sand-0">{copy("field-030", "Privacy")}</Link><Link href={copy("field-031", "/terms")} className="transition-colors hover:text-sand-0">{copy("field-032", "Terms")}</Link></nav>
          <div className="flex flex-col gap-2 sm:items-end sm:text-right">
            <p>{copy("field-033", "Clinical calm, wherever you are. ")}<span aria-hidden="true" className="ml-2 text-clay-300">❦</span></p>
            <a href={copy("field-034", "https://xcreativs.com")} target="_blank" rel="noopener noreferrer" className="font-semibold uppercase tracking-[0.1em] text-eucalyptus-200 transition-colors hover:text-sand-0">{copy("field-035", "DEVELOPED BY XCREATIVS TECHNOLOGIES LTD")}</a>
            <p className="font-semibold uppercase tracking-[0.14em] text-eucalyptus-300">{copy("field-036", "Powered by XHI")}</p>
          </div>
        </div>
      </div>
    </footer>
  );
}
