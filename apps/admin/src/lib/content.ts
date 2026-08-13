/**
 * Typed client for the CMS slice (design/api-contract.md §Site content BE-12).
 *
 *   GET/POST            /v1/admin/content/pages
 *   GET/PATCH/DELETE    /v1/admin/content/pages/{id}
 *   POST                /v1/admin/content/pages/{id}/publish | /unpublish
 *   …the same shape for posts
 *   GET/POST            /v1/admin/content/faqs
 *   PATCH/DELETE        /v1/admin/content/faqs/{id}
 *   GET/POST            /v1/admin/content/testimonials
 *   PATCH/DELETE        /v1/admin/content/testimonials/{id}
 *   POST                /v1/admin/content/testimonials/{id}/approve | /reject
 *
 * Publishing and approving are their own routes on purpose: a PATCH cannot
 * change status, so an edit can never put content live by accident.
 */

import { authedRequest, type RefreshCallbacks, type Session } from "@/lib/api";

export type ContentStatus = "draft" | "published";
export type Moderation = "pending" | "approved" | "rejected";

export interface Page {
  id: string;
  slug: string;
  title: string;
  body: string;
  metaTitle?: string;
  metaDescription?: string;
  status: ContentStatus;
  publishedAt?: string;
  createdAt: string;
  updatedAt: string;
}

export interface Post extends Page {
  excerpt?: string;
  coverImage?: string;
  category?: string;
  tags: string[];
}

export interface FAQ {
  id: string;
  question: string;
  answer: string;
  category?: string;
  sortOrder: number;
  active: boolean;
}

export interface Testimonial {
  id: string;
  authorName: string;
  authorRole?: string;
  quote: string;
  status: Moderation;
  sortOrder: number;
  submittedAt: string;
  approvedAt?: string;
}

/** Create payload. The slug is normalized server-side, so an editor can
 * type a title here and get a usable URL. */
export interface ArticleDraft {
  slug: string;
  title: string;
  body: string;
}

export type PagePatch = Partial<{
  slug: string;
  title: string;
  body: string;
  metaTitle: string;
  metaDescription: string;
}>;

export type PostPatch = PagePatch &
  Partial<{
    excerpt: string;
    coverImage: string;
    category: string;
    tags: string[];
  }>;

/** Shared CRUD shape for pages and posts, which differ only in their extra
 * fields — writing it once keeps the two editors honest with each other. */
function articleApi<T, P>(kind: "pages" | "posts", key: "page" | "post") {
  const base = `/v1/admin/content/${kind}`;
  return {
    async list(session: Session, callbacks: RefreshCallbacks): Promise<T[]> {
      const { items } = await authedRequest<{ items: T[] }>(base, session, callbacks);
      return items;
    },

    async create(session: Session, callbacks: RefreshCallbacks, draft: ArticleDraft): Promise<T> {
      const data = await authedRequest<Record<string, T>>(base, session, callbacks, {
        method: "POST",
        body: draft,
      });
      return data[key];
    },

    async update(
      session: Session,
      callbacks: RefreshCallbacks,
      id: string,
      patch: P,
    ): Promise<T> {
      const data = await authedRequest<Record<string, T>>(`${base}/${id}`, session, callbacks, {
        method: "PATCH",
        body: patch,
      });
      return data[key];
    },

    /** Publishing is its own transition — never a side effect of a save. */
    async setPublished(
      session: Session,
      callbacks: RefreshCallbacks,
      id: string,
      published: boolean,
    ): Promise<T> {
      const data = await authedRequest<Record<string, T>>(
        `${base}/${id}/${published ? "publish" : "unpublish"}`,
        session,
        callbacks,
        { method: "POST" },
      );
      return data[key];
    },

    remove(session: Session, callbacks: RefreshCallbacks, id: string): Promise<void> {
      return authedRequest<void>(`${base}/${id}`, session, callbacks, { method: "DELETE" });
    },
  };
}

export const pagesApi = articleApi<Page, PagePatch>("pages", "page");
export const postsApi = articleApi<Post, PostPatch>("posts", "post");

export interface FAQDraft {
  question: string;
  answer: string;
  category?: string;
  sortOrder?: number;
}

