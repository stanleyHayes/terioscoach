"use client";

import { LoaderCircle } from "lucide-react";
import { useRouter } from "next/navigation";
import { useEffect, useState, type ReactNode } from "react";
import { AdminSidebar } from "@/components/layout/AdminSidebar";
import { AdminTopbar } from "@/components/layout/AdminTopbar";
import { useAuth } from "@/lib/auth";

/**
 * Dashboard shell + route guard.
 * Unauthenticated visitors are redirected to /login; client accounts are
 * rejected inside the auth provider (login + session restore)
 * before they can ever reach this layout.
 */
export default function DashboardLayout({ children }: { children: ReactNode }) {
  const router = useRouter();
  const { status, user, logout } = useAuth();
  const [signingOut, setSigningOut] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(() => {
    if (typeof window === "undefined") return false;
    try {
      return localStorage.getItem("terios-admin-sidebar-collapsed") === "true";
    } catch {
      return false;
    }
  });
  const [mobileNavOpen, setMobileNavOpen] = useState(false);

  useEffect(() => {
    if (status === "unauthenticated") {
      router.replace("/login");
    }
  }, [status, router]);

  useEffect(() => {
    try {
      localStorage.setItem(
        "terios-admin-sidebar-collapsed",
        String(sidebarCollapsed),
      );
    } catch {}
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

  if (status !== "authenticated" || !user) {
    return (
      <main className="flex min-h-screen items-center justify-center bg-surface">
        <p className="flex items-center gap-3 text-sm text-ink-muted">
          <LoaderCircle
            size={16}
            aria-hidden="true"
            className="animate-loading"
          />
          Preparing your practice…
        </p>
      </main>
    );
  }

  return (
    <div className="flex h-dvh overflow-hidden bg-surface">
      <AdminSidebar
        userName={user.name}
        userRole={
          user.role === "practitioner" ? "Owner" : user.roleName || "Staff"
        }
        owner={user.role === "practitioner"}
        permissions={user.permissions}
        collapsed={sidebarCollapsed}
        mobileOpen={mobileNavOpen}
        onMobileClose={() => setMobileNavOpen(false)}
        onRequestExpand={() => setSidebarCollapsed(false)}
      />
      <div className="flex h-dvh min-w-0 flex-1 flex-col overflow-hidden">
        <AdminTopbar
          userName={user.name}
          userRole={user.role === "practitioner" ? "Owner" : user.roleName || "Staff"}
          onSignOut={handleSignOut}
          signingOut={signingOut}
          collapsed={sidebarCollapsed}
          onToggleCollapse={() => setSidebarCollapsed((value) => !value)}
          onOpenMobileNav={() => setMobileNavOpen(true)}
        />
        <main
          id="admin-content"
          className="min-h-0 flex-1 overflow-y-auto overscroll-contain"
        >
          <div className="mx-auto w-full max-w-[1480px] px-4 pt-7 pb-14 sm:px-6 lg:px-8 lg:pt-9">
            {children}
          </div>
        </main>
      </div>
    </div>
  );
}
