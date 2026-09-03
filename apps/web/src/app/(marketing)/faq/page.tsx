import type { Metadata } from "next";
import Image from "next/image";
import Link from "next/link";
import { HelpCircle } from "lucide-react";
import { FAQSearch } from "@/components/content/FAQSearch";
import { Section } from "@/components/marketing/Section";
import { PageIntro } from "@/components/marketing/PageIntro";
import { buttonClasses } from "@/components/ui/Button";
import { EmptyState } from "@/components/ui/EmptyState";
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
      <PageIntro
        eyebrow="Questions"
        title="The things people ask first"
        description="Booking, payment, what a first session is like. If yours is not here, ask it — a real answer comes back."
      />

      <Section containerClassName="pt-0 pb-0">
        <div className="relative min-h-64 overflow-hidden rounded-[2rem] bg-eucalyptus-100 sm:min-h-80 lg:min-h-[420px]">
          <Image src="/images/blog/lake-192979_1280.webp" alt="A quiet mountain lake reflecting the landscape" fill sizes="(min-width: 1280px) 1200px, 94vw" className="object-cover" />
          <div className="absolute inset-0 bg-gradient-to-r from-eucalyptus-950/70 via-eucalyptus-950/20 to-transparent" />
          <p className="absolute bottom-7 left-7 max-w-[24ch] font-display text-[clamp(1.6rem,4vw,2.6rem)] leading-tight text-sand-0 sm:bottom-10 sm:left-10">Clear answers make the next step feel lighter.</p>
        </div>
      </Section>

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
              Something interrupted the connection on our side. Nothing is wrong
              on yours — try again in a moment.
            </p>
            <div className="mt-6">
              <Link
                href="/faq"
                className={buttonClasses({ variant: "secondary" })}
              >
                Try again
              </Link>
            </div>
          </div>
        ) : faqs.length === 0 ? (
          <EmptyState
            icon={<HelpCircle className="size-8" />}
            title="No questions published yet"
            description="Ask yours directly and it will be answered — and probably end up here."
            action={
              <Link href="/contact" className={buttonClasses({ size: "sm" })}>
                Ask a question
              </Link>
            }
          />
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
            Send it over. Questions are answered by the practitioner, not a form
            letter.
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
