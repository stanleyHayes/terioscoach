"use client";

import Link from "next/link";
import { CheckCircle2, Lightbulb } from "lucide-react";
import { Modal } from "@/components/ui/Modal";
import { buttonClasses } from "@/components/ui/Button";
import type { HelpTopic } from "@/lib/help";

export function PageHelpDialog({ open, onClose, topic }: { open: boolean; onClose: () => void; topic: HelpTopic }) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title={`How to use ${topic.title}`}
      description={topic.goal}
      size="form"
      footer={<Link href="/guide" onClick={onClose} className={buttonClasses({ size: "sm" })}>Open full user guide</Link>}
    >
      <ol className="space-y-3">
        {topic.steps.map((step, index) => (
          <li key={step} className="flex gap-3 rounded-xl bg-surface-sunken px-4 py-3 text-sm leading-relaxed text-ink-muted">
            <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-primary text-xs font-semibold text-on-primary">{index + 1}</span>
            <span>{step}</span>
          </li>
        ))}
      </ol>
      {topic.tip ? <div className="mt-4 flex gap-3 rounded-xl border border-border bg-surface-raised px-4 py-3 text-sm leading-relaxed text-ink-muted"><Lightbulb className="mt-0.5 size-4 shrink-0 text-primary" aria-hidden="true"/><span><strong className="text-ink">Good to know:</strong> {topic.tip}</span></div> : null}
      <p className="mt-4 flex items-center gap-2 text-xs text-ink-faint"><CheckCircle2 className="size-4" aria-hidden="true"/>Changes save only when the page confirms them.</p>
    </Modal>
  );
}
