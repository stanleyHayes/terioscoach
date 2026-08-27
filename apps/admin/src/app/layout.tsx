import type { Metadata } from "next";
import { AuthProvider } from "@/lib/auth";
import "./globals.css";

export const metadata: Metadata = {
  title: "Terios Practice",
  description: "The Terios Wellness practice dashboard.",
  robots: { index: false, follow: false, noarchive: true, nosnippet: true },
  icons: {
    icon: [{ url: "/icon.svg", type: "image/svg+xml" }, { url: "/favicon.ico", sizes: "any" }],
    shortcut: "/favicon.ico",
  },
};

/**
 * Every dashboard route renders per request.
 *
 * This is what the nonce-based CSP in `src/proxy.ts` requires. Next stamps
 * the nonce onto its framework and bundle tags while rendering, by reading
 * it out of the request's Content-Security-Policy header — so a page
 * prerendered at build time, when there is no request and no header, ships
 * script tags with no nonce at all. Under `'strict-dynamic'` browsers
 * ignore the `'self'` fallback, so those tags are refused and the app loads
 * as a blank page with a console full of CSP violations.
 *
 * Nothing is given up for it: the dashboard sends `Cache-Control: private,
 * no-store` on every response because it carries one practitioner's client
 * records, so these pages were never cached anywhere to begin with.
 */
export const dynamic = "force-dynamic";

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    // The admin app mounts on the dark theme (deep botanical night, brand.md §3.5).
    <html lang="en" data-theme="dark" className="h-full antialiased">
      <body className="min-h-full bg-surface font-sans text-ink">
        <a href="#admin-content" className="admin-skip-link">Skip to dashboard</a>
        <AuthProvider>{children}</AuthProvider>
      </body>
    </html>
  );
}
