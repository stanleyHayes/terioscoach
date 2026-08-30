"use client";

import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState, type ReactNode } from "react";
import { PortalSidebar } from "@/components/layout/PortalSidebar";
import { PortalTopbar } from "@/components/layout/PortalTopbar";
import { SiteNav } from "@/components/layout/SiteNav";
import { AppSplash } from "@/components/ui/AppSplash";
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
  const [sidebarCollapsed, setSidebarCollapsed] = useState(() => {
    if (typeof window === "undefined") return false;
    try { return localStorage.getItem("terios-portal-sidebar-collapsed") === "true"; }
    catch { return false; }
  });
  const [mobileNavOpen, setMobileNavOpen] = useState(false);

  const guestAllowed = GUEST_PATHS.includes(pathname);

  useEffect(() => {
    if (status === "unauthenticated" && !guestAllowed) {
      router.replace("/login");
    }
  }, [status, guestAllowed, router]);

  useEffect(() => {
    try { localStorage.setItem("terios-portal-sidebar-collapsed", String(sidebarCollapsed)); }
    catch {}
  }, [sidebarCollapsed]);

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
    return <AppSplash />;
  }

  // Guest on a guest path (the booking flow): slim header, no guard.
  if (status !== "authenticated" || !user) {
    if (!guestAllowed) {
      return <AppSplash />;
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
    <div className="flex h-dvh overflow-hidden bg-surface">
      <PortalSidebar
        userName={user.name}
        userEmail={user.email}
        collapsed={sidebarCollapsed}
        mobileOpen={mobileNavOpen}
        onMobileClose={() => setMobileNavOpen(false)}
        onRequestExpand={() => setSidebarCollapsed(false)}
      />
      <div className="flex h-dvh min-w-0 flex-1 flex-col overflow-hidden">
      <PortalTopbar
        userName={user.name}
        userEmail={user.email}
        onSignOut={handleSignOut}
        signingOut={signingOut}
        collapsed={sidebarCollapsed}
        onToggleCollapse={() => setSidebarCollapsed((value) => !value)}
        onOpenMobileNav={() => setMobileNavOpen(true)}
      />
      <main id="main-content" className="min-h-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto overscroll-contain">
        <div className="mx-auto w-full max-w-[1180px] px-4 pt-7 pb-14 sm:px-6 lg:px-8 lg:pt-9">{children}</div>
      </main>
      </div>
    </div>
  );
}
