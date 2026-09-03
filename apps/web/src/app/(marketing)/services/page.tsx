import type { Metadata } from "next";
import Image from "next/image";
import Link from "next/link";
import { Leaf } from "lucide-react";
import { Section } from "@/components/marketing/Section";
import { PageIntro } from "@/components/marketing/PageIntro";
import { buttonClasses } from "@/components/ui/Button";
import { EmptyState } from "@/components/ui/EmptyState";
import { listServices } from "@/lib/api";
import type { ServiceSummary } from "@/lib/api";
import { formatDuration, formatMoney } from "@/lib/format";
import { serviceImageFor } from "@/lib/service-imagery";

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
      <PageIntro
        eyebrow="Services"
        title="Every session, clearly priced"
        description="One-to-one care by video — nursing, coaching and recovery. Prices come live from the practice, so what you see here is what you book."
      />

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
              Something interrupted the connection on our side. Nothing is wrong
              on yours — try again in a moment.
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
          <EmptyState
            icon={<Leaf className="size-8" />}
            title="No services are published yet"
            description="The practice is preparing its bookable services. This page is working; there simply is not a live service to show right now."
            action={<Link href="/contact" className={buttonClasses({ variant: "secondary" })}>Ask about care</Link>}
          />
        ) : (
          <ul className="grid gap-6">
            {services.map((service, index) => (
              <li
                key={service.id}
                className="terios-service-row group grid overflow-hidden rounded-[2rem] border border-border bg-surface-raised shadow-[0_18px_55px_rgba(31,41,34,.05)] transition-[border-color,box-shadow,transform] duration-base hover:-translate-y-0.5 hover:border-eucalyptus-200 hover:shadow-md md:grid-cols-[minmax(260px,.72fr)_1.28fr]"
              >
                <div className="relative min-h-64 overflow-hidden bg-eucalyptus-100 md:min-h-80">
                  <Image
                    src={serviceImageFor(service.name, index)}
                    alt={`${service.name} at Terios Wellness`}
                    fill
                    loading={index === 0 ? "eager" : "lazy"}
                    sizes="(min-width: 768px) 38vw, 94vw"
                    className="object-cover transition-transform duration-page ease-out group-hover:scale-[1.025] motion-reduce:transition-none"
                  />
                  <span className="absolute left-5 top-5 rounded-full border border-sand-0/30 bg-eucalyptus-950/75 px-3 py-1.5 font-mono text-[11px] font-medium text-sand-0 backdrop-blur-md">
                    {String(index + 1).padStart(2, "0")}
                  </span>
                </div>
                <div className="flex flex-col justify-center p-7 sm:p-9 lg:p-12">
                  <h3 className="font-display text-[clamp(1.7rem,3vw,2.5rem)] leading-[1.05] font-semibold tracking-[-0.03em] text-ink">
                    {service.name}
                  </h3>
                  <p className="mt-3 text-sm leading-[1.65] text-ink-muted">
                    {service.description}
                  </p>
                  <p className="mt-6 border-t border-border pt-5 text-sm text-ink-muted">
                    {formatDuration(service.durationMinutes)}
                    <span aria-hidden="true" className="mx-2 text-ink-faint">
                      ·
                    </span>
                    <span className="font-display text-lg font-semibold tabular-nums text-ink">
                      {formatMoney(service.priceKobo, service.currency)}
                    </span>
                  </p>
                  <Link
                    href={`/work-with-me?service=${service.id}`}
                    className={buttonClasses({
                      variant: "secondary",
                      className: "mt-7 self-start sm:min-w-32",
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
