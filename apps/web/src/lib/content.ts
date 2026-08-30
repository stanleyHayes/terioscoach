/**
 * Typed client for the public content surfaces of the Terios API
 * (design/api-contract.md §Site content BE-12, §Reviews BE-14,
 * §Enquiries BE-13). Every route here is anonymous and returns live
 * content only — the API cannot serve a draft through these paths.
 *
 *   GET  /v1/content/posts            ?category=&tag= → {items} (no bodies)
 *   GET  /v1/content/posts/{slug}                     → {post}
 *   GET  /v1/content/pages/{slug}                     → {page}
 *   GET  /v1/content/faqs             ?category=      → {items}
 *   GET  /v1/content/testimonials                     → {items}
 *   GET  /v1/content/reviews          ?limit=         → {items}
 *   GET  /v1/content/reviews/summary                  → {count, average, …}
 *   POST /v1/enquiries                                → 201 {received}
 */

import { request } from "./api";

export interface Post {
  id: string;
  slug: string;
  title: string;
  excerpt?: string;
  /** Absent on the feed: listings carry summaries, not whole articles. */
  body?: string;
  coverImage?: string;
  category?: string;
  tags: string[];
  metaTitle?: string;
  metaDescription?: string;
  /** RFC 3339 UTC; present on anything the public can see. */
  publishedAt?: string;
}

export interface Page {
  id: string;
  slug: string;
  title: string;
  body: string;
  coverImage?: string;
  metaTitle?: string;
  metaDescription?: string;
  publishedAt?: string;
}

export interface FAQ {
  id: string;
  question: string;
  answer: string;
  category?: string;
  sortOrder: number;
}

export interface Testimonial {
  id: string;
  authorName: string;
  authorRole?: string;
  quote: string;
}

export interface PublicReview {
  id: string;
  authorName: string;
  serviceName?: string;
  rating: number;
  comment?: string;
  createdAt: string;
}

export interface ReviewSummary {
  count: number;
  average: number;
  distribution: Record<string, number>;
}

/** Content is edited in the dashboard and published instantly, so nothing
 * here may be served from a stale cached render. */
const live = { cache: "no-store" as const };

export interface PostFilter {
  category?: string;
  tag?: string;
}

/** GET /v1/content/posts — published posts, newest first, bodies omitted. */
export async function listPosts(filter: PostFilter = {}): Promise<Post[]> {
  const query = new URLSearchParams();
  if (filter.category) query.set("category", filter.category);
  if (filter.tag) query.set("tag", filter.tag);
  const suffix = query.toString() ? `?${query.toString()}` : "";

  const { items } = await request<{ items: Post[] }>(`/v1/content/posts${suffix}`, live);
  return items;
}

/** GET /v1/content/posts/{slug} — the full article. Throws ApiError 404
 * post_not_found for a slug that is missing *or* still a draft; the two are
 * deliberately indistinguishable. */
export async function getPost(slug: string): Promise<Post> {
  const { post } = await request<{ post: Post }>(
    `/v1/content/posts/${encodeURIComponent(slug)}`,
    live,
  );
  return post;
}

/** GET /v1/content/pages/{slug} — an editable site page. */
export async function getPage(slug: string): Promise<Page> {
  const { page } = await request<{ page: Page }>(
    `/v1/content/pages/${encodeURIComponent(slug)}`,
    live,
  );
  return page;
}

/** GET /v1/content/faqs — active entries in the practice's own order. */
export async function listFAQs(category?: string): Promise<FAQ[]> {
  const suffix = category ? `?category=${encodeURIComponent(category)}` : "";
  const { items } = await request<{ items: FAQ[] }>(`/v1/content/faqs${suffix}`, live);
  return items;
}

/** GET /v1/content/testimonials — approved quotes only. */
export async function listTestimonials(): Promise<Testimonial[]> {
  const { items } = await request<{ items: Testimonial[] }>("/v1/content/testimonials", live);
  return items;
}

/** GET /v1/content/reviews — approved client reviews. */
export async function listReviews(limit?: number): Promise<PublicReview[]> {
  const suffix = limit ? `?limit=${limit}` : "";
  const { items } = await request<{ items: PublicReview[] }>(`/v1/content/reviews${suffix}`, live);
  return items;
}

/** GET /v1/content/reviews/summary — the aggregate shown beside the list. */
export function getReviewSummary(): Promise<ReviewSummary> {
  return request<ReviewSummary>("/v1/content/reviews/summary", live);
}

export interface EnquiryInput {
  name: string;
  email: string;
  phone?: string;
  subject?: string;
  message: string;
}

/**
 * POST /v1/enquiries — the public contact form.
 *
 * The response is an acknowledgement, not a record: the API deliberately
 * returns nothing an anonymous caller could use. Throws ApiError 429
 * rate_limited when the form has been used too often from one address, and
 * 400 validation_error for bad input.
 */
export async function submitEnquiry(input: EnquiryInput): Promise<void> {
  await request<{ received: boolean }>("/v1/enquiries", {
    method: "POST",
    body: input,
  });
}

/** Groups FAQs by their category for a sectioned page. Entries with no
 * category collect under a general heading, which always sorts last so the
 * practice's own groupings lead. */
export function groupFAQs(faqs: FAQ[], fallbackLabel = "More questions"): [string, FAQ[]][] {
  const groups = new Map<string, FAQ[]>();
  for (const faq of faqs) {
    const key = faq.category?.trim() || fallbackLabel;
    const existing = groups.get(key);
    if (existing) {
      existing.push(faq);
    } else {
      groups.set(key, [faq]);
    }
  }
  return [...groups.entries()].sort(([a], [b]) => {
    if (a === fallbackLabel) return 1;
    if (b === fallbackLabel) return -1;
    return 0;
  });
}

/** Filters FAQs by a free-text query across question and answer. Matching
 * is case- and accent-insensitive so "prenatal" finds "Prénatal". */
export function searchFAQs(faqs: FAQ[], query: string): FAQ[] {
  const needle = normalize(query);
  if (!needle) return faqs;
  return faqs.filter(
    (faq) =>
      normalize(faq.question).includes(needle) ||
      normalize(faq.answer).includes(needle) ||
      normalize(faq.category ?? "").includes(needle),
  );
}

function normalize(value: string): string {
  return value
    .toLowerCase()
    .normalize("NFD")
    // Strip the combining marks NFD just separated out, so "Prénatal" and
    // "prenatal" are the same search term.
    .replace(/[\u0300-\u036f]/g, "")
    .trim();
}
