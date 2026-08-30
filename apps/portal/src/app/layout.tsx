import type { Metadata } from "next";
import type { ReactNode } from "react";
import "./globals.css";

export const metadata: Metadata = {
  metadataBase: new URL(process.env.NEXT_PUBLIC_PORTAL_URL ?? "https://app.terioscoach.com"),
  title: {
    default: "Client portal | Terios Wellness Spa",
    template: "%s | Terios Wellness Spa",
  },
  description: "Your private Terios Wellness Spa care space.",
  robots: { index: false, follow: false, noarchive: true },
  icons: {
    icon: [{ url: "/icon.svg", type: "image/svg+xml" }, { url: "/favicon.ico", sizes: "any" }],
  },
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en" className="h-full antialiased">
      <body className="flex min-h-full flex-col bg-surface font-sans text-ink">
        <a href="#main-content" className="skip-link">Skip to content</a>
        {children}
      </body>
    </html>
  );
}
