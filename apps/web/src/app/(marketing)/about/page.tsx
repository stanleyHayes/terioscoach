import type { Metadata } from "next";
import Link from "next/link";
import { CalendarCheck, MessageSquareHeart, Video } from "lucide-react";
import { Section } from "@/components/marketing/Section";
import { SectionHeading } from "@/components/marketing/SectionHeading";
import { buttonClasses } from "@/components/ui/Button";

export const metadata: Metadata = {
  title: "About",
  description:
    "The practice and approach behind Terios Wellness — registered nursing and wellness coaching, one-to-one and by video.",
};

// TODO(cms): replace with the practitioner bio from the CMS API (long-form
// rich text + portrait). Copy below is placeholder grounded in the brand voice.
const storyParagraphs = [
  "Terios began with a simple observation: people do not need more information about their health — they need someone qualified, unhurried and on their side.",
  "After years of nursing, I wanted to practice care the way it should feel: precise and confidential like a clinic, calm and personal like a retreat. So I built a practice that is both. Sessions happen by video, which means your care fits around your life — whether you are down the road or on another continent.",
  "As the practice grows, so will the space around it: Terios is evolving toward a full wellness spa brand, with the same nursing backbone underneath everything it offers.",
];

const principles = [
  {
    title: "Listen first",
    body: "Every engagement starts with your story, not a protocol. Good care is built on being properly heard.",
  },
  {
    title: "Clinical grounding",
    body: "A registered nurse stands behind every recommendation. Calm never means casual about your health.",
  },
  {
    title: "Small, sustainable steps",
    body: "Plans you can actually keep. Progress measured in weeks and seasons, not in dramatic overhauls.",
  },
  {
    title: "Your pace, your place",
    body: "Sessions by video, scheduled across time zones. Care that adapts to your life instead of interrupting it.",
  },
];

// TODO(cms): replace with verified credentials from the CMS API.
const credentials = [
  { value: "RN", label: "Registered nurse" },
  { value: "Certified", label: "Wellness coach" },
  { value: "Worldwide", label: "Clients across time zones" },
];

const steps = [
  {
    icon: CalendarCheck,
    title: "Book a time",
    body: "Choose a session and a time that suits you. Times are always shown with a timezone, so nothing gets lost in translation.",
  },
  {
    icon: Video,
    title: "Meet by video",
    body: "Join from wherever you are. Sessions are one-to-one, unhurried and entirely focused on you.",
  },
  {
    icon: MessageSquareHeart,
    title: "Follow up in your portal",
    body: "Notes, plans and next steps wait in your secure client portal, with space to ask questions between sessions.",
  },
];

export default function About() {
  return (
    <>
      {/* Page header. */}
      <Section containerClassName="pb-0 lg:pb-0">
        <div className="max-w-[68ch]">
          <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-muted">
            About
          </p>
          <h1 className="mt-4 font-display text-[2.5rem] leading-[1.1] font-medium tracking-[-0.015em] text-ink lg:text-[3rem] [text-wrap:balance]">
            A practice built on calm, clinical care
          </h1>
          <p className="mt-6 max-w-[60ch] text-lg leading-[1.6] text-ink-muted [text-wrap:pretty]">
            Terios Wellness is the one-woman practice of a registered nurse and
            wellness coach — care that is credentialed, confidential and
            genuinely unhurried.
          </p>
        </div>
      </Section>

      {/* Practitioner story — portrait placeholder + bio placeholder. */}
      <Section ariaLabelledby="story-heading">
        <div className="grid gap-10 lg:grid-cols-[2fr_3fr] lg:gap-16">
          {/* TODO(cms): swap for the practitioner portrait (4:5, radius-xl,
              warm natural light per brand.md §7) once media is in the CMS. */}
          <div
            aria-hidden="true"
            className="aspect-[4/5] w-full max-w-[400px] rounded-xl border border-border bg-surface-sunken"
          />
          <div>
            <h2
              id="story-heading"
              className="font-display text-[2rem] leading-[1.15] font-medium tracking-[-0.01em] text-ink lg:text-[2.25rem] [text-wrap:pretty]"
            >
              The practitioner
            </h2>
            <div className="mt-6 flex max-w-[68ch] flex-col gap-5">
              {storyParagraphs.map((paragraph) => (
                <p
                  key={paragraph.slice(0, 32)}
                  className="text-base leading-[1.6] text-ink-muted [text-wrap:pretty]"
                >
                  {paragraph}
                </p>
              ))}
            </div>
          </div>
        </div>
      </Section>

      {/* Philosophy — numbered principles. */}
      <Section background="sunken" ariaLabelledby="philosophy-heading">
        <SectionHeading
          id="philosophy-heading"
          eyebrow="Philosophy"
          title="How I approach care"
          description="Four principles shape every session, every plan and every follow-up."
        />
        <ol className="mt-12 grid gap-x-6 gap-y-10 md:grid-cols-2 lg:grid-cols-4">
          {principles.map((principle, index) => (
            <li key={principle.title}>
              <span
                aria-hidden="true"
                className="font-display text-[2rem] leading-none font-medium text-primary"
              >
                {String(index + 1).padStart(2, "0")}
              </span>
              <h3 className="mt-4 text-base font-semibold leading-[1.4] text-ink">
                {principle.title}
              </h3>
              <p className="mt-2 text-sm leading-[1.55] text-ink-muted">
                {principle.body}
              </p>
            </li>
          ))}
        </ol>
      </Section>

      {/* Credentials strip — placeholders. */}
      <Section
        background="raised"
        ariaLabelledby="credentials-heading"
        className="border-y border-border"
        containerClassName="py-10 lg:py-12"
      >
        <h2 id="credentials-heading" className="sr-only">
          Credentials
        </h2>
        <ul className="grid gap-8 text-center sm:grid-cols-3">
          {credentials.map((credential) => (
            <li key={credential.label}>
              <span className="font-display text-[2rem] leading-[1.15] font-medium tracking-[-0.01em] text-ink">
                {credential.value}
              </span>
              <span className="mt-1 block text-[13px] font-medium tracking-[0.01em] text-ink-muted">
                {credential.label}
              </span>
            </li>
          ))}
        </ul>
      </Section>

      {/* How sessions work — three steps. */}
      <Section ariaLabelledby="how-it-works-heading">
        <SectionHeading
          id="how-it-works-heading"
          eyebrow="How sessions work"
          title="From booking to follow-up"
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
      </Section>

      {/* Closing CTA. */}
      <Section
        background="sunken"
        ariaLabelledby="about-cta-heading"
        className="border-t border-border"
      >
        <div className="mx-auto max-w-[60ch] text-center">
          <h2
            id="about-cta-heading"
            className="font-display text-[2rem] leading-[1.15] font-medium tracking-[-0.01em] text-ink lg:text-[2.25rem] [text-wrap:balance]"
          >
            Care, one conversation at a time
          </h2>
          <p className="mt-5 text-lg leading-[1.6] text-ink-muted [text-wrap:pretty]">
            If this approach sounds like what you have been looking for, the
            next step is simply a conversation.
          </p>
          <div className="mt-10">
            <Link
              href="/work-with-me"
              className={buttonClasses({ size: "lg" })}
            >
              Work with me
            </Link>
          </div>
        </div>
      </Section>
    </>
  );
}