export const faqsApi = {
  async list(session: Session, callbacks: RefreshCallbacks): Promise<FAQ[]> {
    const { items } = await authedRequest<{ items: FAQ[] }>(
      "/v1/admin/content/faqs",
      session,
      callbacks,
    );
    return items;
  },

  async create(session: Session, callbacks: RefreshCallbacks, draft: FAQDraft): Promise<FAQ> {
    const { faq } = await authedRequest<{ faq: FAQ }>(
      "/v1/admin/content/faqs",
      session,
      callbacks,
      { method: "POST", body: draft },
    );
    return faq;
  },

  async update(
    session: Session,
    callbacks: RefreshCallbacks,
    id: string,
    patch: Partial<FAQDraft & { active: boolean }>,
  ): Promise<FAQ> {
    const { faq } = await authedRequest<{ faq: FAQ }>(
      `/v1/admin/content/faqs/${id}`,
      session,
      callbacks,
      { method: "PATCH", body: patch },
    );
    return faq;
  },

  remove(session: Session, callbacks: RefreshCallbacks, id: string): Promise<void> {
    return authedRequest<void>(`/v1/admin/content/faqs/${id}`, session, callbacks, {
      method: "DELETE",
    });
  },
};

export interface TestimonialDraft {
  authorName: string;
  authorRole?: string;
  quote: string;
}

export const testimonialsApi = {
  async list(
    session: Session,
    callbacks: RefreshCallbacks,
    status?: Moderation,
  ): Promise<Testimonial[]> {
    const suffix = status ? `?status=${status}` : "";
    const { items } = await authedRequest<{ items: Testimonial[] }>(
      `/v1/admin/content/testimonials${suffix}`,
      session,
      callbacks,
    );
    return items;
  },

  async create(
    session: Session,
    callbacks: RefreshCallbacks,
    draft: TestimonialDraft,
  ): Promise<Testimonial> {
    const { testimonial } = await authedRequest<{ testimonial: Testimonial }>(
      "/v1/admin/content/testimonials",
      session,
      callbacks,
      { method: "POST", body: draft },
    );
    return testimonial;
  },

  /** Approving publishes to the site; rejecting takes it off. Both are
   * reversible — moderation is a judgement, not a lifecycle. */
  async moderate(
    session: Session,
    callbacks: RefreshCallbacks,
    id: string,
    approve: boolean,
  ): Promise<Testimonial> {
    const { testimonial } = await authedRequest<{ testimonial: Testimonial }>(
      `/v1/admin/content/testimonials/${id}/${approve ? "approve" : "reject"}`,
      session,
      callbacks,
      { method: "POST" },
    );
    return testimonial;
  },

  remove(session: Session, callbacks: RefreshCallbacks, id: string): Promise<void> {
    return authedRequest<void>(`/v1/admin/content/testimonials/${id}`, session, callbacks, {
      method: "DELETE",
    });
  },
};

/**
 * The same slug rule the server applies, so the editor can show the URL a
 * title will produce before saving. The server remains the authority; this
 * only has to agree with it.
 */
export function slugify(raw: string): string {
  const transliterations: Record<string, string> = {
    á: "a", à: "a", â: "a", ä: "a", ã: "a", å: "a", æ: "ae",
    ç: "c",
    é: "e", è: "e", ê: "e", ë: "e",
    í: "i", ì: "i", î: "i", ï: "i",
    ñ: "n",
    ó: "o", ò: "o", ô: "o", ö: "o", õ: "o", ø: "o", œ: "oe",
    ú: "u", ù: "u", û: "u", ü: "u",
    ý: "y", ÿ: "y",
    ß: "ss", ð: "d", þ: "th",
  };

  let out = "";
  let lastHyphen = true;
  for (const character of raw.toLowerCase().trim()) {
    const ascii = transliterations[character];
    if (ascii) {
      out += ascii;
      lastHyphen = false;
      continue;
    }
    if (/[a-z0-9]/.test(character)) {
      out += character;
      lastHyphen = false;
    } else if (/[-_ /]/.test(character) && !lastHyphen) {
      out += "-";
      lastHyphen = true;
    }
  }
  return out.replace(/-+$/, "");
}
