import Link from "next/link";

const exploreLinks = [
  { href: "/", label: "Home" },
  { href: "/about", label: "About" },
  { href: "/blog", label: "Blog" },
  { href: "/faq", label: "FAQ" },
];

const practiceLinks = [
  { href: "/services", label: "Services" },
  { href: "/work-with-me", label: "Work With Me" },
  { href: "/contact", label: "Contact" },
];

const columnHeadingClasses =
  "text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-faint";

const columnLinkClasses =
  "text-sm text-ink-muted transition-colors duration-instant ease-out hover:text-ink";

function FooterColumn({
  label,
  links,
}: {
  label: string;
  links: Array<{ href: string; label: string }>;
}) {
  return (
    <nav aria-label={label}>
      <h2 className={columnHeadingClasses}>{label}</h2>
      <ul className="mt-4 flex flex-col gap-3">
        {links.map((link) => (
          <li key={link.href}>
            <Link href={link.href} className={columnLinkClasses}>
              {link.label}
            </Link>
          </li>
        ))}
      </ul>
    </nav>
  );
}

/** Marketing footer (design-system §30 companion). The botanical glyph ❦ is
 * reserved for this footer only (brand.md §2). */
export function SiteFooter() {
  const year = new Date().getFullYear();

  return (
    <footer className="border-t border-border bg-surface">
      <div className="mx-auto max-w-[1200px] px-6 py-16 lg:px-12">
        <div className="grid gap-12 md:grid-cols-3">
          <div>
            <Link
              href="/"
              className="font-display text-2xl font-medium tracking-[-0.01em] text-ink"
            >
              Terios Wellness
            </Link>
            <p className="mt-4 max-w-[40ch] text-sm leading-[1.55] text-ink-muted">
              Clinical calm — the trust of a nurse, the exhale of a spa.
              Video-first wellness care, wherever you are.
            </p>
          </div>
          <FooterColumn label="Explore" links={exploreLinks} />
          <FooterColumn label="Practice" links={practiceLinks} />
        </div>

        <div className="mt-16 flex items-center justify-between border-t border-border pt-6">
          <p className="text-[13px] font-medium tracking-[0.01em] text-ink-faint">
            © {year} Terios Wellness Spa
          </p>
          <span aria-hidden="true" className="text-ink-faint">
            ❦
          </span>
        </div>
      </div>
    </footer>
  );
}
