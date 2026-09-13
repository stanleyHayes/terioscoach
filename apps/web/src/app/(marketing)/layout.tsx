import { getSiteValues } from "@/lib/site-copy";
import { SiteCopyProvider } from "@/lib/site-copy-context";
import type { ReactNode } from "react";
import { SiteFooter } from "@/components/layout/SiteFooter";
import { SiteNav } from "@/components/layout/SiteNav";

/** Marketing chrome: public pages get the site nav + footer. The (portal)
 * route group opts out — auth screens stand alone and the portal renders
 * its own authed top nav (design-system §3.30). */
export default async function MarketingLayout({ children }: { children: ReactNode }) {
  const scopes = ["header", "footer", "intro", "contact-form", "faq-search"];
  const values = Object.fromEntries(await Promise.all(scopes.map(async scope => [scope, await getSiteValues(scope)])));
  return (
    <SiteCopyProvider values={values}>
      <SiteNav />
      <main id="main-content" className="flex-1">{children}</main>
      <SiteFooter />
    </SiteCopyProvider>
  );
}
