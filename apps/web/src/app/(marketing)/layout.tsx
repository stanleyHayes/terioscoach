import type { ReactNode } from "react";
import { SiteFooter } from "@/components/layout/SiteFooter";
import { SiteNav } from "@/components/layout/SiteNav";

/** Marketing chrome: public pages get the site nav + footer. The (portal)
 * route group opts out — auth screens stand alone and the portal renders
 * its own authed top nav (design-system §3.30). */
export default function MarketingLayout({ children }: { children: ReactNode }) {
  return (
    <>
      <SiteNav />
      <main className="flex-1">{children}</main>
      <SiteFooter />
    </>
  );
}
