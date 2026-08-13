import type { Metadata } from "next";
import Link from "next/link";
import { Clock, Mail, MessageCircle } from "lucide-react";
import { EnquiryForm } from "@/components/content/EnquiryForm";
import { Section } from "@/components/marketing/Section";
import { PageIntro } from "@/components/marketing/PageIntro";

export const metadata: Metadata = {
  title: "Contact",
  description:
    "Ask about a session, a service or anything else — messages reach the practitioner directly and are answered within a working day.",
  alternates: { canonical: "/contact" },
  openGraph: {
    type: "website",
    url: "/contact",
    title: "Contact",
  },
};

export default function ContactPage() {
  return (
    <>
      <PageIntro eyebrow="Contact" title="Start with a question" description="You do not have to know what you need yet. Say what is going on and the right next step comes back — even if that step is somewhere else." />

      <Section ariaLabelledby="contact-form-heading">
        <div className="grid gap-12 lg:grid-cols-[minmax(0,1fr)_320px] lg:gap-16">
          <div>
            <h2 id="contact-form-heading" className="sr-only">
              Send a message
            </h2>
            <EnquiryForm />
          </div>

          <aside className="flex flex-col gap-8 lg:pt-2">
            <div className="flex gap-4">
              <span className="flex size-10 shrink-0 items-center justify-center rounded-full bg-surface-sunken">
                <MessageCircle size={18} aria-hidden="true" className="text-ink-muted" />
              </span>
              <div>
                <h3 className="text-base font-semibold leading-[1.4] text-ink">
                  A person reads it
                </h3>
                <p className="mt-1 text-sm leading-[1.55] text-ink-muted">
                  Messages go to the practitioner, not an inbox nobody opens.
                </p>
              </div>
            </div>

            <div className="flex gap-4">
              <span className="flex size-10 shrink-0 items-center justify-center rounded-full bg-surface-sunken">
                <Clock size={18} aria-hidden="true" className="text-ink-muted" />
              </span>
              <div>
                <h3 className="text-base font-semibold leading-[1.4] text-ink">
                  Usually within a working day
                </h3>
                <p className="mt-1 text-sm leading-[1.55] text-ink-muted">
                  Sooner where it matters. Weekends run slower.
                </p>
              </div>
            </div>

            <div className="flex gap-4">
              <span className="flex size-10 shrink-0 items-center justify-center rounded-full bg-surface-sunken">
                <Mail size={18} aria-hidden="true" className="text-ink-muted" />
              </span>
              <div>
                <h3 className="text-base font-semibold leading-[1.4] text-ink">
                  Already a client?
                </h3>
                <p className="mt-1 text-sm leading-[1.55] text-ink-muted">
                  Your portal is the faster route for anything about a booked
                  session.
                </p>
                <Link
                  href="/portal"
                  className="mt-2 inline-flex text-sm font-medium text-primary transition-colors duration-instant ease-out hover:text-primary-hover"
                >
                  Open your portal
                </Link>
              </div>
            </div>
          </aside>
        </div>
      </Section>
    </>
  );
}
