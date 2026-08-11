import Link from "next/link";
import { Globe, ShieldCheck, Video } from "lucide-react";
import { Section } from "@/components/marketing/Section";
import { SectionHeading } from "@/components/marketing/SectionHeading";
import { buttonClasses } from "@/components/ui/Button";

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

// TODO(cms): replace with services from the CMS API (WEB-04 services page
// will own the canonical fetch; this preview should consume the same data).
const servicesPreview = [
  {
    title: "Wellness coaching",
    body: "Ongoing one-on-one coaching to build sustainable rhythms of rest, movement and nourishment — at your pace.",
  },
  {
    title: "Nursing consultations",
    body: "Clinical guidance from a registered nurse, by video. Clear answers, careful follow-up, no waiting rooms.",
  },
  {
    title: "Rest & recovery programs",
    body: "Structured multi-week programs that pair nursing oversight with coaching, for seasons that ask more of you.",
  },
];

// TODO(cms): replace with testimonials from the CMS API once published.
const testimonials = [
  {
    quote:
      "I came for the coaching and stayed for the calm. Every session ends with a plan I can actually keep.",
    name: "A long-term client",
    detail: "Wellness coaching, 8 months",
  },
  {
    quote:
      "Speaking with a nurse who also understands rest changed how I recover. I feel looked after, not processed.",
    name: "A client abroad",
    detail: "Nursing consultations, by video",
  },
];

export default function Home() {
  return (
    <>
      {/* Hero — design-system §2: min-height 88vh, content vertically centered.
          Type: display-xl headline, body-lg lead (brand.md §4). */}
      <Section containerClassName="flex min-h-[88vh] flex-col justify-center">
        <div className="max-w-[68ch]">
          <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-muted">
            Nursing &amp; wellness coaching
          </p>
          <h1 className="mt-4 font-display text-[3.5rem] leading-[1.05] font-medium tracking-[-0.02em] text-ink lg:text-[4rem] [text-wrap:balance]">
            Clinical calm, wherever you are
          </h1>
          <p className="mt-6 max-w-[60ch] text-lg leading-[1.6] text-ink-muted [text-wrap:pretty]">
            Terios is a one-woman practice bringing registered nursing and
            wellness coaching together — unhurried video sessions and steady
            guidance, wherever in the world you happen to be.
          </p>
          <div className="mt-10 flex flex-wrap items-center gap-3">
            <Link
              href="/work-with-me"
              className={buttonClasses({ size: "lg" })}
            >
              Book a session
            </Link>
            <Link
              href="/services"
              className={buttonClasses({ variant: "secondary", size: "lg" })}
            >
              Explore services
            </Link>
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
          description="Three ways to work together, each one-to-one and by video. Every engagement begins with a conversation, not a form."
        />
        <ul className="mt-12 grid gap-6 md:grid-cols-3">
          {servicesPreview.map((service) => (
            <li key={service.title}>
              <Link
                href="/services"
                className="block h-full rounded-xl border border-border bg-surface-raised p-8 transition-[border-color,box-shadow,transform] duration-base ease-out hover:-translate-y-0.5 hover:border-border-strong hover:shadow-sm"
              >
                <h3 className="font-display text-2xl leading-[1.2] font-medium tracking-[-0.01em] text-ink [text-wrap:pretty]">
                  {service.title}
                </h3>
                <p className="mt-3 text-sm leading-[1.55] text-ink-muted">
                  {service.body}
                </p>
                <span className="mt-6 inline-block text-sm font-medium tracking-[0.005em] text-primary">
                  Learn more
                </span>
              </Link>
            </li>
          ))}
        </ul>
      </Section>

      {/* Approach teaser → /about. */}
      <Section background="sunken" ariaLabelledby="approach-heading">
        <div className="grid items-center gap-10 lg:grid-cols-2 lg:gap-16">
          <SectionHeading
            id="approach-heading"
            eyebrow="The approach"
            title="The trust of a nurse, the exhale of a spa"
            description="Terios sits deliberately between a clinic and a retreat. You get the precision and confidentiality of registered nursing, delivered with the unhurried warmth of a wellness practice — and a plan shaped around your life, not around a system."
          />
          <div className="lg:justify-self-end">
            <Link
              href="/about"
              className={buttonClasses({ variant: "secondary", size: "lg" })}
            >
              About the practice
            </Link>
          </div>
        </div>
      </Section>

      {/* Testimonials — static placeholders until the CMS ships. Clay appears
          once here (quote marks), within the ≤2-per-screen budget (brand §3). */}
      <Section ariaLabelledby="testimonials-heading">
        <SectionHeading
          id="testimonials-heading"
          eyebrow="Kind words"
          title="What clients say"
          align="center"
        />
        <ul className="mx-auto mt-12 grid max-w-[960px] gap-6 md:grid-cols-2">
          {testimonials.map((testimonial) => (
            <li key={testimonial.name}>
              <figure className="flex h-full flex-col rounded-xl border border-border bg-surface-raised p-8">
                <span
                  aria-hidden="true"
                  className="font-display text-[2.5rem] leading-none font-medium text-accent"
                >
                  &ldquo;
                </span>
                <blockquote className="mt-2 flex-1">
                  <p className="font-display text-xl leading-[1.4] font-medium tracking-[-0.005em] text-ink [text-wrap:pretty]">
                    {testimonial.quote}
                  </p>
                </blockquote>
                <figcaption className="mt-6">
                  <span className="block text-sm font-semibold text-ink">
                    {testimonial.name}
                  </span>
                  <span className="mt-0.5 block text-[13px] font-medium tracking-[0.01em] text-ink-faint">
                    {testimonial.detail}
                  </span>
                </figcaption>
              </figure>
            </li>
          ))}
        </ul>
      </Section>

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
