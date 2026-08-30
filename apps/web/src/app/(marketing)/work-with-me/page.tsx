import type { Metadata } from "next";
import Link from "next/link";
import Image from "next/image";
import {
  ArrowUpRight,
  CalendarCheck,
  Clock3,
  CreditCard,
  ListChecks,
} from "lucide-react";
import { Section } from "@/components/marketing/Section";
import { PageIntro } from "@/components/marketing/PageIntro";
import { SectionHeading } from "@/components/marketing/SectionHeading";
import { buttonClasses } from "@/components/ui/Button";
import { EmptyState } from "@/components/ui/EmptyState";
import { listServices } from "@/lib/api";
import { cn } from "@/lib/cn";
import { formatDuration, formatMoney } from "@/lib/format";
import { getPage } from "@/lib/content";

export const metadata: Metadata = {
  title: "Work With Me",
  description:
    "Choose a service, pick a time, confirm and pay — your Terios account is created during booking.",
  alternates: { canonical: "/work-with-me" },
  openGraph: {
    type: "website",
    url: "/work-with-me",
    title: "Work With Me",
  },
};

// Same live catalog as /services — fetched at request time (no-store), so
// prices shown here always match the dashboard.
export const dynamic = "force-dynamic";

const steps = [
  {
    icon: ListChecks,
    title: "Choose a service",
    body: "Pick the care that fits the season you are in. Every service is one-to-one and by video.",
  },
  {
    icon: CalendarCheck,
    title: "Pick a time",
    body: "A live calendar shows real openings with a clear timezone, so nothing gets lost in translation.",
  },
  {
    icon: CreditCard,
    title: "Confirm & pay",
    body: "Review the details, pay securely, and your session is booked. Confirmation lands in your inbox.",
  },
];

