"use client";

import { useState } from "react";
import { ArticleSection } from "@/components/content/ArticleSection";
import { FAQManager } from "@/components/content/FAQManager";
import { TestimonialModeration } from "@/components/content/TestimonialModeration";
import { cn } from "@/lib/cn";
import { AdminPageHeader } from "@/components/layout/AdminPageHeader";

/**
 * Site content (ADM-07) — the practitioner's own CMS.
 *
 * Four things live here and they are genuinely different kinds of content,
 * so they get tabs rather than four sidebar entries: this is one job ("look
 * after the website") done in one place, and the sidebar is for jobs, not
 * for record types.
 *
 * The tabs are local state, not routes. Nothing here is worth linking to
 * from outside the dashboard, and a route per tab would mean four screens
 * that each have to re-authenticate and re-fetch on every switch.
 */

const TABS = [
  { id: "pages", label: "Pages" },
  { id: "blog", label: "Blog" },
  { id: "faqs", label: "FAQs" },
  { id: "testimonials", label: "Testimonials" },
] as const;

type TabId = (typeof TABS)[number]["id"];

export default function ContentPage() {
  const [tab, setTab] = useState<TabId>(() => {
    if (typeof window === "undefined") return "pages";
    const requested = new URLSearchParams(window.location.search).get("tab");
    return TABS.some((item) => item.id === requested) ? (requested as TabId) : "pages";
  });

  return (
    <div data-admin-page="content" className="flex flex-col gap-6">
      <AdminPageHeader eyebrow="Publishing desk" title="Site content" description="Draft, review and publish everything the public site shows. Nothing goes live until you choose it." />

      {/* A real tablist: arrow keys move between tabs and only the selected
          panel is in the tab order, which is what a screen reader user
          expects of something that looks like this. */}
      <div role="tablist" aria-label="Content type" className="flex flex-wrap gap-1 border-b border-border">
        {TABS.map(({ id, label }, index) => (
          <button
            key={id}
            type="button"
            role="tab"
            id={`content-tab-${id}`}
            aria-selected={tab === id}
            aria-controls={`content-panel-${id}`}
            tabIndex={tab === id ? 0 : -1}
            onClick={() => setTab(id)}
            onKeyDown={(event) => {
              const step = event.key === "ArrowRight" ? 1 : event.key === "ArrowLeft" ? -1 : 0;
              if (step === 0) return;
              event.preventDefault();
              const next = TABS[(index + step + TABS.length) % TABS.length]!;
              setTab(next.id);
              document.getElementById(`content-tab-${next.id}`)?.focus();
            }}
            className={cn(
              "-mb-px border-b-2 px-4 py-2.5 text-sm font-medium transition-colors duration-instant ease-out",
              tab === id
                ? "border-primary text-ink"
                : "border-transparent text-ink-muted hover:text-ink",
            )}
          >
            {label}
          </button>
        ))}
      </div>

      {TABS.map(({ id }) =>
        tab === id ? (
          <div
            key={id}
            role="tabpanel"
            id={`content-panel-${id}`}
            aria-labelledby={`content-tab-${id}`}
            tabIndex={0}
          >
            {id === "pages" ? <ArticleSection kind="page" /> : null}
            {id === "blog" ? <ArticleSection kind="post" /> : null}
            {id === "faqs" ? <FAQManager /> : null}
            {id === "testimonials" ? <TestimonialModeration /> : null}
          </div>
        ) : null,
      )}
    </div>
  );
}
