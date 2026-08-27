/**
 * Site-wide SEO constants and helpers (WEB-10).
 *
 * The canonical origin is configuration, not a literal, so preview
 * deployments do not advertise themselves as the production site — a
 * preview that emits production canonicals and sitemap URLs is how a
 * staging build ends up competing with the real one in search results.
 */

export const SITE_URL = (
  process.env.NEXT_PUBLIC_SITE_URL ?? "https://terioswellness.com"
).replace(/\/$/, "");

export const SITE_NAME = "Terios Wellness Spa";

export const SITE_DESCRIPTION =
  "One-to-one nursing consultations, wellness coaching and recovery programs by video — clinical calm, wherever you are.";

/**
 * Whether crawlers should index this deployment.
 *
 * Only the production origin is indexable. Everything else — previews,
 * staging, a local build someone exposed — asks to be left alone, because
 * the cost of a preview being indexed is real and the cost of this check is
 * nothing.
 */
export function isIndexable(): boolean {
  return process.env.NEXT_PUBLIC_ALLOW_INDEXING === "true" || isProductionOrigin();
}

function isProductionOrigin(): boolean {
  return (
    process.env.VERCEL_ENV === "production" ||
    (process.env.NODE_ENV === "production" && process.env.VERCEL_ENV === undefined)
  );
}

/**
 * Paths that belong to a signed-in client or to getting signed in.
 *
 * One list, because it has two consumers that must agree — `robots.ts`
 * keeps these out of the index, and `Analytics` keeps them out of the
 * measurement — and they had already drifted once: the recovery routes
 * were added to robots when they shipped and not to analytics, which left
 * `/reset-password?token=…` being reported as a page view with a live
 * one-time credential in the URL.
 */
export const PRIVATE_PATHS = [
  "/portal",
  "/login",
  "/register",
  "/forgot-password",
  "/reset-password",
] as const;

/** Whether a pathname belongs to the private surface above. */
export function isPrivatePath(pathname: string): boolean {
  return PRIVATE_PATHS.some(
    (path) => pathname === path || pathname.startsWith(`${path}/`),
  );
}

/** Absolute URL for a site-relative path, for canonicals and OG tags. */
export function absoluteUrl(path: string): string {
  return `${SITE_URL}${path.startsWith("/") ? path : `/${path}`}`;
}
