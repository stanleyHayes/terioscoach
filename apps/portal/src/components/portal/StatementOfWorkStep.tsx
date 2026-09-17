"use client";

import { useState, type FormEvent } from "react";
import { ScrollText } from "lucide-react";
import { ApiError } from "@/lib/api";
import type { Agreement, StatementOfWork } from "@/lib/agreements";
import { TextInput } from "@/components/ui/TextInput";
import { Button } from "@/components/ui/Button";

export function StatementOfWorkStep({ agreement, clientName, stepInfo, onSubmitted, onBack }: {
  agreement: Agreement;
  clientName: string;
  stepInfo?: { current: number; total: number };
  onSubmitted?: (answers: StatementOfWork) => Promise<void>;
  onBack: () => void;
}) {
  const [name, setName] = useState(clientName);
  const [effectiveDate, setEffectiveDate] = useState("");
  const [term, setTerm] = useState("");
  const [monthlyFee, setMonthlyFee] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [error, setError] = useState<string | null>(null);

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (submitting || !onSubmitted) return;
    const nextErrors: Record<string, string> = {};
    if (!name.trim()) nextErrors.name = "Enter the client's name.";
    const date = new Date(`${effectiveDate}T00:00:00Z`);
    if (!/^\d{4}-\d{2}-\d{2}$/.test(effectiveDate) || !Number.isFinite(date.getTime()) || date.toISOString().slice(0, 10) !== effectiveDate) {
      nextErrors.date = "Enter a valid date as YYYY-MM-DD.";
    }
    if (!/^\d+$/.test(term) || Number(term) < 1 || Number(term) > 1200) nextErrors.term = "Enter a whole number from 1 to 1200 months.";
    if (!monthlyFee.trim()) nextErrors.fee = "Enter the monthly fee, including currency.";
    setErrors(nextErrors);
    if (Object.keys(nextErrors).length) return;
    setSubmitting(true);
    setError(null);
    try {
      await onSubmitted({ clientName: name.trim(), effectiveDate, initialTermMonths: Number(term), monthlyFee: monthlyFee.trim() });
    } catch (failure) {
      setError(failure instanceof ApiError ? failure.message : "Your Statement of Work could not be submitted. Try again.");
      setSubmitting(false);
    }
  }

  return (
    <section className="flex flex-col gap-5">
      <div className="flex items-start gap-3">
        <span className="mt-0.5 flex size-10 shrink-0 items-center justify-center rounded-xl bg-eucalyptus-100 text-eucalyptus-800"><ScrollText size={18} aria-hidden="true" /></span>
        <div>
          {stepInfo && stepInfo.total > 1 ? <p className="text-xs font-semibold uppercase tracking-wider text-primary">Document {stepInfo.current} of {stepInfo.total}</p> : null}
          <h2 className="text-lg font-semibold text-ink">{agreement.title}</h2>
          <p className="mt-1 text-sm text-ink-muted">Complete these details and send them to your practitioner. No signature is required.</p>
        </div>
      </div>
      <form noValidate onSubmit={(event) => void submit(event)} className="flex flex-col gap-5 rounded-2xl border border-border bg-surface-raised p-5">
        <TextInput label="Client name" required autoComplete="name" value={name} onChange={(event) => setName(event.target.value)} maxLength={120} disabled={submitting} error={errors.name} />
        <TextInput label="Effective date" required placeholder="YYYY-MM-DD" hint="The day your services will start, for example 2026-10-01." value={effectiveDate} onChange={(event) => setEffectiveDate(event.target.value)} maxLength={10} disabled={submitting} error={errors.date} />
        <TextInput label="Initial term (months)" required inputMode="numeric" placeholder="Number of months" value={term} onChange={(event) => setTerm(event.target.value)} maxLength={4} disabled={submitting} error={errors.term} />
        <TextInput label="Monthly fee" required placeholder="e.g. USD 200" hint="Enter the amount and currency agreed with your practitioner." value={monthlyFee} onChange={(event) => setMonthlyFee(event.target.value)} maxLength={120} disabled={submitting} error={errors.fee} />
        {error ? <p role="alert" className="text-sm text-danger-ink">{error}</p> : null}
        <div className="flex flex-wrap items-center gap-3">
          <Button type="submit" loading={submitting} disabled={!onSubmitted || submitting}>{submitting ? "Submitting…" : "Submit and continue"}</Button>
          <Button type="button" variant="ghost" disabled={submitting} onClick={onBack}>Back</Button>
        </div>
      </form>
    </section>
  );
}
