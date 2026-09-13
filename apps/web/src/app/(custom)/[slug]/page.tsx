import type { Metadata } from "next";
import Image from "next/image";
import { notFound } from "next/navigation";
import { getPage } from "@/lib/content";
import { ApiError } from "@/lib/api";
import { Prose } from "@/components/content/Prose";
import { Section } from "@/components/marketing/Section";
import { PageIntro } from "@/components/marketing/PageIntro";

export const dynamic = "force-dynamic";
type Props = { params: Promise<{ slug: string }> };

async function loadPage(slug: string) {
  if (slug.startsWith("website-content-")) notFound();
  try {
    return await getPage(slug);
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) notFound();
    throw error;
  }
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params;
  const page = await loadPage(slug);
  return {
    title: page.metaTitle || page.title,
    description: page.metaDescription,
    alternates: { canonical: `/${page.slug}` },
    openGraph: {
      title: page.metaTitle || page.title,
      description: page.metaDescription,
      images: page.coverImage ? [page.coverImage] : undefined,
    },
  };
}

export default async function CustomPage({ params }: Props) {
  const page = await loadPage((await params).slug);
  return (
    <>
      <PageIntro
        eyebrow="Terios Wellness"
        title={page.title}
        description={page.metaDescription || ""}
      />
      <Section>
        <article className="mx-auto max-w-3xl">
          {page.coverImage ? (
            <div className="relative mb-10 aspect-[16/9] overflow-hidden rounded-3xl">
              <Image
                src={page.coverImage}
                alt={page.title}
                fill
                unoptimized
                sizes="(min-width: 768px) 768px, 94vw"
                className="object-cover"
              />
            </div>
          ) : null}
          <Prose body={page.body} />
        </article>
      </Section>
    </>
  );
}
