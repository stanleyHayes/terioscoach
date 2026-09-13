import { getSiteCopy } from "@/lib/site-copy";
import Link from "next/link";
import { ContentImage as Image } from "@/components/content/ContentImage";
import { ArrowRight, Globe, HeartPulse, ShieldCheck, Video } from "lucide-react";
import { Testimonials } from "@/components/content/Testimonials";
import { HeroWatermark } from "@/components/marketing/HeroWatermark";
import { Section } from "@/components/marketing/Section";
import { SectionHeading } from "@/components/marketing/SectionHeading";
import { buttonClasses } from "@/components/ui/Button";
import {
  getReviewSummary,
  getPage,
  listReviews,
  listTestimonials,
  type PublicReview,
  type ReviewSummary,
  type Testimonial,
} from "@/lib/content";
import { listServices, type ServiceSummary } from "@/lib/api";
import { serviceImageFor } from "@/lib/service-imagery";

// Testimonials and reviews are moderated in the dashboard and appear the
// moment they are approved, so this page is never statically cached.
export const dynamic = "force-dynamic";



/** Social proof, fetched together so the count beside the stars always
 * matches the list under them. Every route here returns approved content
 * only; a failure degrades to an empty section rather than a broken page. */
async function loadSocialProof(): Promise<{
  testimonials: Testimonial[];
  reviews: PublicReview[];
  summary: ReviewSummary | undefined;
}> {
  const [testimonials, reviews, summary] = await Promise.all([
    listTestimonials().catch(() => []),
    listReviews(4).catch(() => []),
    getReviewSummary().catch(() => undefined),
  ]);
  return { testimonials, reviews, summary };
}

