"use client";

import { useEffect, useRef, useState } from "react";
import Image from "next/image";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { ArrowUpRight, Menu, X } from "lucide-react";
import { buttonClasses } from "@/components/ui/Button";
import { cn } from "@/lib/cn";

const links = [
  { href: "/", label: "Home" },
  { href: "/about", label: "About" },
  { href: "/services", label: "Services" },
  { href: "/blog", label: "Blog" },
  { href: "/faq", label: "FAQ" },
  { href: "/contact", label: "Contact" },
];

const wordmarkClasses =
  "group inline-flex items-center gap-3 font-display text-xl font-semibold tracking-[-0.035em] text-ink";

const iconButtonClasses = cn(
  "inline-flex h-10 w-10 items-center justify-center rounded-xl border border-border/80 bg-surface-raised text-ink shadow-xs",
  "transition-[color,background-color,transform] duration-fast ease-out",
  "hover:bg-surface-sunken active:scale-[.94]",
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
    const onScroll = () => setScrolled(window.scrollY > 48);
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
      data-header-state={scrolled ? "floating" : "settled"}
      className={cn(
        "pointer-events-none sticky top-0 z-sticky transition-[padding] duration-page ease-out motion-reduce:transition-none",
        scrolled ? "px-3 pt-3 sm:px-5" : "px-0 pt-0",
      )}
    >
      <nav
        aria-label="Main"
        className={cn(
          "pointer-events-auto mx-auto flex items-center justify-between gap-6 px-4 backdrop-blur-xl transition-[height,max-width,border-radius,background-color,border-color,box-shadow] duration-page ease-out motion-reduce:transition-none sm:px-5 lg:grid lg:grid-cols-[auto_1fr_auto]",
          scrolled
            ? "h-[66px] max-w-[1240px] rounded-[1.35rem] border border-border/85 bg-surface/92 shadow-[0_18px_50px_rgba(28,51,40,.10)]"
            : "h-[76px] max-w-[100vw] rounded-none border-x-0 border-t-0 border-b border-border/65 bg-surface/96 shadow-[0_1px_0_rgba(28,51,40,.04)]",
        )}
      >
        <Link href="/" className={wordmarkClasses}>
          <span className="flex size-10 items-center justify-center rounded-[.85rem] bg-surface-raised p-1 shadow-xs transition-transform duration-base group-hover:-rotate-2"><Image src="/images/brand/identity/terios-mark.svg" alt="" width={40} height={60} className="h-full w-auto" /></span>
          <span>Terios <span className="font-medium text-ink-muted">Wellness</span></span>
        </Link>

        <ul className="mx-auto hidden items-center gap-1 rounded-full border border-border/70 bg-surface-sunken/70 p-1 lg:flex">
          {links.map((link) => {
            const active = isActive(link.href);
            return (
              <li key={link.href}>
                <Link
                  href={link.href}
                  aria-current={active ? "page" : undefined}
                  className={cn(
                    "group relative flex h-9 items-center gap-1.5 rounded-full px-3 text-[13px] font-medium tracking-[0.005em]",
                    "transition-[color,background-color,box-shadow] duration-fast ease-out",
                    active ? "bg-surface-raised font-semibold text-ink shadow-xs" : "text-ink-muted hover:bg-surface-raised/70 hover:text-ink",
                  )}
                >
                  {active ? <span aria-hidden="true" className="size-1.5 rounded-full bg-clay-500" /> : null}
                  {link.label}
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
            Book now <ArrowUpRight aria-hidden="true" className="size-3.5" />
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
          className="pointer-events-auto fixed inset-0 z-overlay flex w-full max-w-full flex-col overflow-x-hidden overflow-y-hidden bg-eucalyptus-900 text-sand-0 lg:hidden"
        >
          <div className="flex h-[76px] items-center justify-between border-b border-sand-0/12 px-6">
            <span className="inline-flex items-center gap-3 font-display text-xl font-semibold tracking-[-0.035em]"><span className="flex size-10 items-center justify-center rounded-[.85rem] bg-sand-0 p-1"><Image src="/images/brand/identity/terios-mark.svg" alt="" width={40} height={60} className="h-full w-auto" /></span>Terios Wellness</span>
            <button
              type="button"
              aria-label="Close menu"
              onClick={() => setOpen(false)}
              className="inline-flex size-10 items-center justify-center rounded-xl border border-sand-0/16 text-sand-0 transition-transform active:scale-[.94]"
            >
              <X aria-hidden="true" className="size-6" />
            </button>
          </div>

          <nav aria-label="Mobile" className="relative min-w-0 flex-1 overflow-x-hidden overflow-y-auto px-6 py-8">
            <div aria-hidden="true" className="absolute -right-24 top-8 size-64 rounded-full border border-eucalyptus-700" />
            <p className="mb-6 text-[10px] font-semibold uppercase tracking-[.16em] text-eucalyptus-300">Navigate the practice</p>
            <ul className="relative flex flex-col">
              {links.map((link, index) => {
                const active = isActive(link.href);
                return (
                  <li key={link.href}>
                    <Link
                      href={link.href}
                      aria-current={active ? "page" : undefined}
                      style={{ animationDelay: `${index * 40}ms` }}
                      className={cn(
                        "group flex items-center justify-between border-t border-sand-0/12 py-4 font-display text-[clamp(1.5rem,7vw,2.1rem)] font-semibold tracking-[-.03em]",
                        "transition-colors duration-fast ease-out",
                        active ? "text-sand-0" : "text-eucalyptus-200 hover:text-sand-0",
                      )}
                    >
                      <span className="min-w-0"><span className="mr-4 font-mono text-[10px] font-normal text-eucalyptus-400">{String(index + 1).padStart(2, '0')}</span>{link.label}</span>
                      <ArrowUpRight aria-hidden="true" className="size-5 text-eucalyptus-400 transition-transform group-hover:-translate-y-1 group-hover:translate-x-1" />
                    </Link>
                  </li>
                );
              })}
            </ul>
          </nav>

          <div className="flex flex-col gap-3 border-t border-sand-0/12 bg-eucalyptus-800/35 px-6 py-6">
            <Link href="/work-with-me" className={buttonClasses({ fullWidth: true })}>
              Book now
            </Link>
            <Link
              href="/login"
              className={buttonClasses({ variant: "secondary", fullWidth: true, className: "border-sand-0/20 text-sand-0 hover:bg-sand-0/8" })}
            >
              Sign in
            </Link>
          </div>
        </div>
      )}
    </header>
  );
}
