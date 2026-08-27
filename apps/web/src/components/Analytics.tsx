"use client";

import { Analytics as VercelAnalytics } from "@vercel/analytics/react";
import { SpeedInsights } from "@vercel/speed-insights/next";
import { usePathname } from "next/navigation";
import { isIndexable, isPrivatePath } from "@/lib/seo";

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
 *  - **The private surface is not measured** — the portal and every route
 *    that leads into it. A portal path belongs to a signed-in client
 *    looking at their own health information, and measuring it would tie a
 *    URL containing a session id to a visitor. The auth and recovery
 *    routes are worse still: `/reset-password` carries a live one-time
 *    token in its query string. The list lives in `lib/seo` because
 *    `robots.ts` needs the same one.
 *
 *  - **Preview deployments are not measured**, for the same reason they are
 *    not indexed: staging traffic in the production numbers makes the
 *    numbers useless.
 */
export function Analytics() {
  const pathname = usePathname();

  if (isPrivatePath(pathname) || !isIndexable()) return null;

  return (
    <>
      <VercelAnalytics beforeSend={withoutQueryString} />
      <SpeedInsights />
    </>
  );
}

/**
 * Strips the query string from every reported URL.
 *
 * `isPrivatePath` already keeps the recovery routes out of the
 * measurement, so this is the second layer rather than the first — but it
 * is the layer that does not depend on anyone remembering to extend a list
 * when they add a route. Nothing on this site needs query parameters
 * measured, and the one that would hurt most to record is a password-reset
 * token arriving as `?token=…`.
 */
function withoutQueryString<Event extends { url: string }>(event: Event): Event {
  // Relative or malformed URLs are not something this library sends, but a
  // throw here would take the page down over a page view.
  const stripped = URL.parse(event.url);
  if (stripped === null) return event;
  stripped.search = "";
  return { ...event, url: stripped.toString() };
}
