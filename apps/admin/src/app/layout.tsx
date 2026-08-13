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
