"use client";

import { LoaderCircle } from "lucide-react";
import { useRouter } from "next/navigation";
import { useEffect, useState, type ReactNode } from "react";
import { AdminSidebar } from "@/components/layout/AdminSidebar";
import { AdminTopbar } from "@/components/layout/AdminTopbar";
import { useAuth } from "@/lib/auth";

/**
 * Dashboard shell + route guard.
 * Unauthenticated visitors are redirected to /login; non-practitioner
 * accounts are rejected inside the auth provider (login + session restore)
 * before they can ever reach this layout.
 */
export default function DashboardLayout({ children }: { children: ReactNode }) {
  const router = useRouter();
  const { status, user, logout } = useAuth();
  const [signingOut, setSigningOut] = useState(false);

  useEffect(() => {
    if (status === "unauthenticated") {
      router.replace("/login");
    }
  }, [status, router]);

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
          <LoaderCircle size={16} aria-hidden="true" className="animate-loading" />
          Preparing your practice…
        </p>
      </main>
    );
  }

  return (
    <div className="flex min-h-screen">
      <AdminSidebar userName={user.name} />
      <div className="flex min-w-0 flex-1 flex-col">
        <AdminTopbar
          userName={user.name}
          onSignOut={handleSignOut}
          signingOut={signingOut}
        />
        <main id="admin-content" className="mx-auto w-full max-w-[1480px] flex-1 px-4 pt-7 pb-14 sm:px-6 lg:px-8 lg:pt-9">
          {children}
        </main>
      </div>
    </div>
  );
}
