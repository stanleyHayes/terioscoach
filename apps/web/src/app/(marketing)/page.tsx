import Link from "next/link";
import Image from "next/image";
import { ArrowRight, Globe, HeartPulse, ShieldCheck, Video } from "lucide-react";
import { Testimonials } from "@/components/content/Testimonials";
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

const trustPoints = [
  {
    icon: Video,
    title: "Sessions by video",
    body: "Care from your own space, on your schedule.",
  },
  {
    icon: Globe,
    title: "Clients worldwide",
    body: "Time zones are a detail, not a barrier.",
  },
  {
    icon: ShieldCheck,
    title: "Secure client portal",
    body: "Your notes, plans and messages stay private.",
  },
];

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
      <Section background="night" className="terios-grain overflow-hidden" containerClassName="grid min-h-[calc(100dvh-72px)] items-center gap-14 py-16 lg:grid-cols-[1.05fr_.95fr] lg:gap-20">
        <div className="max-w-[68ch]">
          <p className="text-[11px] font-semibold uppercase tracking-[0.12em] text-eucalyptus-300">
            Nursing &amp; wellness coaching
          </p>
          <h1 className="mt-5 font-display text-[3.7rem] leading-[.92] font-semibold tracking-[-0.055em] text-sand-0 sm:text-[4.5rem] lg:text-[5.75rem] [text-wrap:balance]">
            Care that lets you <em className="font-medium text-clay-300">exhale.</em>
          </h1>
          <p className="mt-7 max-w-[56ch] text-lg leading-[1.7] text-eucalyptus-200 [text-wrap:pretty]">
            {homeLead}
          </p>
          <div className="mt-10 flex flex-wrap items-center gap-3">
            <Link
              href="/work-with-me"
              className={buttonClasses({ size: "lg", className: "!bg-sand-0 !text-eucalyptus-900 shadow-none hover:!bg-eucalyptus-100" })}
            >
              Book a session
            </Link>
            <Link
              href="/services"
              className={buttonClasses({ variant: "secondary", size: "lg", className: "border-sand-0/25 text-sand-0 hover:border-sand-0/40 hover:bg-sand-0/8" })}
            >
              Explore services
            </Link>
          </div>
          <div className="mt-12 flex flex-wrap gap-x-7 gap-y-3 border-t border-sand-0/12 pt-6 text-[13px] font-medium text-eucalyptus-300">
            <span>Registered nurse-led</span><span>Private by design</span><span>Available worldwide</span>
          </div>
        </div>
        <div className="relative mx-auto aspect-[4/5] w-full max-w-[480px] lg:mr-0">
          <div className="absolute inset-5 rotate-3 rounded-[3rem_1.5rem_3rem_1.5rem] bg-eucalyptus-100" />
          <div className="absolute inset-0 -rotate-2 overflow-hidden rounded-[2rem_4.5rem_2rem_4.5rem] border border-eucalyptus-200 bg-eucalyptus-900 shadow-[0_35px_90px_rgba(28,51,40,.22)]">
            <Image
              src={homePage?.coverImage || "/images/brand/theresa-yirerong-clinical.webp"}
              alt="Theresa Yirerong, registered nurse and wellness coach"
              fill
              priority
              unoptimized={Boolean(homePage?.coverImage?.startsWith("http"))}
              sizes="(min-width: 1024px) 480px, 90vw"
              className="object-cover"
            />
            <div className="absolute inset-0 bg-gradient-to-t from-eucalyptus-950/75 via-transparent to-transparent" />
            <div className="absolute inset-x-8 bottom-8 rounded-[1.5rem] border border-sand-0/15 bg-sand-0/10 p-6 text-sand-0 backdrop-blur-md">
              <HeartPulse className="size-6 text-eucalyptus-300" aria-hidden="true" />
              <p className="mt-8 font-display text-3xl leading-tight">Clinical confidence.<br />Human warmth.</p>
              <p className="mt-3 text-sm leading-relaxed text-sand-0/70">One practitioner. Unhurried attention. Care shaped around you.</p>
            </div>
          </div>
          <div className="absolute -left-5 top-14 rounded-full border border-border bg-surface-raised px-4 py-2 text-xs font-semibold text-primary shadow-md">Video-first care</div>
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
          Why clients trust Terios
        </h2>
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
          eyebrow="Services"
          title="Care that fits the season you are in"
          description="Explore the practice’s current one-to-one services. Every published offering comes directly from the live care menu."
        />
        {services?.length ? <ul className="mt-12 grid gap-5 lg:grid-cols-12">
          {services.slice(0, 3).map((service: ServiceSummary, index) => (
            <li key={service.id} className={index === 0 ? "lg:col-span-5" : index === 1 ? "lg:col-span-7" : "lg:col-span-12"}>
              <Link
                href={`/work-with-me?service=${service.id}`}
                className={`terios-feature-card group grid h-full min-h-72 overflow-hidden rounded-[2rem] border border-border/80 bg-surface-raised/90 shadow-[0_18px_60px_rgba(31,41,34,.04)] transition-[border-color,box-shadow,transform] duration-base ease-out hover:-translate-y-1 hover:border-eucalyptus-200 hover:shadow-md ${index === 2 ? "lg:grid-cols-[.8fr_1.2fr]" : ""}`}
              >
                <span className={`relative min-h-52 overflow-hidden bg-eucalyptus-100 ${index === 2 ? "lg:min-h-64" : ""}`}>
                  <Image src={service.imageUrl || serviceImageFor(service.name, index)} alt="" fill unoptimized={Boolean(service.imageUrl?.startsWith("http"))} sizes={index === 2 ? "(min-width: 1024px) 38vw, 94vw" : "(min-width: 1024px) 45vw, 94vw"} className="object-cover transition-transform duration-page group-hover:scale-[1.03] motion-reduce:transition-none" />
                  <span className="absolute left-5 top-5 rounded-full bg-eucalyptus-950/75 px-3 py-1.5 font-mono text-[11px] text-sand-0 backdrop-blur-md">0{index + 1}</span>
                </span>
                <div className="flex max-w-[54ch] flex-col justify-end p-8">
                  <h3 className="font-display text-3xl leading-[1.08] font-medium tracking-[-0.02em] text-ink">{service.name}</h3>
                  <p className="mt-4 text-sm leading-[1.65] text-ink-muted">{service.description}</p>
                  <span className="mt-7 inline-flex items-center gap-2 text-sm font-semibold text-primary">Book this service <ArrowRight className="size-4 transition-transform group-hover:translate-x-1" /></span>
                </div>
              </Link>
            </li>
          ))}
        </ul> : <p className="mt-10 rounded-2xl border border-border bg-surface-raised p-6 text-sm text-ink-muted">The live service menu is being prepared. <Link href="/services" className="font-semibold text-primary">View service availability</Link>.</p>}
      </Section>

      {/* Approach teaser → /about. */}
      <Section background="sunken" ariaLabelledby="approach-heading">
        <div className="grid items-center gap-10 lg:grid-cols-[1fr_.9fr] lg:gap-16">
          <div>
            <SectionHeading
              id="approach-heading"
              eyebrow="The approach"
              title="The trust of a nurse, the exhale of a spa"
              description="Terios sits deliberately between a clinic and a retreat. You get the precision and confidentiality of registered nursing, delivered with the unhurried warmth of a wellness practice — and a plan shaped around your life, not around a system."
            />
            <Link href="/about" className={buttonClasses({ variant: "secondary", size: "lg", className: "mt-8" })}>
              About the practice
            </Link>
          </div>
          <div className="relative aspect-[4/3] overflow-hidden rounded-[2rem_4rem_2rem_4rem] bg-eucalyptus-100 shadow-[0_24px_70px_rgba(31,41,34,.12)]">
            <Image src="/images/brand/portraits/theresa-yirerong-by-jinnifer-douglass-010.webp" alt="Theresa Yirerong welcoming clients to Terios Wellness" fill sizes="(min-width: 1024px) 42vw, 94vw" className="object-cover" />
            <div className="absolute inset-x-5 bottom-5 rounded-[1.25rem] border border-sand-0/25 bg-eucalyptus-950/70 p-5 text-sand-0 backdrop-blur-md">
              <p className="font-display text-xl">A familiar face from first conversation to follow-up.</p>
              <Link href="/about" className="mt-3 inline-flex items-center gap-2 text-sm font-semibold text-eucalyptus-100">Meet Theresa <ArrowRight className="size-4" /></Link>
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
            eyebrow="Kind words"
            title="What clients say"
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
            Begin where you are
          </h2>
          <p className="mt-5 text-lg leading-[1.6] text-ink-muted [text-wrap:pretty]">
            You do not need to be ready, fixed or finished. Book a first
            conversation and we will find the next right step together.
          </p>
          <div className="mt-10">
            <Link
              href="/work-with-me"
              className={buttonClasses({ size: "lg" })}
            >
              Book a session
            </Link>
          </div>
        </div>
      </Section>
    </>
  );
}
