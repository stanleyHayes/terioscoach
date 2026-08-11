import type { Metadata } from "next";
import Link from "next/link";
import { Leaf } from "lucide-react";
import { Section } from "@/components/marketing/Section";
import { buttonClasses } from "@/components/ui/Button";
import { listServices } from "@/lib/api";
import type { ServiceSummary } from "@/lib/api";
import { formatDuration, formatMoney } from "@/lib/format";

export const metadata: Metadata = {
  title: "Services",
  description:
    "One-to-one nursing consultations, wellness coaching and recovery programs — live pricing, by video, wherever you are.",
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
      {/* Page header — eyebrow/display pattern per SectionHeading (h1 here). */}
      <Section containerClassName="pb-0 lg:pb-0">
        <div className="max-w-[68ch]">
          <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-muted">
            Services
          </p>
          <h1 className="mt-4 font-display text-[2.5rem] leading-[1.1] font-medium tracking-[-0.015em] text-ink lg:text-[3rem] [text-wrap:balance]">
            Every session, clearly priced
          </h1>
          <p className="mt-6 max-w-[60ch] text-lg leading-[1.6] text-ink-muted [text-wrap:pretty]">
            One-to-one care by video — nursing, coaching and recovery. Prices
            come live from the practice, so what you see here is what you
            book.
          </p>
        </div>
      </Section>

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
          <ul className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
            {services.map((service) => (
              <li key={service.id}>
                <div className="flex h-full flex-col rounded-xl border border-border bg-surface-raised p-8">
                  <h3 className="font-display text-2xl leading-[1.2] font-medium tracking-[-0.01em] text-ink [text-wrap:pretty]">
                    {service.name}
                  </h3>
                  <p className="mt-3 flex-1 text-sm leading-[1.55] text-ink-muted">
                    {service.description}
                  </p>
                  {/* Price emphasis: Fraunces, tabular numerals. */}
                  <p className="mt-8 text-sm text-ink-muted">
                    {formatDuration(service.durationMinutes)}
                    <span aria-hidden="true" className="mx-2 text-ink-faint">
                      ·
                    </span>
                    <span className="font-display text-xl font-medium tabular-nums text-ink">
                      {formatMoney(service.priceKobo, service.currency)}
                    </span>
                  </p>
                  <div className="mt-6">
                    <Link
                      href={`/work-with-me?service=${service.id}`}
                      className={buttonClasses({
                        variant: "secondary",
                        fullWidth: true,
                      })}
                    >
                      Book this
                    </Link>
                  </div>
                </div>
              </li>
            ))}
          </ul>
        )}
      </Section>
    </>
  );
}
