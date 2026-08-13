import type { Metadata } from "next";
import type { ReactNode } from "react";
import { AuthProvider } from "@/lib/auth";

/** Everything under (portal) — auth screens and the portal itself — shares
 * the client auth session provider. */
export default function PortalGroupLayout({ children }: { children: ReactNode }) {
  return <AuthProvider>{children}</AuthProvider>;
}
export const metadata: Metadata = {
  robots: { index: false, follow: false, noarchive: true },
};
