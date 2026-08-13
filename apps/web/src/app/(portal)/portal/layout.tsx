"use client";

import { LoaderCircle } from "lucide-react";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState, type ReactNode } from "react";
import { PortalNav } from "@/components/layout/PortalNav";
import { SiteNav } from "@/components/layout/SiteNav";
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
        <SiteNav />
        <main id="main-content" className="mx-auto w-full max-w-[1040px] flex-1 px-5 pt-8 pb-20 sm:px-6 lg:pt-12">
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
      <main id="main-content" className="mx-auto w-full max-w-[1040px] flex-1 px-5 pt-8 pb-20 sm:px-6 lg:pt-12">
        {children}
      </main>
    </>
  );
}
