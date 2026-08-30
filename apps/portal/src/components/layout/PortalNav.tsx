"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { ChevronDown, Globe2, Leaf, LogOut, Plus, Settings } from "lucide-react";
import { buttonClasses } from "@/components/ui/Button";
import { cn } from "@/lib/cn";

/**
 * Portal top nav — design-system §3.30 (customer top nav, portal variant) +
 * §2 (client portal content max 960px). Wordmark left, section links with
 * underline-grow, primary "Book" action, user menu right (36px avatar,
 * custom dropdown — native elements are forbidden). Sections beyond
 * Overview/Sessions are "#" placeholders until their CX tasks land.
 */

const sections = [
  { href: "/portal", label: "Overview" },
  { href: "/portal/sessions", label: "Consultations" },
  { href: "/portal/forms", label: "Forms" },
  { href: "/portal/documents", label: "Documents" },
  { href: "/portal/payments", label: "Payments" },
  { href: "/portal/reviews", label: "Reviews" },
  { href: "/portal/settings", label: "Settings" },
];

function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "·";
  const first = parts[0][0] ?? "";
  const last = parts.length > 1 ? (parts[parts.length - 1][0] ?? "") : "";
  return (first + last).toUpperCase();
}

export interface PortalNavProps {
  userName: string;
  userEmail?: string;
  onSignOut: () => void;
  signingOut?: boolean;
}

export function PortalNav({ userName, userEmail, onSignOut, signingOut = false }: PortalNavProps) {
  const pathname = usePathname();
  const [scrolled, setScrolled] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  const isActive = (href: string) => href !== "#" && pathname === href;

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 8);
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  /* While the user menu is open: Escape closes, outside clicks close. */
  useEffect(() => {
    if (!menuOpen) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        setMenuOpen(false);
      }
    };
    const onPointerDown = (event: PointerEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setMenuOpen(false);
      }
    };
    document.addEventListener("keydown", onKeyDown);
    document.addEventListener("pointerdown", onPointerDown);
    return () => {
      document.removeEventListener("keydown", onKeyDown);
      document.removeEventListener("pointerdown", onPointerDown);
    };
  }, [menuOpen]);

  return (
    <header
      className={cn(
        "sticky top-0 z-sticky border-b border-eucalyptus-800 bg-eucalyptus-900 text-sand-0 transition-shadow duration-base ease-out",
        scrolled && "shadow-[0_18px_50px_rgba(28,51,40,.2)]",
      )}
    >
      <nav
        aria-label="Portal"
        className="mx-auto grid max-w-[1040px] grid-cols-[1fr_auto] items-center gap-x-5 px-5 pb-3 pt-4 sm:px-6 lg:grid-cols-[auto_1fr_auto] lg:py-3"
      >
        <Link
          href="/portal"
          className="inline-flex items-center gap-2.5 font-display text-xl font-semibold tracking-[-0.03em] text-sand-0"
        >
          <span className="flex size-8 items-center justify-center rounded-full bg-sand-0 text-eucalyptus-900"><Leaf size={14} aria-hidden="true" /></span>Terios <span className="font-medium text-eucalyptus-300">Wellness</span>
        </Link>

        <ul className="order-3 col-span-2 mt-3 flex items-center gap-1 overflow-x-auto border-t border-sand-0/10 pt-3 lg:order-none lg:col-span-1 lg:mt-0 lg:justify-center lg:border-0 lg:pt-0">
          {sections.map((section) => {
            const active = isActive(section.href);
            return (
              <li key={section.label}>
                <Link
                  href={section.href}
                  aria-current={active ? "page" : undefined}
                  className={cn(
                    "group relative rounded-full px-3 py-2 text-sm font-medium tracking-[0.005em] whitespace-nowrap",
                    "transition-colors duration-instant ease-out",
                    active ? "bg-sand-0 text-eucalyptus-900" : "text-eucalyptus-200 hover:bg-sand-0/8 hover:text-sand-0",
                  )}
                >
                  {section.label}
                </Link>
              </li>
            );
          })}
        </ul>

        <div className="flex items-center gap-3">
          <a
            href={process.env.NEXT_PUBLIC_WEBSITE_URL ?? "https://terioscoach.com"}
            aria-label="Back to website"
            className="inline-flex h-10 items-center gap-2 rounded-full px-2.5 text-sm font-medium text-eucalyptus-200 transition-colors duration-fast hover:bg-sand-0/8 hover:text-sand-0 xl:px-3"
          >
            <Globe2 size={15} aria-hidden="true" />
            <span className="hidden xl:inline">Website</span>
          </a>
          {/* Primary action (§3.30: primary sm "Book a session" equivalent for
              the portal variant). */}
          <Link href="/portal/book" className={buttonClasses({ size: "sm", className: "hidden !bg-sand-0 !text-eucalyptus-900 shadow-none hover:!bg-eucalyptus-100 sm:inline-flex" })}>
            <Plus size={14} aria-hidden="true" /> Book
          </Link>
          <div ref={menuRef} className="relative">
          <button
            type="button"
            aria-haspopup="menu"
            aria-expanded={menuOpen}
            onClick={() => setMenuOpen((open) => !open)}
            className={cn(
              "flex h-10 items-center gap-2 rounded-full pr-2 pl-1 text-sm font-medium text-sand-0",
              "transition-colors duration-fast ease-out",
              "hover:bg-sand-0/8 active:bg-sand-0/12",
            )}
          >
            <span
              aria-hidden="true"
              className="flex size-9 items-center justify-center rounded-full bg-clay-300 font-display text-sm font-semibold text-eucalyptus-900"
            >
              {initials(userName)}
            </span>
            <span className="hidden max-w-[140px] truncate xl:block">{userName}</span>
            <ChevronDown
              size={16}
              aria-hidden="true"
              className={cn(
                "text-eucalyptus-300 transition-transform duration-fast ease-out",
                menuOpen && "rotate-180",
              )}
            />
          </button>

          {menuOpen ? (
            <div
              role="menu"
              aria-label="Account"
              className="animate-fade-in absolute right-0 z-dropdown mt-2 w-60 rounded-2xl border border-border bg-surface-raised p-2 text-ink shadow-lg"
            >
              <div className="px-3 py-2">
                <p className="text-sm font-medium text-ink">{userName}</p>
                {userEmail ? (
                  <p className="mt-0.5 truncate text-[13px] leading-[1.45] text-ink-faint">
                    {userEmail}
                  </p>
                ) : null}
              </div>
              <div aria-hidden="true" className="my-1 border-t border-border" />
              <Link role="menuitem" href="/portal/settings" onClick={() => setMenuOpen(false)} className="flex h-9 w-full items-center gap-2 rounded-sm px-3 text-sm font-medium text-ink-muted hover:bg-surface-sunken hover:text-ink"><Settings size={16} aria-hidden="true"/>Profile & preferences</Link>
              <button
                type="button"
                role="menuitem"
                disabled={signingOut}
                onClick={() => {
                  setMenuOpen(false);
                  onSignOut();
                }}
                className={cn(
                  "flex h-9 w-full items-center gap-2 rounded-sm px-3 text-sm font-medium text-danger",
                  "transition-colors duration-instant ease-out",
                  "hover:bg-danger-bg disabled:pointer-events-none disabled:opacity-50",
                )}
              >
                <LogOut size={16} aria-hidden="true" />
                {signingOut ? "Signing out…" : "Sign out"}
              </button>
            </div>
          ) : null}
          </div>
        </div>
      </nav>
    </header>
  );
}
