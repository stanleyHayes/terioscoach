import type { MetadataRoute } from "next";
import { SITE_URL, isIndexable } from "@/lib/seo";

/**
 * robots.txt (WEB-10).
 *
 * Two rules matter here. The client portal is disallowed outright: it is
 * behind authentication, so a crawler can only ever get a login page from
 * it, and having those URLs in an index invites confusion. And a deployment
 * that is not the production origin refuses everything — a preview build
 * competing with the real site in search results is a genuine cost, and the
 * check to prevent it is free.
 */
export default function robots(): MetadataRoute.Robots {
  if (!isIndexable()) {
    return {
      rules: { userAgent: "*", disallow: "/" },
    };
  }

  return {
    rules: {
      userAgent: "*",
      allow: "/",
      disallow: ["/portal/", "/login", "/register", "/forgot-password", "/reset-password"],
    },
    sitemap: `${SITE_URL}/sitemap.xml`,
    host: SITE_URL,
  };
}
