import type { Metadata } from "next";
import { CalendarClock, HeartHandshake, Laptop, UserRoundCheck } from "lucide-react";
import { LegalPage } from "@/components/marketing/LegalPage";

export const metadata: Metadata = { title: "Terms", description: "Terms for using Terios Wellness services and digital practice platform.", alternates: { canonical: "/terms" } };

const sections = [
  { title: "Using the practice", body: "Book and use services only for yourself unless the practitioner has agreed otherwise. Keep your account details private and provide accurate information relevant to your care.", icon: UserRoundCheck },
  { title: "Appointments", body: "Your booking confirmation states the time, timezone, price and applicable change rules. Rescheduling, cancellation and refund outcomes follow the terms shown during booking.", icon: CalendarClock },
  { title: "Clinical boundaries", body: "Online wellness and nursing support does not replace emergency care. If you may be experiencing an emergency, contact local emergency services immediately.", icon: HeartHandshake },
  { title: "Digital access", body: "We work to keep the portal available and secure, but maintenance and network conditions can interrupt access. Contact the practice when an interruption affects an upcoming session.", icon: Laptop },
] as const;

export default function TermsPage() {
  return <LegalPage eyebrow="Terms" title="Clear expectations make care easier." description="The practical agreement behind appointments, the client portal, payments and online sessions." summary="Use the practice honestly, keep your account secure, and rely on the details shown when you book. We will communicate clearly when access, timing or care boundaries matter." notice="Final launch terms should be reviewed by a qualified adviser in the practice’s operating jurisdictions." sections={sections} relatedHref="/privacy" relatedLabel="Read our privacy policy" />;
}
