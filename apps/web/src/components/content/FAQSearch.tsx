"use client";

import { ChevronDown, Search, SearchX } from "lucide-react";
import Link from "next/link";
import { EmptyState } from "@/components/ui/EmptyState";
import { buttonClasses } from "@/components/ui/Button";
import { useMemo, useState } from "react";
import { cn } from "@/lib/cn";
import { groupFAQs, searchFAQs, type FAQ } from "@/lib/content";

/**
 * FAQ list with custom search and disclosure (design-system §3.15, §3.3).
 *
 * Everything here is built rather than borrowed: the search field is a
 * styled TextInput pattern, and each question is a button + region rather
 * than <details>/<summary>, whose marker and open/close behaviour cannot be
 * styled consistently across browsers. No native elements — the platform
 * rule, and the reason the disclosure animates the way the rest of the site
 * does.
 */
export interface FAQSearchProps {
  faqs: FAQ[];
}

export function FAQSearch({ faqs }: FAQSearchProps) {
  const [query, setQuery] = useState("");
  const [openId, setOpenId] = useState<string | null>(null);

  const groups = useMemo(
    () => groupFAQs(searchFAQs(faqs, query)),
    [faqs, query],
  );
  const matchCount = useMemo(
    () => searchFAQs(faqs, query).length,
    [faqs, query],
  );
  const searching = query.trim().length > 0;

  return (
    <div className="flex flex-col gap-14">
      <div className="rounded-[2rem] bg-eucalyptus-50 p-6 sm:p-8">
        <div className="max-w-[520px]">
          <label
            htmlFor="faq-search"
            className="block text-sm font-medium text-ink"
          >
            Search the questions
          </label>
          <div className="relative mt-1.5">
            <Search
              size={16}
              aria-hidden="true"
              className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-faint"
            />
            <input
              id="faq-search"
              type="text"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Try “payment” or “first session”"
              autoComplete="off"
              className={cn(
                "h-12 w-full rounded-full border border-border bg-surface-raised pl-10 pr-4 text-base text-ink shadow-sm",
                "placeholder:text-ink-faint",
                "transition-colors duration-instant ease-out",
                "focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/20",
              )}
            />
          </div>
          {/* Announced politely so a screen-reader user hears the result count
            change without the field losing focus. */}
          <p
            aria-live="polite"
            className="mt-2 min-h-[1.25rem] text-[13px] text-ink-faint"
          >
            {searching
              ? `${matchCount} ${matchCount === 1 ? "question" : "questions"} match “${query.trim()}”`
              : ""}
          </p>
        </div>
      </div>

      {matchCount === 0 ? (
        <EmptyState
          icon={<SearchX className="size-8" />}
          title="Nothing matches that"
          description="Try a different word — or send the question over and it will be answered directly."
          action={
            <>
              <button
                type="button"
                onClick={() => setQuery("")}
                className={buttonClasses({ variant: "secondary", size: "sm" })}
              >
                Clear search
              </button>
              <Link href="/contact" className={buttonClasses({ size: "sm" })}>
                Ask a question
              </Link>
            </>
          }
        />
      ) : (
        <div className="flex flex-col gap-12">
          {groups.map(([category, entries]) => (
            <section
              key={category}
              aria-labelledby={`faq-group-${slugify(category)}`}
              className="grid gap-5 lg:grid-cols-[15rem_minmax(0,1fr)] lg:gap-12"
            >
              <h2
                id={`faq-group-${slugify(category)}`}
                className="font-display text-2xl leading-[1.2] font-semibold tracking-[-0.025em] text-ink lg:sticky lg:top-32 lg:self-start"
              >
                {category}
              </h2>
              <ul className="divide-y divide-border border-y border-border">
                {entries.map((faq) => {
                  const open = openId === faq.id;
                  return (
                    <li key={faq.id}>
                      <h3>
                        <button
                          type="button"
                          aria-expanded={open}
                          aria-controls={`faq-answer-${faq.id}`}
                          onClick={() => setOpenId(open ? null : faq.id)}
                          className={cn(
                            "flex w-full items-start justify-between gap-6 py-6 text-left",
                            "transition-colors duration-instant ease-out hover:text-primary",
                            "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/30",
                          )}
                        >
                          <span className="text-lg font-medium leading-[1.45] text-ink">
                            {faq.question}
                          </span>
                          <ChevronDown
                            size={18}
                            aria-hidden="true"
                            className={cn(
                              "mt-0.5 shrink-0 text-ink-faint transition-transform duration-fast ease-out",
                              open && "rotate-180",
                            )}
                          />
                        </button>
                      </h3>
                      <div
                        id={`faq-answer-${faq.id}`}
                        role="region"
                        hidden={!open}
                        className="pb-6"
                      >
                        <p className="max-w-[68ch] text-base leading-[1.7] text-ink-muted [text-wrap:pretty] whitespace-pre-line">
                          {faq.answer}
                        </p>
                      </div>
                    </li>
                  );
                })}
              </ul>
            </section>
          ))}
        </div>
      )}
    </div>
  );
}

/** Stable id fragment for a group heading. */
function slugify(value: string): string {
  return value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
}
