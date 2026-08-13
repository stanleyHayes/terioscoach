import type { Metadata } from "next";
import { Database, Eye, Hand, Waypoints } from "lucide-react";
import { LegalPage } from "@/components/marketing/LegalPage";

export const metadata: Metadata = { title: "Privacy", description: "How Terios Wellness handles personal and care information.", alternates: { canonical: "/privacy" } };

const sections = [
  { title: "What we collect", body: "We collect the details needed to answer enquiries, manage your account, provide sessions, process payments and keep the records you ask us to hold.", icon: Database },
  { title: "Why we use it", body: "Your information supports your care, booking administration, required clinical records and direct communication with you. We do not sell personal information.", icon: Waypoints },
  { title: "Who can access it", body: "Access is limited to the practitioner and service providers required to operate the practice securely. Those providers process information under their own security and privacy commitments.", icon: Eye },
  { title: "Your choices", body: "You may ask to access, correct or discuss deletion of your information. Some clinical and financial records must be retained where the law requires it.", icon: Hand },
] as const;

export default function PrivacyPage() {
  return <LegalPage eyebrow="Privacy" title="Your information stays part of your care." description="A plain-language overview of what the practice keeps, why it is needed, and the choices you have." summary="We use your information to provide care, run your bookings and keep the records the practice needs. It is never treated as a product, and access stays limited." notice="This page is a service summary and should be reviewed against the practice’s final legal requirements before launch." sections={sections} relatedHref="/terms" relatedLabel="Read our terms" />;
}
