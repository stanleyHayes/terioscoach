import { BookOpenCheck } from "lucide-react";
import { AdminPageHeader } from "@/components/layout/AdminPageHeader";
import { ADMIN_HELP_TOPICS } from "@/lib/help";

export default function UserGuidePage() {
  return (
    <div data-admin-page="guide" className="flex flex-col gap-6">
      <AdminPageHeader eyebrow="Help centre" title="User guide" description="A practical guide to completing the main job in every practice workspace." />
      <div className="grid gap-4 xl:grid-cols-2">
        {ADMIN_HELP_TOPICS.filter((topic) => topic.path !== "/guide").map((topic) => (
          <section key={topic.path} className="rounded-[1.5rem] border border-border bg-surface-raised p-6 shadow-soft">
            <div className="flex items-start gap-3"><span className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-eucalyptus-100 text-primary"><BookOpenCheck className="size-5" aria-hidden="true"/></span><div><h2 className="font-display text-xl font-semibold text-ink">{topic.title}</h2><p className="mt-1 text-sm leading-relaxed text-ink-muted">{topic.goal}</p></div></div>
            <ol className="mt-5 space-y-2 border-t border-border pt-4">{topic.steps.map((step, index) => <li key={step} className="flex gap-3 text-sm leading-relaxed text-ink-muted"><span className="font-semibold text-primary">{index + 1}.</span><span>{step}</span></li>)}</ol>
            {topic.tip ? <p className="mt-4 rounded-xl bg-surface-sunken px-4 py-3 text-xs leading-relaxed text-ink-muted"><strong className="text-ink">Good to know:</strong> {topic.tip}</p> : null}
          </section>
        ))}
      </div>
    </div>
  );
}
