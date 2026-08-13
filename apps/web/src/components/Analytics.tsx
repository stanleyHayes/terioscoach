"use client";

import { Analytics as VercelAnalytics } from "@vercel/analytics/react";
import { SpeedInsights } from "@vercel/speed-insights/next";
import { usePathname } from "next/navigation";
import { isIndexable } from "@/lib/seo";

/**
 * Analytics (WEB-10).
 *
 * Vercel Analytics and Speed Insights, chosen over Google Analytics and
 * Plausible for one reason that outranks the rest here: **they set no
 * cookies and store no personal data**, so this site needs no consent
 * banner. A wellness practice asking a visitor to accept tracking before
 * they can read about massage is a worse first impression than any amount
 * of funnel data is worth.
 *
 * They are also first-party — served through the deployment's own origin,
 * not a third-party domain — so an ad blocker does not silently zero the
 * numbers, and there is no external script host to trust.
 *
 * Two deliberate exclusions:
 *
 *  - **The portal is not measured.** Every path under /portal belongs to a
 *    signed-in client looking at their own health information. Page views
 *    there would tie a URL containing a session id to a visitor, which is
 *    not something a practice should be collecting to find out which page
 *    is popular.
 *
 *  - **Preview deployments are not measured**, for the same reason they are
 *    not indexed: staging traffic in the production numbers makes the
 *    numbers useless.
 */
export function Analytics() {
  const pathname = usePathname();

  const isPrivate =
    pathname.startsWith("/portal") ||
    pathname.startsWith("/login") ||
    pathname.startsWith("/register");

  if (isPrivate || !isIndexable()) return null;

  return (
    <>
      <VercelAnalytics />
      <SpeedInsights />
    </>
  );
}
