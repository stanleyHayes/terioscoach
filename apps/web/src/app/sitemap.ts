import type { MetadataRoute } from "next";
import { listPosts } from "@/lib/content";
import { SITE_URL } from "@/lib/seo";

/**
 * Sitemap (WEB-10).
 *
 * The static routes are listed by hand — there are few of them and they
 * change rarely. Blog articles come from the CMS, so the sitemap is
 * generated at request time: a post published in the dashboard should be
 * crawlable immediately, not after the next deploy.
 *
 * Portal routes are deliberately absent. They are behind authentication and
 * have nothing to offer a crawler.
 */
export const dynamic = "force-dynamic";

const staticRoutes: Array<{
  path: string;
  changeFrequency: MetadataRoute.Sitemap[number]["changeFrequency"];
  priority: number;
}> = [
  { path: "/", changeFrequency: "monthly", priority: 1 },
  { path: "/about", changeFrequency: "monthly", priority: 0.8 },
  { path: "/services", changeFrequency: "weekly", priority: 0.9 },
  { path: "/work-with-me", changeFrequency: "monthly", priority: 0.9 },
  { path: "/blog", changeFrequency: "weekly", priority: 0.7 },
  { path: "/faq", changeFrequency: "monthly", priority: 0.6 },
  { path: "/contact", changeFrequency: "yearly", priority: 0.6 },
  { path: "/privacy", changeFrequency: "yearly", priority: 0.3 },
  { path: "/terms", changeFrequency: "yearly", priority: 0.3 },
];

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const now = new Date();

  const entries: MetadataRoute.Sitemap = staticRoutes.map((route) => ({
    url: `${SITE_URL}${route.path}`,
    lastModified: now,
    changeFrequency: route.changeFrequency,
    priority: route.priority,
  }));

  // A content API that is briefly unavailable should cost the crawler the
  // article list, not the whole sitemap.
  const posts = await listPosts().catch(() => []);
  for (const post of posts) {
    entries.push({
      url: `${SITE_URL}/blog/${post.slug}`,
      lastModified: post.publishedAt ? new Date(post.publishedAt) : now,
      changeFrequency: "yearly",
      priority: 0.5,
    });
  }

  return entries;
}
