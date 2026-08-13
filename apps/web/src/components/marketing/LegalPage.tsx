import type { LucideIcon } from "lucide-react";
import { ArrowUpRight, CalendarDays, Mail, ShieldCheck } from "lucide-react";
import Link from "next/link";
import { PageIntro } from "@/components/marketing/PageIntro";
import { Section } from "@/components/marketing/Section";

export interface LegalSection {
  title: string;
  body: string;
  icon: LucideIcon;
}

interface LegalPageProps {
  eyebrow: string;
  title: string;
  description: string;
  summary: string;
  notice: string;
  sections: readonly LegalSection[];
  relatedHref: "/privacy" | "/terms";
  relatedLabel: string;
}

function sectionId(title: string) {
  return title.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/(^-|-$)/g, "");
}

export function LegalPage({
  eyebrow,
  title,
  description,
  summary,
  notice,
  sections,
  relatedHref,
  relatedLabel,
}: LegalPageProps) {
  return (
    <>
      <PageIntro eyebrow={eyebrow} title={title} description={description} />
      <Section ariaLabelledby="legal-overview-heading">
        <article className="mx-auto max-w-[1120px]">
          <div className="terios-legal-overview overflow-hidden rounded-[2rem] border border-eucalyptus-800 bg-eucalyptus-900 text-sand-0 shadow-[0_28px_90px_rgba(28,51,40,.16)]">
            <div className="grid gap-8 px-6 py-8 sm:px-9 sm:py-10 lg:grid-cols-[1fr_auto] lg:items-end lg:px-12">
              <div className="max-w-[720px]">
                <span className="inline-flex items-center gap-2 rounded-full border border-white/15 bg-white/8 px-3 py-1.5 text-[11px] font-semibold uppercase tracking-[0.12em] text-eucalyptus-100">
                  <ShieldCheck size={14} aria-hidden="true" />
                  Plain-language policy
                </span>
                <h2 id="legal-overview-heading" className="mt-6 font-display text-[clamp(1.65rem,3vw,2.5rem)] leading-[1.12] font-medium tracking-[-0.025em] text-sand-0">
                  The important part, up front.
                </h2>
                <p className="mt-4 max-w-[62ch] text-base leading-[1.75] text-eucalyptus-100">
                  {summary}
                </p>
              </div>
              <div className="flex items-center gap-3 border-t border-white/12 pt-5 text-sm text-eucalyptus-100 lg:border-t-0 lg:border-l lg:py-2 lg:pl-8">
                <CalendarDays size={18} aria-hidden="true" className="text-clay-200" />
                <span><span className="block text-[10px] font-semibold uppercase tracking-[0.12em] text-eucalyptus-300">Last reviewed</span>12 August 2026</span>
              </div>
            </div>
            <p className="border-t border-white/12 bg-black/8 px-6 py-4 text-[13px] leading-[1.6] text-eucalyptus-200 sm:px-9 lg:px-12">
              {notice}
            </p>
          </div>

          <div className="mt-14 grid gap-10 lg:grid-cols-[15rem_minmax(0,1fr)] lg:gap-16">
            <aside className="lg:sticky lg:top-28 lg:self-start">
              <p className="text-[11px] font-semibold uppercase tracking-[0.12em] text-primary">On this page</p>
              <nav aria-label={`${eyebrow} contents`} className="mt-5 border-l border-eucalyptus-200">
                {sections.map((section, index) => (
                  <a key={section.title} href={`#${sectionId(section.title)}`} className="group flex items-center gap-3 border-l-2 border-transparent py-2.5 pl-4 text-sm text-ink-muted transition-colors hover:border-primary hover:text-ink">
                    <span className="font-display text-xs tabular-nums text-ink-faint group-hover:text-primary">{String(index + 1).padStart(2, "0")}</span>
                    {section.title}
                  </a>
                ))}
              </nav>
              <Link href={relatedHref} className="mt-7 inline-flex items-center gap-2 text-sm font-semibold text-primary hover:text-primary-hover">
                {relatedLabel}<ArrowUpRight size={15} aria-hidden="true" />
              </Link>
            </aside>

            <div className="flex min-w-0 flex-col gap-5">
              {sections.map((section, index) => {
                const Icon = section.icon;
                return (
                  <section id={sectionId(section.title)} key={section.title} className="terios-legal-chapter scroll-mt-28 overflow-hidden border border-border/75 bg-surface-raised">
                    <div className="grid sm:grid-cols-[5rem_minmax(0,1fr)]">
                      <div className="flex items-center justify-between border-b border-border/70 bg-eucalyptus-50 px-5 py-4 sm:flex-col sm:items-center sm:justify-start sm:gap-5 sm:border-r sm:border-b-0 sm:px-3 sm:py-7">
                        <span className="font-display text-sm font-semibold tabular-nums text-primary">{String(index + 1).padStart(2, "0")}</span>
                        <span className="flex size-9 items-center justify-center rounded-full bg-surface-raised text-primary shadow-sm"><Icon size={17} aria-hidden="true" /></span>
                      </div>
                      <div className="px-6 py-6 sm:px-8 sm:py-8">
                        <h2 className="font-display text-[1.45rem] leading-[1.2] font-medium tracking-[-0.02em] text-ink">{section.title}</h2>
                        <p className="mt-3 max-w-[66ch] text-base leading-[1.8] text-ink-muted">{section.body}</p>
                      </div>
                    </div>
                  </section>
                );
              })}

              <section className="mt-3 flex flex-col gap-5 rounded-[1.5rem_2.5rem_1.5rem_1.5rem] border border-clay-100 bg-clay-50 p-6 sm:flex-row sm:items-center sm:justify-between sm:p-8" aria-labelledby="legal-question-heading">
                <div>
                  <p className="text-[11px] font-semibold uppercase tracking-[0.12em] text-clay-700">Still unsure?</p>
                  <h2 id="legal-question-heading" className="mt-2 font-display text-xl font-medium text-ink">Ask before you continue.</h2>
                  <p className="mt-2 text-sm leading-[1.6] text-ink-muted">We would rather explain something clearly than leave you guessing.</p>
                </div>
                <Link href="/contact" className="terios-button terios-button-primary relative inline-flex min-h-11 shrink-0 items-center justify-center gap-2 rounded-full bg-eucalyptus-900 px-5 text-sm font-semibold text-sand-0 transition-transform hover:-translate-y-0.5">
                  <Mail size={16} aria-hidden="true" /> Contact the practice
                </Link>
              </section>
            </div>
          </div>
        </article>
      </Section>
    </>
  );
}
