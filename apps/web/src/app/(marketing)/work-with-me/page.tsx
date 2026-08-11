import type { Metadata } from "next";
import Link from "next/link";
import { CalendarCheck, CreditCard, ListChecks } from "lucide-react";
import { Section } from "@/components/marketing/Section";
import { SectionHeading } from "@/components/marketing/SectionHeading";
import { buttonClasses } from "@/components/ui/Button";
import { listServices } from "@/lib/api";
import type { ServiceSummary } from "@/lib/api";
import { cn } from "@/lib/cn";
import { formatDuration, formatMoney } from "@/lib/format";

export const metadata: Metadata = {
  title: "Work With Me",
  description:
    "Choose a service, pick a time, confirm and pay — your Terios account is created during booking.",
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
  const selectedId = typeof serviceParam === "string" ? serviceParam : undefined;

  let services: ServiceSummary[] | null;
  try {
    services = await listServices();
  } catch {
    services = null;
  }

  return (
    <>
      {/* Page header. */}
      <Section containerClassName="pb-0 lg:pb-0">
        <div className="max-w-[68ch]">
          <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-muted">
            Work with me
          </p>
          <h1 className="mt-4 font-display text-[2.5rem] leading-[1.1] font-medium tracking-[-0.015em] text-ink lg:text-[3rem] [text-wrap:balance]">
            Begin with a single step
          </h1>
          <p className="mt-6 max-w-[60ch] text-lg leading-[1.6] text-ink-muted [text-wrap:pretty]">
            Choose the care that fits, pick a time, confirm and pay. There is
            no separate sign-up — your account is created while you book, and
            it becomes your private client portal.
          </p>
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
          <p className="mt-12 text-sm leading-[1.55] text-ink-muted">
            The service menu is being refreshed. Check back soon.
          </p>
        ) : (
          <ul className="mt-12 flex flex-col gap-4">
            {services.map((service) => {
              const selected = service.id === selectedId;
              return (
                <li key={service.id}>
                  <div
                    aria-current={selected || undefined}
                    className={cn(
                      "flex flex-col gap-5 rounded-lg border bg-surface-raised p-6 sm:flex-row sm:items-center sm:justify-between",
                      selected
                        ? "border-[1.5px] border-primary bg-eucalyptus-50"
                        : "border-border",
                    )}
                  >
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-3">
                        <h3 className="text-base font-semibold leading-[1.4] text-ink">
                          {service.name}
                        </h3>
                        {selected && (
                          <span className="text-[11px] font-semibold uppercase tracking-[0.08em] text-primary">
                            Selected
                          </span>
                        )}
                      </div>
                      <p className="mt-1 text-sm leading-[1.55] text-ink-muted">
                        {service.description}
                      </p>
                      <p className="mt-2 text-sm text-ink-muted">
                        {formatDuration(service.durationMinutes)}
                        <span aria-hidden="true" className="mx-2 text-ink-faint">
                          ·
                        </span>
                        <span className="font-display text-base font-medium tabular-nums text-ink">
                          {formatMoney(service.priceKobo, service.currency)}
                        </span>
                      </p>
                    </div>
                    <div className="shrink-0">
                      {/* Placeholder anchor — the booking flow (WEB-09/CX-04)
                          mounts at #book below. */}
                      <Link href="#book" className={buttonClasses({ size: "sm" })}>
                        Choose
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

      {/* #book — target of every "Choose" above. Structure only: the live
          calendar and secure checkout arrive with the booking flow
          (WEB-09/CX-04). */}
      <Section id="book" ariaLabelledby="book-heading">
        <div className="mx-auto max-w-[480px] rounded-xl border border-border bg-surface-raised p-8 text-center">
          <h2
            id="book-heading"
            className="font-display text-xl leading-[1.3] font-medium text-ink"
          >
            Booking opens here
          </h2>
          <p className="mt-3 text-sm leading-[1.55] text-ink-muted">
            The live calendar and secure checkout are being finished now. When
            they arrive, this is where you will pick your time and confirm —
            your chosen service stays selected above.
          </p>
        </div>
      </Section>
    </>
  );
}
