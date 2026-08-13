import type { Metadata } from "next";
import { Analytics } from "@/components/Analytics";
import { SITE_DESCRIPTION, SITE_NAME, SITE_URL, isIndexable } from "@/lib/seo";
import "./globals.css";

/**
 * Root metadata (WEB-10).
 *
 * `metadataBase` is what makes every relative OG image and canonical below
 * resolve to an absolute URL, and it comes from configuration so a preview
 * deployment never claims the production origin. Indexing is opt-in for the
 * same reason: only the real site invites crawlers.
 */
export const metadata: Metadata = {
  metadataBase: new URL(SITE_URL),
  title: {
    default: `${SITE_NAME} — clinical calm, wherever you are`,
    template: `%s | ${SITE_NAME}`,
  },
  description: SITE_DESCRIPTION,
  applicationName: SITE_NAME,
  icons: {
    icon: [{ url: "/icon.svg", type: "image/svg+xml" }, { url: "/favicon.ico", sizes: "any" }],
    shortcut: "/favicon.ico",
  },
  alternates: { canonical: "/" },
  openGraph: {
    type: "website",
    siteName: SITE_NAME,
    title: `${SITE_NAME} — clinical calm, wherever you are`,
    description: SITE_DESCRIPTION,
    url: "/",
    locale: "en_GB",
  },
  twitter: {
    card: "summary_large_image",
    title: SITE_NAME,
    description: SITE_DESCRIPTION,
  },
  robots: isIndexable()
    ? { index: true, follow: true }
    : // A preview build that gets indexed competes with the real site.
      { index: false, follow: false },
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className="h-full antialiased">
      <body className="flex min-h-full flex-col bg-surface font-sans text-ink">
        <a href="#main-content" className="skip-link">Skip to content</a>
        {children}
        {/* Last in the body: nothing a visitor came for waits on it. */}
        <Analytics />
      </body>
    </html>
  );
}
