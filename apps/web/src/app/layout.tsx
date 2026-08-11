import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: {
    default: "Terios Wellness Spa",
    template: "%s | Terios Wellness Spa",
  },
  description:
    "One-on-one wellness care from a registered nurse — video sessions, calm guidance, wherever you are.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className="h-full antialiased">
      <body className="flex min-h-full flex-col bg-surface font-sans text-ink">
        {children}
      </body>
    </html>
  );
}
