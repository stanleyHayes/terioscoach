"use client";

import { LoaderCircle } from "lucide-react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState, type ReactNode } from "react";
import { PortalNav } from "@/components/layout/PortalNav";
import { buttonClasses } from "@/components/ui/Button";
import { useAuth } from "@/lib/auth";

/** The booking flow (WEB-09) is portal-side but open to visitors: guests
 * choose a service and a time first, and sign in (or register) only at the
 * confirm step, which sends them to /login?next=… and back. */
const GUEST_PATHS = ["/portal/book"];

/**
 * Portal shell + route guard.
 * Unauthenticated visitors are redirected to /login (except on guest paths);
 * while the session is being restored a branded loading state is shown.
 * Content column is 960px per design-system §2 (client portal).
 */
export default function PortalLayout({ children }: { children: ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const { status, user, logout } = useAuth();
  const [signingOut, setSigningOut] = useState(false);

  const guestAllowed = GUEST_PATHS.includes(pathname);

  useEffect(() => {
    if (status === "unauthenticated" && !guestAllowed) {
      router.replace("/login");
    }
  }, [status, guestAllowed, router]);

  async function handleSignOut() {
    setSigningOut(true);
    try {
      await logout();
      router.replace("/login");
    } finally {
      setSigningOut(false);
    }
  }

  if (status === "loading") {
    return (
      <main className="flex flex-1 items-center justify-center">
        <p className="flex items-center gap-3 text-sm text-ink-muted">
          <LoaderCircle size={16} aria-hidden="true" className="animate-spin" />
          Preparing your portal…
        </p>
      </main>
    );
  }

  // Guest on a guest path (the booking flow): slim header, no guard.
  if (status !== "authenticated" || !user) {
    if (!guestAllowed) {
      return (
        <main className="flex flex-1 items-center justify-center">
          <p className="flex items-center gap-3 text-sm text-ink-muted">
            <LoaderCircle size={16} aria-hidden="true" className="animate-spin" />
            Preparing your portal…
          </p>
        </main>
      );
    }
    return (
      <>
        <header className="sticky top-0 z-sticky border-b border-border bg-surface/92 backdrop-blur-md">
          <nav
            aria-label="Portal"
            className="mx-auto flex h-[72px] max-w-[960px] items-center justify-between gap-8 px-6"
          >
            <Link
              href="/"
              className="font-display text-2xl font-medium tracking-[-0.01em] text-ink"
            >
              Terios Wellness
            </Link>
            <Link href="/login" className={buttonClasses({ variant: "ghost", size: "sm" })}>
              Sign in
            </Link>
          </nav>
        </header>
        <main className="mx-auto w-full max-w-[960px] flex-1 px-6 pt-10 pb-16">
          {children}
        </main>
      </>
    );
  }

  return (
    <>
      <PortalNav
        userName={user.name}
        userEmail={user.email}
        onSignOut={handleSignOut}
        signingOut={signingOut}
      />
      <main className="mx-auto w-full max-w-[960px] flex-1 px-6 pt-10 pb-16">
        {children}
      </main>
    </>
  );
}
