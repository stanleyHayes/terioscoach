import type { Metadata } from "next";
import Link from "next/link";
import { HelpCircle } from "lucide-react";
import { FAQSearch } from "@/components/content/FAQSearch";
import { Section } from "@/components/marketing/Section";
import { PageIntro } from "@/components/marketing/PageIntro";
import { buttonClasses } from "@/components/ui/Button";
import { listFAQs, type FAQ } from "@/lib/content";

export const metadata: Metadata = {
  title: "Questions",
  description:
    "Booking, payment, first sessions and what to expect — the questions people ask before working with Terios, answered plainly.",
  alternates: { canonical: "/faq" },
  openGraph: {
    type: "website",
    url: "/faq",
    title: "Questions",
  },
};

export const dynamic = "force-dynamic";

export default async function FAQPage() {
  let faqs: FAQ[] | null;
  try {
    faqs = await listFAQs();
  } catch {
    faqs = null;
  }

  return (
    <>
      <PageIntro eyebrow="Questions" title="The things people ask first" description="Booking, payment, what a first session is like. If yours is not here, ask it — a real answer comes back." />

      <Section ariaLabelledby="faq-heading">
        <h2 id="faq-heading" className="sr-only">
          Frequently asked questions
        </h2>

        {faqs === null ? (
          <div
            role="alert"
            className="mx-auto max-w-[480px] rounded-xl border border-border bg-surface-raised p-8 text-center"
          >
            <h3 className="font-display text-xl leading-[1.3] font-medium text-ink">
              The questions didn&rsquo;t load
            </h3>
            <p className="mt-3 text-sm leading-[1.55] text-ink-muted">
              Something interrupted the connection on our side. Nothing is
              wrong on yours — try again in a moment.
            </p>
            <div className="mt-6">
              <Link href="/faq" className={buttonClasses({ variant: "secondary" })}>
                Try again
              </Link>
            </div>
          </div>
        ) : faqs.length === 0 ? (
          <div className="mx-auto flex max-w-[360px] flex-col items-center px-6 py-12 text-center">
            <span className="flex size-16 items-center justify-center rounded-full bg-surface-sunken">
              <HelpCircle aria-hidden="true" className="size-8 text-ink-faint" />
            </span>
            <h3 className="mt-5 font-display text-xl leading-[1.3] font-medium text-ink">
              No questions published yet
            </h3>
            <p className="mt-2 text-sm leading-[1.55] text-ink-muted">
              Ask yours directly and it will be answered — and probably end up
              here.
            </p>
            <div className="mt-6">
              <Link href="/contact" className={buttonClasses({ size: "sm" })}>
                Ask a question
              </Link>
            </div>
          </div>
        ) : (
          <FAQSearch faqs={faqs} />
        )}
      </Section>

      <Section background="sunken">
        <div className="mx-auto max-w-[560px] text-center">
          <h2 className="font-display text-[2rem] leading-[1.15] font-medium tracking-[-0.01em] text-ink [text-wrap:balance]">
            Still wondering something?
          </h2>
          <p className="mt-4 text-base leading-[1.6] text-ink-muted [text-wrap:pretty]">
            Send it over. Questions are answered by the practitioner, not a
            form letter.
          </p>
          <div className="mt-8">
            <Link href="/contact" className={buttonClasses()}>
              Get in touch
            </Link>
          </div>
        </div>
      </Section>
    </>
  );
}
