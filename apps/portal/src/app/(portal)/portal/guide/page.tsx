import { BookOpenCheck } from "lucide-react";
import { PortalPage } from "@/components/portal/PortalPage";
import { PORTAL_HELP_TOPICS } from "@/lib/help";

export default function PortalGuidePage() {
  return (
    <PortalPage title="User guide" intro="Short, practical instructions for every main task in your portal.">
      <div className="grid gap-4 lg:grid-cols-2">
        {PORTAL_HELP_TOPICS.filter((topic) => topic.path !== "/portal/guide").map((topic) => (
          <section key={topic.path} className="rounded-[1.5rem] border border-border bg-surface-raised p-6 shadow-soft">
            <div className="flex items-start gap-3"><span className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-eucalyptus-100 text-primary"><BookOpenCheck className="size-5" aria-hidden="true"/></span><div><h2 className="font-display text-xl font-semibold text-ink">{topic.title}</h2><p className="mt-1 text-sm leading-relaxed text-ink-muted">{topic.goal}</p></div></div>
            <ol className="mt-5 space-y-2 border-t border-border pt-4">{topic.steps.map((step, index) => <li key={step} className="flex gap-3 text-sm leading-relaxed text-ink-muted"><span className="font-semibold text-primary">{index + 1}.</span><span>{step}</span></li>)}</ol>
            {topic.tip ? <p className="mt-4 rounded-xl bg-surface-sunken px-4 py-3 text-xs leading-relaxed text-ink-muted"><strong className="text-ink">Good to know:</strong> {topic.tip}</p> : null}
          </section>
        ))}
      </div>
    </PortalPage>
  );
}