export default async function Home() {
  const copy = await getSiteCopy("home");

const trustPoints = [
  {
    icon: Video,
    title: copy("field-001", "Sessions by video"),
    body: copy("field-002", "Care from your own space, on your schedule."),
  },
  {
    icon: Globe,
    title: copy("field-003", "Clients worldwide"),
    body: copy("field-004", "Time zones are a detail, not a barrier."),
  },
  {
    icon: ShieldCheck,
    title: copy("field-005", "Secure client portal"),
    body: copy("field-006", "Your notes, plans and messages stay private."),
  },
];

  const [{ testimonials, reviews, summary }, homePage, services] = await Promise.all([
    loadSocialProof(),
    getPage("home").catch(() => undefined),
    listServices().catch(() => null),
  ]);
  const hasSocialProof = testimonials.length > 0 || reviews.length > 0;
  const homeLead = homePage?.body?.split(/\n\s*\n/).find(Boolean) ?? "Registered nursing and wellness coaching, brought together in one calm, private practice. Thoughtful video sessions. A plan that fits your real life. Care that travels with you.";
  return (
    <>
      {/* Hero — design-system §2: min-height 88vh, content vertically centered.
          Type: display-xl headline, body-lg lead (brand.md §4). */}
      <Section background="night" className="terios-grain relative overflow-hidden" containerClassName="relative grid min-h-[calc(100dvh-72px)] items-center gap-14 py-16 lg:grid-cols-[1.05fr_.95fr] lg:gap-20" overlay={<HeroWatermark />}>
        <div className="max-w-[68ch]">
          <p className="text-[11px] font-semibold uppercase tracking-[0.12em] text-eucalyptus-300">
            {copy("field-007", "Nursing & wellness coaching ")}</p>
          <h1 className="mt-5 font-display text-[3.7rem] leading-[.92] font-semibold tracking-[-0.055em] text-sand-0 sm:text-[4.5rem] lg:text-[5.75rem] [text-wrap:balance]">
            {copy("field-008", "Care that lets you ")}<em className="font-medium text-clay-300">{copy("field-009", "exhale.")}</em>
          </h1>
          <p className="mt-7 max-w-[56ch] text-lg leading-[1.7] text-eucalyptus-200 [text-wrap:pretty]">
            {copy("hero-description", homeLead)}
          </p>
          <div className="mt-10 flex flex-wrap items-center gap-3">
            <Link
              href={copy("field-010", "/work-with-me")}
              className={buttonClasses({ size: "lg", className: "!bg-sand-0 !text-eucalyptus-900 shadow-none hover:!bg-eucalyptus-100" })}
            >
              {copy("field-011", "Book a session ")}</Link>
            <Link
              href={copy("field-012", "/services")}
              className={buttonClasses({ variant: "inverse", size: "lg" })}
            >
              {copy("field-013", "Explore services ")}</Link>
          </div>
          <div className="mt-12 flex flex-wrap gap-x-7 gap-y-3 border-t border-sand-0/12 pt-6 text-[13px] font-medium text-eucalyptus-300">
            <span>{copy("field-014", "Registered nurse-led")}</span><span>{copy("field-015", "Private by design")}</span><span>{copy("field-016", "Available worldwide")}</span>
          </div>
        </div>
        <div className="mx-auto w-full max-w-[480px] overflow-hidden rounded-[2rem_4rem_2rem_2rem] border border-eucalyptus-300/40 bg-sand-0 text-eucalyptus-900 lg:mr-0">
          <div className="relative aspect-[5/4]">
            <Image unoptimized
              src={copy("field-017", homePage?.coverImage || "/images/brand/theresa-yirerong-clinical.webp")}
              alt={copy("field-018", "Theresa Yirerong, registered nurse and wellness coach")}
              fill
              priority

              sizes="(min-width: 1024px) 480px, 90vw"
              className="object-cover object-top"
            />
            <span className="absolute left-5 top-5 rounded-full bg-sand-0 px-3 py-2 text-xs font-semibold text-eucalyptus-900">{copy("field-019", "Video-first care")}</span>
          </div>
          <div className="p-6 sm:p-8">
            <div className="flex items-start justify-between gap-4">
              <p className="font-display text-2xl leading-tight sm:text-3xl">{copy("field-020", "Clinical confidence.")}<br />{copy("field-021", "Human warmth.")}</p>
              <HeartPulse className="mt-1 size-6 shrink-0 text-primary" aria-hidden="true" />
            </div>
            <p className="mt-3 max-w-[30ch] text-sm leading-relaxed text-ink-muted">{copy("field-022", "One practitioner. Unhurried attention. Care shaped around you.")}</p>
          </div>
        </div>
      </Section>

      {/* Trust strip — three quiet proof points, border-defined (brand §3:
          borders, not shadows, define structure on the page background). */}
      <Section
        background="raised"
        ariaLabelledby="trust-heading"
        className="border-y border-border"
        containerClassName="py-10 lg:py-12"
      >
        <h2 id="trust-heading" className="sr-only">
          {copy("field-023", "Why clients trust Terios ")}</h2>
        <ul className="grid gap-8 md:grid-cols-3 md:gap-6">
          {trustPoints.map((point) => (
            <li key={point.title} className="flex items-start gap-4">
              <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-eucalyptus-50 text-primary">
                <point.icon aria-hidden="true" className="size-5" />
              </span>
              <span>
                <span className="block text-base font-semibold leading-[1.4] text-ink">
                  {point.title}
                </span>
                <span className="mt-1 block text-sm leading-[1.55] text-ink-muted">
                  {point.body}
                </span>
              </span>
            </li>
          ))}
        </ul>
      </Section>

      {/* Services preview — marketing feature cards (§3.21): radius-xl,
          padding space-8, whole card is one link. */}
      <Section ariaLabelledby="services-preview-heading">
        <SectionHeading
          id="services-preview-heading"
          eyebrow={copy("field-024", "Services")}
          title={copy("field-025", "Care that fits the season you are in")}
          description={copy("field-026", "Explore the practice’s current one-to-one services. Every published offering comes directly from the live care menu.")}
        />
        {services?.length ? <ul className="mt-12 grid gap-5 lg:grid-cols-12">
          {services.slice(0, 3).map((service: ServiceSummary, index) => (
            <li key={service.id} className={index === 0 ? "lg:col-span-5" : index === 1 ? "lg:col-span-7" : "lg:col-span-12"}>
              <Link
                href={`/work-with-me?service=${service.id}`}
                className={`terios-feature-card group grid h-full min-h-72 overflow-hidden rounded-[2rem] border border-border/80 bg-surface-raised/90 shadow-[0_18px_60px_rgba(31,41,34,.04)] transition-[border-color,box-shadow,transform] duration-base ease-out hover:-translate-y-1 hover:border-eucalyptus-200 hover:shadow-md ${index === 2 ? "lg:grid-cols-[.8fr_1.2fr]" : ""}`}
              >
                <span className={`relative min-h-52 overflow-hidden bg-eucalyptus-100 ${index === 2 ? "lg:min-h-64" : ""}`}>
                  <Image unoptimized src={service.imageUrl || serviceImageFor(service.name, index)} alt="" fill  sizes={index === 2 ? "(min-width: 1024px) 38vw, 94vw" : "(min-width: 1024px) 45vw, 94vw"} className="object-cover transition-transform duration-page group-hover:scale-[1.03] motion-reduce:transition-none" />
                  <span className="absolute left-5 top-5 rounded-full bg-eucalyptus-900/75 px-3 py-1.5 font-mono text-[11px] text-sand-0 backdrop-blur-md">{copy("field-027", "0")}{index + 1}</span>
                </span>
                <div className="flex max-w-[54ch] flex-col justify-end p-8">
                  <h3 className="font-display text-3xl leading-[1.08] font-medium tracking-[-0.02em] text-ink">{service.name}</h3>
                  <p className="mt-4 text-sm leading-[1.65] text-ink-muted">{service.description}</p>
                  <span className="mt-7 inline-flex items-center gap-2 text-sm font-semibold text-primary">{copy("field-028", "Book this service ")}<ArrowRight className="size-4 transition-transform group-hover:translate-x-1" /></span>
                </div>
              </Link>
            </li>
          ))}
        </ul> : <p className="mt-10 rounded-2xl border border-border bg-surface-raised p-6 text-sm text-ink-muted">{copy("field-029", "The live service menu is being prepared. ")}<Link href={copy("field-030", "/services")} className="font-semibold text-primary">{copy("field-031", "View service availability")}</Link>.</p>}
      </Section>

      {/* Approach teaser → /about. */}
      <Section background="sunken" ariaLabelledby="approach-heading">
        <div className="grid items-center gap-10 lg:grid-cols-[1fr_.9fr] lg:gap-16">
          <div>
            <SectionHeading
              id="approach-heading"
              eyebrow={copy("field-032", "The approach")}
              title={copy("field-033", "The trust of a nurse, the exhale of a spa")}
              description={copy("field-034", "Terios sits deliberately between a clinic and a retreat. You get the precision and confidentiality of registered nursing, delivered with the unhurried warmth of a wellness practice — and a plan shaped around your life, not around a system.")}
            />
            <Link href={copy("field-035", "/about")} className={buttonClasses({ variant: "secondary", size: "lg", className: "mt-8" })}>
              {copy("field-036", "About the practice ")}</Link>
          </div>
          <div className="relative aspect-[4/3] overflow-hidden rounded-[2rem_4rem_2rem_4rem] bg-eucalyptus-100 shadow-[0_24px_70px_rgba(31,41,34,.12)]">
            <Image unoptimized src={copy("field-037", "/images/brand/portraits/theresa-yirerong-by-jinnifer-douglass-010.webp")} alt={copy("field-038", "Theresa Yirerong welcoming clients to Terios Wellness")} fill sizes="(min-width: 1024px) 42vw, 94vw" className="object-cover" />
            <div className="absolute inset-x-5 bottom-5 rounded-[1.25rem] border border-sand-0/25 bg-eucalyptus-900/70 p-5 text-sand-0 backdrop-blur-md">
              <p className="font-display text-xl">{copy("field-039", "A familiar face from first conversation to follow-up.")}</p>
              <Link href={copy("field-040", "/about")} className="mt-3 inline-flex items-center gap-2 text-sm font-semibold text-eucalyptus-100">{copy("field-041", "Meet Theresa ")}<ArrowRight className="size-4" /></Link>
            </div>
          </div>
        </div>
      </Section>

      {/* Testimonials — static placeholders until the CMS ships. Clay appears
          once here (quote marks), within the ≤2-per-screen budget (brand §3). */}
      {hasSocialProof ? (
        <Section ariaLabelledby="testimonials-heading">
          <SectionHeading
            id="testimonials-heading"
            eyebrow={copy("field-042", "Kind words")}
            title={copy("field-043", "What clients say")}
            align="center"
          />
          <Testimonials
            className="mx-auto mt-12 max-w-[960px]"
            testimonials={testimonials}
            reviews={reviews}
            summary={summary}
          />
        </Section>
      ) : null}

      {/* Closing CTA band. */}
      <Section
        background="sunken"
        ariaLabelledby="closing-cta-heading"
        className="border-t border-border"
      >
        <div className="mx-auto max-w-[60ch] text-center">
          <h2
            id="closing-cta-heading"
            className="font-display text-[2rem] leading-[1.15] font-medium tracking-[-0.01em] text-ink lg:text-[2.25rem] [text-wrap:balance]"
          >
            {copy("field-044", "Begin where you are ")}</h2>
          <p className="mt-5 text-lg leading-[1.6] text-ink-muted [text-wrap:pretty]">
            {copy("field-045", "You do not need to be ready, fixed or finished. Book a first conversation and we will find the next right step together. ")}</p>
          <div className="mt-10">
            <Link
              href={copy("field-046", "/work-with-me")}
              className={buttonClasses({ size: "lg" })}
            >
              {copy("field-047", "Book a session ")}</Link>
          </div>
        </div>
      </Section>
    </>
  );
}
