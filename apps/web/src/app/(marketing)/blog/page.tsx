import type { Metadata } from "next";
import Link from "next/link";
import { BookOpen } from "lucide-react";
import { Section } from "@/components/marketing/Section";
import { PageIntro } from "@/components/marketing/PageIntro";
import { buttonClasses } from "@/components/ui/Button";
import { listPosts, type Post } from "@/lib/content";
import { formatSessionDate } from "@/lib/format";

export const metadata: Metadata = {
  title: "Journal",
  description:
    "Notes on recovery, rest and everyday wellbeing from the Terios practice — written between sessions, for the time between sessions.",
  alternates: { canonical: "/blog" },
  openGraph: {
    type: "website",
    url: "/blog",
    title: "Journal",
  },
};

// Posts are published from the dashboard and go live immediately, so this
// page must never serve a statically cached render.
export const dynamic = "force-dynamic";

interface BlogPageProps {
  searchParams: Promise<{ category?: string; tag?: string }>;
}

export default async function BlogPage({ searchParams }: BlogPageProps) {
  const { category, tag } = await searchParams;

  let posts: Post[] | null;
  try {
    posts = await listPosts({ category, tag });
  } catch {
    // A failed fetch renders a branded inline error below — never a crash.
    posts = null;
  }

  const filter = category ?? tag;

  return (
    <>
      <PageIntro eyebrow="Journal" title="Notes for the time between sessions" description="Recovery is mostly what happens after you leave. These are the things worth knowing in between — written plainly, no hurry." />

      <Section ariaLabelledby="journal-heading">
        <div className="mb-8 flex flex-wrap items-baseline justify-between gap-4">
          <h2 id="journal-heading" className="sr-only">
            Articles
          </h2>
          {filter ? (
            <p className="text-sm text-ink-muted">
              Showing{" "}
              <span className="font-medium text-ink">{filter}</span>
              <Link
                href="/blog"
                className="ml-3 text-sm font-medium text-primary transition-colors duration-instant ease-out hover:text-primary-hover"
              >
                Show everything
              </Link>
            </p>
          ) : null}
        </div>

        {posts === null ? (
          <div
            role="alert"
            className="mx-auto max-w-[480px] rounded-xl border border-border bg-surface-raised p-8 text-center"
          >
            <h3 className="font-display text-xl leading-[1.3] font-medium text-ink">
              The journal didn&rsquo;t load
            </h3>
            <p className="mt-3 text-sm leading-[1.55] text-ink-muted">
              Something interrupted the connection on our side. Nothing is
              wrong on yours — try again in a moment.
            </p>
            <div className="mt-6">
              <Link href="/blog" className={buttonClasses({ variant: "secondary" })}>
                Try again
              </Link>
            </div>
          </div>
        ) : posts.length === 0 ? (
          <div className="mx-auto flex max-w-[360px] flex-col items-center px-6 py-12 text-center">
            <span className="flex size-16 items-center justify-center rounded-full bg-surface-sunken">
              <BookOpen aria-hidden="true" className="size-8 text-ink-faint" />
            </span>
            <h3 className="mt-5 font-display text-xl leading-[1.3] font-medium text-ink">
              {filter ? "Nothing under that heading yet" : "The first note is on its way"}
            </h3>
            <p className="mt-2 text-sm leading-[1.55] text-ink-muted">
              {filter
                ? "Try the full journal — there may be something close by."
                : "When an article is published it appears here. Check back soon."}
            </p>
            {filter ? (
              <div className="mt-6">
                <Link href="/blog" className={buttonClasses({ variant: "secondary", size: "sm" })}>
                  Read everything
                </Link>
              </div>
            ) : null}
          </div>
        ) : (
          <ul className="grid gap-px overflow-hidden rounded-[2rem] border border-border bg-border lg:grid-cols-2">
            {posts.map((post, index) => (
              <li key={post.id} className={index === 0 ? "lg:col-span-2" : ""}>
                <article className={`terios-journal-entry group flex h-full flex-col bg-surface-raised p-8 transition-colors hover:bg-eucalyptus-50 ${index === 0 ? "min-h-80 justify-end sm:p-12" : "min-h-72"}`}>
                  {post.category ? (
                    <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-muted">
                      {post.category}
                    </p>
                  ) : null}
                  <h3 className={`mt-3 font-display leading-[1.05] font-semibold tracking-[-0.035em] text-ink ${index === 0 ? "max-w-[18ch] text-[clamp(2.25rem,5vw,4.5rem)]" : "text-3xl"}`}>
                    {/* The card is not a link; the title is — one target,
                        no nested interactive elements (design-system §3.21). */}
                    <Link
                      href={`/blog/${post.slug}`}
                      className="transition-colors duration-instant ease-out hover:text-primary"
                    >
                      {post.title}
                    </Link>
                  </h3>
                  {post.excerpt ? (
                    <p className="mt-3 flex-1 text-sm leading-[1.55] text-ink-muted [text-wrap:pretty]">
                      {post.excerpt}
                    </p>
                  ) : (
                    <div className="flex-1" />
                  )}
                  {post.publishedAt ? (
                    <p className="mt-8 text-[13px] tabular-nums text-ink-faint">
                      <time dateTime={post.publishedAt}>
                        {formatSessionDate(post.publishedAt, "UTC")}
                      </time>
                    </p>
                  ) : null}
                </article>
              </li>
            ))}
          </ul>
        )}
      </Section>
    </>
  );
}
