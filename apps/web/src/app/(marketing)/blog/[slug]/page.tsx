import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ArrowLeft } from "lucide-react";
import { Prose } from "@/components/content/Prose";
import { Section } from "@/components/marketing/Section";
import { ApiError } from "@/lib/api";
import { getPost, type Post } from "@/lib/content";
import { formatSessionDate } from "@/lib/format";

export const dynamic = "force-dynamic";

interface ArticlePageProps {
  params: Promise<{ slug: string }>;
}

/** Loads the article, turning the API's "missing or still a draft" 404 into
 * Next's own not-found. The two cases are indistinguishable by design. */
async function loadPost(slug: string): Promise<Post> {
  try {
    return await getPost(slug);
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) {
      notFound();
    }
    throw error;
  }
}

export async function generateMetadata({ params }: ArticlePageProps): Promise<Metadata> {
  const { slug } = await params;
  let post: Post;
  try {
    post = await getPost(slug);
  } catch {
    // Metadata must never be the thing that breaks a page; the page's own
    // loader decides between not-found and an error.
    return { title: "Journal" };
  }

  const description = post.metaDescription ?? post.excerpt;
  return {
    title: post.metaTitle ?? post.title,
    description,
    openGraph: {
      type: "article",
      title: post.metaTitle ?? post.title,
      description,
      publishedTime: post.publishedAt,
      images: post.coverImage ? [{ url: post.coverImage }] : undefined,
    },
  };
}

export default async function ArticlePage({ params }: ArticlePageProps) {
  const { slug } = await params;
  const post = await loadPost(slug);

  return (
    <>
      <Section background="night" className="border-b border-eucalyptus-800" containerClassName="py-20 lg:py-28">
        <div className="max-w-[68ch]">
          <Link
            href="/blog"
            className="inline-flex items-center gap-2 text-sm font-medium text-eucalyptus-300 transition-colors duration-instant ease-out hover:text-sand-0"
          >
            <ArrowLeft size={16} aria-hidden="true" />
            All notes
          </Link>

          {post.category ? (
            <p className="mt-10 text-[11px] font-semibold uppercase tracking-[0.12em] text-eucalyptus-300">
              {post.category}
            </p>
          ) : null}
          <h1 className="mt-5 font-display text-[clamp(3rem,7vw,6.5rem)] leading-[.92] font-semibold tracking-[-0.055em] text-sand-0 [text-wrap:balance]">
            {post.title}
          </h1>
          {post.publishedAt ? (
            <p className="mt-6 text-[13px] tabular-nums text-eucalyptus-300">
              <time dateTime={post.publishedAt}>
                {formatSessionDate(post.publishedAt, "UTC")}
              </time>
            </p>
          ) : null}
          {post.excerpt ? (
            <p className="mt-7 max-w-[58ch] text-lg leading-[1.7] text-eucalyptus-200 [text-wrap:pretty]">
              {post.excerpt}
            </p>
          ) : null}
        </div>
      </Section>

      <Section>
        <article>
          {post.coverImage ? (
            /* The CMS stores a Cloudinary URL; the domain refuses anything
               that is not http(s) or site-relative, so this cannot become a
               script URL. Plain <img>: the source is remote and arbitrary,
               which next/image would need configured hosts for. */
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={post.coverImage}
              alt={`Cover image for ${post.title}`}
              className="mb-10 w-full max-w-[68ch] rounded-xl border border-border object-cover"
            />
          ) : null}
          {post.body ? <Prose body={post.body} /> : null}

          {post.tags.length > 0 ? (
            <ul className="mt-12 flex max-w-[68ch] flex-wrap gap-2">
              {post.tags.map((tag) => (
                <li key={tag}>
                  <Link
                    href={`/blog?tag=${encodeURIComponent(tag)}`}
                    className="inline-flex rounded-full bg-surface-sunken px-3 py-1 text-[13px] text-ink-muted transition-colors duration-instant ease-out hover:text-ink"
                  >
                    {tag}
                  </Link>
                </li>
              ))}
            </ul>
          ) : null}
        </article>
      </Section>
    </>
  );
}
