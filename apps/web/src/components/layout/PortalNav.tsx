"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { ChevronDown, LogOut } from "lucide-react";
import { cn } from "@/lib/cn";

/**
 * Portal top nav — design-system §3.30 (customer top nav, portal variant) +
 * §2 (client portal content max 960px). Wordmark left, section links with
 * underline-grow, user menu right (36px avatar, custom dropdown — native
 * elements are forbidden). Sections beyond Overview are "#" placeholders
 * until their CX tasks land.
 */

const sections = [
  { href: "/portal", label: "Overview", placeholder: false },
  { href: "#", label: "Sessions", placeholder: true },
  { href: "#", label: "Forms", placeholder: true },
  { href: "#", label: "Documents", placeholder: true },
  { href: "#", label: "Payments", placeholder: true },
  { href: "#", label: "Reviews", placeholder: true },
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
        "sticky top-0 z-sticky border-b transition-colors duration-base ease-out",
        scrolled ? "border-border bg-surface/92 backdrop-blur-md" : "border-transparent",
      )}
    >
      <nav
        aria-label="Portal"
        className="mx-auto flex h-[72px] max-w-[960px] items-center justify-between gap-8 px-6"
      >
        <Link
          href="/portal"
          className="font-display text-2xl font-medium tracking-[-0.01em] text-ink"
        >
          Terios Wellness
        </Link>

        <ul className="flex flex-1 items-center gap-8 overflow-x-auto">
          {sections.map((section) => {
            const active = isActive(section.href);
            return (
              <li key={section.label}>
                <Link
                  href={section.href}
                  aria-current={active ? "page" : undefined}
                  className={cn(
                    "group relative py-1 text-sm font-medium tracking-[0.005em] whitespace-nowrap",
                    "transition-colors duration-instant ease-out",
                    active ? "font-semibold text-ink" : "text-ink-muted hover:text-ink",
                  )}
                >
                  {section.label}
                  <span
                    aria-hidden="true"
                    className={cn(
                      "absolute -bottom-[6px] left-0 h-0.5 bg-primary",
                      "transition-[width] duration-fast ease-out",
                      active ? "w-full" : "w-0 group-hover:w-full",
                    )}
                  />
                </Link>
              </li>
            );
          })}
        </ul>

        <div ref={menuRef} className="relative">
          <button
            type="button"
            aria-haspopup="menu"
            aria-expanded={menuOpen}
            onClick={() => setMenuOpen((open) => !open)}
            className={cn(
              "flex h-10 items-center gap-2 rounded-full pr-2 pl-1 text-sm font-medium text-ink",
              "transition-colors duration-fast ease-out",
              "hover:bg-surface-sunken active:bg-eucalyptus-100",
            )}
          >
            <span
              aria-hidden="true"
              className="flex size-9 items-center justify-center rounded-full bg-primary font-display text-sm text-on-primary"
            >
              {initials(userName)}
            </span>
            <span className="max-w-[140px] truncate">{userName}</span>
            <ChevronDown
              size={16}
              aria-hidden="true"
              className={cn(
                "text-ink-faint transition-transform duration-fast ease-out",
                menuOpen && "rotate-180",
              )}
            />
          </button>

          {menuOpen ? (
            <div
              role="menu"
              aria-label="Account"
              className="animate-fade-in absolute right-0 z-dropdown mt-2 w-56 rounded-md border border-border bg-surface-raised p-1 shadow-md"
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
      </nav>
    </header>
  );
}
