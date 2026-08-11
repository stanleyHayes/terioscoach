"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Menu, X } from "lucide-react";
import { buttonClasses } from "@/components/ui/Button";
import { cn } from "@/lib/cn";

const links = [
  { href: "/", label: "Home" },
  { href: "/about", label: "About" },
  { href: "/services", label: "Services" },
  { href: "/work-with-me", label: "Work With Me" },
  { href: "/blog", label: "Blog" },
  { href: "/faq", label: "FAQ" },
  { href: "/contact", label: "Contact" },
];

const wordmarkClasses =
  "font-display text-2xl font-medium tracking-[-0.01em] text-ink";

const iconButtonClasses = cn(
  "inline-flex h-10 w-10 items-center justify-center rounded-md text-ink",
  "transition-colors duration-fast ease-out",
  "hover:bg-surface-sunken active:bg-eucalyptus-100",
);

/** Customer top nav (design-system §30): sticky 72px bar, max-width 1200px,
 * underline-grow links, scrolled backdrop, full-screen mobile overlay menu. */
export function SiteNav() {
  const pathname = usePathname();
  const [scrolled, setScrolled] = useState(false);
  const [open, setOpen] = useState(false);
  const overlayRef = useRef<HTMLDivElement>(null);

  const isActive = (href: string) =>
    href === "/" ? pathname === "/" : pathname.startsWith(href);

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 8);
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  /* Close the mobile menu on navigation (adjust state during render — the
   * documented alternative to a setState-in-effect). */
  const [lastPathname, setLastPathname] = useState(pathname);
  if (lastPathname !== pathname) {
    setLastPathname(pathname);
    setOpen(false);
  }

  /* While open: move focus in, trap it, close on Escape, restore on exit. */
  useEffect(() => {
    if (!open) return;
    const overlay = overlayRef.current;
    if (!overlay) return;

    const previouslyFocused = document.activeElement as HTMLElement | null;
    const focusable = () =>
      Array.from(
        overlay.querySelectorAll<HTMLElement>("a[href], button:not([disabled])"),
      );
    focusable()[0]?.focus();

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        setOpen(false);
        return;
      }
      if (event.key !== "Tab") return;
      const items = focusable();
      if (items.length === 0) return;
      const first = items[0];
      const last = items[items.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };

    document.addEventListener("keydown", onKeyDown);
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKeyDown);
      document.body.style.overflow = "";
      previouslyFocused?.focus();
    };
  }, [open]);

  return (
    <header
      className={cn(
        "sticky top-0 z-sticky border-b transition-colors duration-base ease-out",
        scrolled || open
          ? "border-border bg-surface/92 backdrop-blur-md"
          : "border-transparent",
      )}
    >
      <nav
        aria-label="Main"
        className="mx-auto flex h-[72px] max-w-[1200px] items-center justify-between gap-8 px-6 lg:px-12"
      >
        <Link href="/" className={wordmarkClasses}>
          Terios Wellness
        </Link>

        <ul className="hidden items-center gap-8 lg:flex">
          {links.map((link) => {
            const active = isActive(link.href);
            return (
              <li key={link.href}>
                <Link
                  href={link.href}
                  aria-current={active ? "page" : undefined}
                  className={cn(
                    "group relative py-1 text-sm font-medium tracking-[0.005em]",
                    "transition-colors duration-instant ease-out",
                    active ? "font-semibold text-ink" : "text-ink-muted hover:text-ink",
                  )}
                >
                  {link.label}
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

        <div className="hidden items-center gap-2 lg:flex">
          <Link href="/login" className={buttonClasses({ variant: "ghost", size: "sm" })}>
            Sign in
          </Link>
          <Link href="/work-with-me" className={buttonClasses({ variant: "primary", size: "sm" })}>
            Book now
          </Link>
        </div>

        <button
          type="button"
          aria-label="Open menu"
          aria-expanded={open}
          aria-controls="mobile-menu"
          onClick={() => setOpen(true)}
          className={cn(iconButtonClasses, "lg:hidden")}
        >
          <Menu aria-hidden="true" className="size-6" />
        </button>
      </nav>

      {open && (
        <div
          ref={overlayRef}
          id="mobile-menu"
          role="dialog"
          aria-modal="true"
          aria-label="Site menu"
          className="animate-fade-in fixed inset-0 z-overlay flex flex-col bg-surface lg:hidden"
        >
          <div className="flex h-[72px] items-center justify-between border-b border-border px-6">
            <span className={wordmarkClasses}>Terios Wellness</span>
            <button
              type="button"
              aria-label="Close menu"
              onClick={() => setOpen(false)}
              className={iconButtonClasses}
            >
              <X aria-hidden="true" className="size-6" />
            </button>
          </div>

          <nav aria-label="Mobile" className="flex-1 overflow-y-auto px-6 py-8">
            <ul className="flex flex-col gap-1">
              {links.map((link, index) => {
                const active = isActive(link.href);
                return (
                  <li key={link.href}>
                    <Link
                      href={link.href}
                      aria-current={active ? "page" : undefined}
                      style={{ animationDelay: `${index * 40}ms` }}
                      className={cn(
                        "animate-fade-in block rounded-md px-2 py-3 text-lg font-semibold",
                        "transition-colors duration-instant ease-out",
                        active ? "text-primary" : "text-ink hover:text-primary",
                      )}
                    >
                      {link.label}
                    </Link>
                  </li>
                );
              })}
            </ul>
          </nav>

          <div className="flex flex-col gap-3 border-t border-border px-6 py-6">
            <Link href="/work-with-me" className={buttonClasses({ fullWidth: true })}>
              Book now
            </Link>
            <Link
              href="/login"
              className={buttonClasses({ variant: "ghost", fullWidth: true })}
            >
              Sign in
            </Link>
          </div>
        </div>
      )}
    </header>
  );
}
