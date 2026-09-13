import { getSiteValues, getSiteCopy } from "@/lib/site-copy";
import type { Metadata } from "next";
import { Database, Eye, Hand, Waypoints } from "lucide-react";
import { LegalPage } from "@/components/marketing/LegalPage";

export const metadata: Metadata = { title: "Privacy", description: "How Terios Wellness handles personal and care information.", alternates: { canonical: "/privacy" } };



export default async function PrivacyPage() {
  const copy = await getSiteCopy("privacy");

const sections = [
  { title: copy("field-001", "What we collect"), body: copy("field-002", "We collect the details needed to answer enquiries, manage your account, provide sessions, process payments and keep the records you ask us to hold."), icon: Database },
  { title: copy("field-003", "Why we use it"), body: copy("field-004", "Your information supports your care, booking administration, required clinical records and direct communication with you. We do not sell personal information."), icon: Waypoints },
  { title: copy("field-005", "Who can access it"), body: copy("field-006", "Access is limited to the practitioner and service providers required to operate the practice securely. Those providers process information under their own security and privacy commitments."), icon: Eye },
  { title: copy("field-007", "Your choices"), body: copy("field-008", "You may ask to access, correct or discuss deletion of your information. Some clinical and financial records must be retained where the law requires it."), icon: Hand },
] as const;

  return <LegalPage copyValues={await getSiteValues("legal")} eyebrow={copy("field-009", "Privacy")} title={copy("field-010", "Your information stays part of your care.")} description={copy("field-011", "A plain-language overview of what the practice keeps, why it is needed, and the choices you have.")} summary={copy("field-012", "We use your information to provide care, run your bookings and keep the records the practice needs. It is never treated as a product, and access stays limited.")} notice={copy("field-013", "This page is a service summary and should be reviewed against the practice’s final legal requirements before launch.")} sections={sections} relatedHref={copy("field-014", "/terms")} relatedLabel={copy("field-015", "Read our terms")} />;
}