export default async function WorkWithMePage({
  searchParams,
}: {
  searchParams: Promise<{ service?: string | string[] }>;
}) {
  // ?service=<id> (from a "Book this" card on /services) pre-highlights the
  // chosen service in the list below.
  const { service: serviceParam } = await searchParams;
  const selectedId =
    typeof serviceParam === "string" ? serviceParam : undefined;

  const [services, page] = await Promise.all([
    listServices().catch(() => null),
    getPage("work-with-me").catch(() => undefined),
  ]);

  return (
    <>
      <PageIntro
        eyebrow="Work with me"
        title="Begin with a single step"
        description={
          page?.body ||
          "Choose the care that fits, pick a time, confirm and pay. There is no separate sign-up — your account is created while you book, and it becomes your private client portal."
        }
      />

      <Section containerClassName="pt-0 pb-0">
        <div className="relative aspect-[3/2] max-h-[520px] overflow-hidden rounded-[2rem] bg-eucalyptus-50 lg:aspect-[21/9]">
          <Image
            src={page?.coverImage || "/images/marketing/services-care.webp"}
            alt="A calm virtual wellness consultation setting"
            fill
            unoptimized={Boolean(page?.coverImage?.startsWith("http"))}
            sizes="(min-width: 1280px) 1200px, 94vw"
            className="object-cover"
          />
        </div>
      </Section>

      {/* Service chooser — compact rows from the same live catalog. Selected
          card treatment per design-system §3.8 (RadioCard): 1.5px primary
          border + eucalyptus-50. */}
      <Section ariaLabelledby="choose-service-heading">
        <SectionHeading
          id="choose-service-heading"
          eyebrow="Step one"
          title="Choose your service"
          description="Live from the practice menu — durations and prices are always current."
        />
        {services === null ? (
          <div
            role="alert"
            className="mx-auto mt-12 max-w-[480px] rounded-xl border border-border bg-surface-raised p-8 text-center"
          >
            <h3 className="font-display text-xl leading-[1.3] font-medium text-ink">
              The service menu didn&rsquo;t load
            </h3>
            <p className="mt-3 text-sm leading-[1.55] text-ink-muted">
              Something interrupted the connection on our side. Try again in a
              moment.
            </p>
            <div className="mt-6">
              <Link
                href="/work-with-me"
                className={buttonClasses({ variant: "secondary" })}
              >
                Try again
              </Link>
            </div>
          </div>
        ) : services.length === 0 ? (
          <div className="mt-12">
            <EmptyState
              icon={<ListChecks className="size-8" />}
              title="The service menu is being refreshed"
              description="When a service is published, it appears here. Check back soon."
            />
          </div>
        ) : (
          <ul className="mt-12 flex flex-col gap-4">
            {services.map((service, index) => {
              const selected = service.id === selectedId;
              return (
                <li key={service.id}>
                  <div
                    aria-current={selected || undefined}
                    className={cn(
                      "terios-choice-card group relative grid overflow-hidden border sm:grid-cols-[4.5rem_minmax(0,1fr)_auto]",
                      selected
                        ? "is-selected border-eucalyptus-800 bg-eucalyptus-900 text-sand-0"
                        : "border-border/80 bg-surface-raised text-ink",
                    )}
                  >
                    <span
                      className="terios-choice-index font-display"
                      aria-hidden="true"
                    >
                      {String(index + 1).padStart(2, "0")}
                    </span>
                    <div className="min-w-0 px-5 py-5 sm:px-6 sm:py-7">
                      <div className="flex flex-wrap items-center gap-3">
                        <h3 className="font-display text-[1.35rem] font-medium leading-[1.2] tracking-[-0.01em]">
                          {service.name}
                        </h3>
                        {selected && (
                          <span className="text-[11px] font-semibold uppercase tracking-[0.08em] text-primary">
                            Selected
                          </span>
                        )}
                      </div>
                      <p className="mt-2 text-sm leading-[1.55] text-ink-muted group-[.is-selected]:text-eucalyptus-100">
                        {service.description}
                      </p>
                      <p className="mt-4 inline-flex items-center gap-2 rounded-full bg-surface-sunken px-3 py-1.5 text-xs font-semibold text-ink-muted group-[.is-selected]:bg-white/10 group-[.is-selected]:text-sand-100">
                        <Clock3 size={14} aria-hidden="true" />
                        {formatDuration(service.durationMinutes)}
                      </p>
                    </div>
                    <div className="flex items-center justify-between gap-5 border-t border-border/70 px-5 py-4 sm:flex-col sm:items-end sm:justify-between sm:border-t-0 sm:border-l sm:px-6 sm:py-7 group-[.is-selected]:border-white/15">
                      <span className="font-display text-xl font-medium tabular-nums">
                        {formatMoney(service.priceKobo, service.currency)}
                      </span>
                      {/* Straight into the booking flow (WEB-09), service
                          preselected. */}
                      <Link
                        href={`/portal/book?service=${service.id}`}
                        className={cn(
                          buttonClasses({ size: "sm" }),
                          selected &&
                            "border-white/20 bg-sand-0 text-eucalyptus-900 hover:bg-sand-100",
                        )}
                      >
                        Choose <ArrowUpRight size={15} aria-hidden="true" />
                      </Link>
                    </div>
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </Section>

      {/* How booking works — three quiet steps. */}
      <Section background="sunken" ariaLabelledby="how-booking-works-heading">
        <SectionHeading
          id="how-booking-works-heading"
          eyebrow="How booking works"
          title="Three steps, no paperwork"
          align="center"
        />
        <ol className="mx-auto mt-12 grid max-w-[960px] gap-10 md:grid-cols-3 md:gap-6">
          {steps.map((step, index) => (
            <li key={step.title} className="flex flex-col items-start">
              <span className="flex h-10 w-10 items-center justify-center rounded-full bg-eucalyptus-50 text-primary">
                <step.icon aria-hidden="true" className="size-5" />
              </span>
              <h3 className="mt-5 text-base font-semibold leading-[1.4] text-ink">
                <span className="text-ink-faint">{index + 1}. </span>
                {step.title}
              </h3>
              <p className="mt-2 text-sm leading-[1.55] text-ink-muted">
                {step.body}
              </p>
            </li>
          ))}
        </ol>
        <p className="mx-auto mt-10 max-w-[60ch] text-center text-sm leading-[1.55] text-ink-muted">
          No account is needed before you start — you create yours during
          booking, and it becomes your private client portal.
        </p>
      </Section>
    </>
  );
}
