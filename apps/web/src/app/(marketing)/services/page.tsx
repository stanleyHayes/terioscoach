import type { Metadata } from "next";
import Link from "next/link";
import { Leaf } from "lucide-react";
import { Section } from "@/components/marketing/Section";
import { PageIntro } from "@/components/marketing/PageIntro";
import { buttonClasses } from "@/components/ui/Button";
import { listServices } from "@/lib/api";
import type { ServiceSummary } from "@/lib/api";
import { formatDuration, formatMoney } from "@/lib/format";

export const metadata: Metadata = {
  title: "Services",
  description:
    "One-to-one nursing consultations, wellness coaching and recovery programs — live pricing, by video, wherever you are.",
  alternates: { canonical: "/services" },
  openGraph: {
    type: "website",
    url: "/services",
    title: "Services",
  },
};

// Fetched at request time (listServices uses cache: "no-store"): prices and
// the catalog are edited in the practitioner dashboard, and this page must
// always show current values — never a statically cached render.
export const dynamic = "force-dynamic";

export default async function ServicesPage() {
  let services: ServiceSummary[] | null;
  try {
    services = await listServices();
  } catch {
    // A failed fetch renders a branded inline error below — never a crash.
    services = null;
  }

  return (
    <>
      <PageIntro eyebrow="Services" title="Every session, clearly priced" description="One-to-one care by video — nursing, coaching and recovery. Prices come live from the practice, so what you see here is what you book." />

      {/* Service menu — marketing feature cards (design-system §3.21):
          radius-xl, space-8 padding. The card itself is not a link because
          it holds its own "Book this" action (§3.21: no nested interactive
          elements). */}
      <Section ariaLabelledby="service-menu-heading">
        <h2 id="service-menu-heading" className="sr-only">
          Service menu
        </h2>
        {services === null ? (
          <div
            role="alert"
            className="mx-auto max-w-[480px] rounded-xl border border-border bg-surface-raised p-8 text-center"
          >
            <h3 className="font-display text-xl leading-[1.3] font-medium text-ink">
              The service menu didn&rsquo;t load
            </h3>
            <p className="mt-3 text-sm leading-[1.55] text-ink-muted">
              Something interrupted the connection on our side. Nothing is
              wrong on yours — try again in a moment.
            </p>
            <div className="mt-6">
              <Link
                href="/services"
                className={buttonClasses({ variant: "secondary" })}
              >
                Try again
              </Link>
            </div>
          </div>
        ) : services.length === 0 ? (
          /* EmptyState (design-system §3.27). */
          <div className="mx-auto flex max-w-[360px] flex-col items-center px-6 py-12 text-center">
            <span className="flex h-16 w-16 items-center justify-center rounded-full bg-surface-sunken">
              <Leaf aria-hidden="true" className="size-8 text-ink-faint" />
            </span>
            <h3 className="mt-5 font-display text-xl leading-[1.3] font-medium text-ink">
              The menu is being refreshed
            </h3>
            <p className="mt-2 text-sm leading-[1.55] text-ink-muted">
              When a service is published, it appears here. Check back soon.
            </p>
          </div>
        ) : (
          <ul className="divide-y divide-border border-y border-border">
            {services.map((service, index) => (
              <li key={service.id} className="terios-service-row group grid gap-6 py-9 transition-[background-color,transform] duration-base hover:bg-eucalyptus-50/60 sm:grid-cols-[4rem_minmax(0,1fr)_auto] sm:px-5 lg:items-center lg:gap-10">
                <span aria-hidden="true" className="font-mono text-xs text-ink-faint">{String(index + 1).padStart(2, "0")}</span>
                <div className="max-w-[46rem]">
                  <h3 className="font-display text-[clamp(1.7rem,3vw,2.5rem)] leading-[1.05] font-semibold tracking-[-0.03em] text-ink">{service.name}</h3>
                  <p className="mt-3 text-sm leading-[1.65] text-ink-muted">{service.description}</p>
                  <p className="mt-5 text-sm text-ink-muted">
                    {formatDuration(service.durationMinutes)}
                    <span aria-hidden="true" className="mx-2 text-ink-faint">
                      ·
                    </span>
                    <span className="font-display text-lg font-semibold tabular-nums text-ink">
                      {formatMoney(service.priceKobo, service.currency)}
                    </span>
                  </p>
                </div>
                  <div className="self-center sm:justify-self-end">
                    <Link
                      href={`/work-with-me?service=${service.id}`}
                      className={buttonClasses({
                        variant: "secondary",
                        className: "sm:min-w-32",
                      })}
                    >
                      Book this
                    </Link>
                  </div>
              </li>
            ))}
          </ul>
        )}
      </Section>
    </>
  );
}
