"use client";

import { Calendar } from "lucide-react";
import { Card } from "@/components/ui/Card";
import { useAuth } from "@/lib/auth";

export default function OverviewPage() {
  const { user } = useAuth();

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <h1 className="text-[22px] leading-[1.3] font-semibold tracking-[-0.005em] text-ink">
          Overview
        </h1>
      </div>

      {/* EmptyState per design-system §3.27 (admin variant: heading-md title) */}
      <Card className="flex justify-center py-12">
        <div className="flex max-w-[360px] flex-col items-center px-6 text-center">
          <span className="flex size-16 items-center justify-center rounded-full bg-surface-sunken">
            <Calendar size={32} aria-hidden="true" className="text-ink-faint" />
          </span>
          <h2 className="mt-6 text-lg leading-[1.35] font-semibold text-ink">
            Nothing scheduled yet{user?.name ? `, ${user.name.split(" ")[0]}` : ""}
          </h2>
          <p className="mt-2 text-sm leading-[1.55] text-ink-muted">
            When clients book sessions, they will appear here.
          </p>
        </div>
      </Card>
    </div>
  );
}
