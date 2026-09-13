import { getSiteValues, getSiteCopy } from "@/lib/site-copy";
import type { Metadata } from "next";
import { CalendarClock, HeartHandshake, Laptop, UserRoundCheck } from "lucide-react";
import { LegalPage } from "@/components/marketing/LegalPage";

export const metadata: Metadata = { title: "Terms", description: "Terms for using Terios Wellness services and digital practice platform.", alternates: { canonical: "/terms" } };



export default async function TermsPage() {
  const copy = await getSiteCopy("terms");

const sections = [
  { title: copy("field-001", "Using the practice"), body: copy("field-002", "Book and use services only for yourself unless the practitioner has agreed otherwise. Keep your account details private and provide accurate information relevant to your care."), icon: UserRoundCheck },
  { title: copy("field-003", "Appointments"), body: copy("field-004", "Your booking confirmation states the time, timezone, price and applicable change rules. Rescheduling, cancellation and refund outcomes follow the terms shown during booking."), icon: CalendarClock },
  { title: copy("field-005", "Clinical boundaries"), body: copy("field-006", "Online wellness and nursing support does not replace emergency care. If you may be experiencing an emergency, contact local emergency services immediately."), icon: HeartHandshake },
  { title: copy("field-007", "Digital access"), body: copy("field-008", "We work to keep the portal available and secure, but maintenance and network conditions can interrupt access. Contact the practice when an interruption affects an upcoming session."), icon: Laptop },
] as const;

  return <LegalPage copyValues={await getSiteValues("legal")} eyebrow={copy("field-009", "Terms")} title={copy("field-010", "Clear expectations make care easier.")} description={copy("field-011", "The practical agreement behind appointments, the client portal, payments and online sessions.")} summary={copy("field-012", "Use the practice honestly, keep your account secure, and rely on the details shown when you book. We will communicate clearly when access, timing or care boundaries matter.")} notice={copy("field-013", "Final launch terms should be reviewed by a qualified adviser in the practice’s operating jurisdictions.")} sections={sections} relatedHref={copy("field-014", "/privacy")} relatedLabel={copy("field-015", "Read our privacy policy")} />;
}
