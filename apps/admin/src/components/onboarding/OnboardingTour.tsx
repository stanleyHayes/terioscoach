"use client";

import { CalendarDays, ChartNoAxesCombined, Sparkles, UsersRound, X } from "lucide-react";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/Button";

const KEY = "terios.admin.onboarding.complete";
const STEPS = [
  { icon: Sparkles, title: "Welcome to your practice", body: "This short tour points out the four places you will use most. You can restart it from Profile & preferences." },
  { icon: CalendarDays, title: "Run your schedule", body: "Calendar shows every consultation. Availability controls the custom dates, working hours, breaks and booking windows clients can choose." },
  { icon: UsersRound, title: "Keep care connected", body: "Clients holds records, forms, documents, private notes and shared feedback together. Open a scheduled session to start its secure video room." },
  { icon: ChartNoAxesCombined, title: "Know what needs attention", body: "Overview and page-level summaries surface bookings, payments, forms and follow-ups before you enter the detailed records." },
];

export function OnboardingTour() {
  const [open, setOpen] = useState(() => {
    if (typeof window === "undefined") return false;
    try { return localStorage.getItem(KEY) !== "true"; } catch { return false; }
  });
  const [step, setStep] = useState(0);
  useEffect(() => {
    const restart = () => { setStep(0); setOpen(true); };
    window.addEventListener("terios:onboarding", restart);
    return () => window.removeEventListener("terios:onboarding", restart);
  }, []);
  if (!open) return null;
  const current = STEPS[step]; const Icon = current.icon;
  const close = () => { try { localStorage.setItem(KEY, "true"); } catch {} setOpen(false); };
  return <div className="fixed inset-0 z-[100] grid place-items-center bg-overlay p-4 backdrop-blur-sm" role="dialog" aria-modal="true" aria-labelledby="tour-title">
    <div className="relative w-full max-w-lg overflow-hidden rounded-[2rem] border border-border bg-surface-raised p-7 shadow-lg sm:p-9">
      <button type="button" onClick={close} aria-label="Close tutorial" className="absolute right-5 top-5 grid size-9 place-items-center rounded-full text-ink-muted hover:bg-surface-sunken"><X size={18}/></button>
      <div className="mb-7 flex gap-2" aria-label={`Step ${step + 1} of ${STEPS.length}`}>{STEPS.map((_, index)=><span key={index} className={`h-1.5 flex-1 rounded-full ${index <= step ? "bg-primary" : "bg-border"}`}/>)}</div>
      <span className="grid size-14 place-items-center rounded-2xl bg-eucalyptus-100 text-eucalyptus-800"><Icon size={25}/></span>
      <p className="mt-6 text-xs font-semibold tracking-[.12em] text-primary uppercase">Getting started · {step + 1}/{STEPS.length}</p>
      <h2 id="tour-title" className="mt-2 font-display text-3xl font-semibold tracking-[-.04em] text-ink">{current.title}</h2>
      <p className="mt-3 text-sm leading-7 text-ink-muted">{current.body}</p>
      <div className="mt-8 flex justify-between gap-3"><Button variant="ghost" onClick={close}>Skip tour</Button><div className="flex gap-2">{step > 0 ? <Button variant="secondary" onClick={()=>setStep(step-1)}>Back</Button> : null}<Button onClick={()=> step === STEPS.length - 1 ? close() : setStep(step+1)}>{step === STEPS.length - 1 ? "Start working" : "Next"}</Button></div></div>
    </div>
  </div>;
}
