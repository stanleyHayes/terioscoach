"use client";

import { ArrowLeft, ArrowUpRight, Check, CircleAlert, Clock3, ListChecks } from "lucide-react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useMemo, useRef, useState } from "react";
import { SlotPicker } from "@/components/booking/SlotPicker";
import { bookingStatusMeta } from "@/components/booking/booking-status";
import { Badge } from "@/components/ui/Badge";
import { Button, buttonClasses } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { EmptyState } from "@/components/ui/EmptyState";
import { ApiError, listServices, type ServiceSummary } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { createBooking, type Booking, type Slot } from "@/lib/bookings";
import { paymentsApi } from "@/lib/portal";
import {
  browserTimeZone,
  formatDuration,
  formatMoney,
  formatSessionDate,
  formatTimeRange,
  gmtOffsetLabel,
} from "@/lib/format";
import { cn } from "@/lib/cn";

/**
 * Booking flow (WEB-09) — portal-side, guest-friendly.
 * Step 1 choose a service (preselected from /work-with-me?service=) →
 * step 2 pick a time (SlotPicker, visitor timezone) → step 3 review & confirm.
 * Confirming requires an account: guests are sent to /login?next=… with the
 * chosen service + slot in the URL and land back here signed in, state intact.
 * Confirming creates the booking and then hands off to Paystack's hosted
 * checkout. The two are deliberately separate: the booking is confirmed the
 * moment it is created, so a failed or abandoned checkout costs the client
 * their money, never their slot. They can pay from the portal afterwards.
 */

type Step = "service" | "time" | "review" | "done";

const stepTitles: Record<Exclude<Step, "done">, string> = {
  service: "Choose your service",
  time: "Pick a time",
  review: "Review your booking",
};

const stepNumbers: Record<Exclude<Step, "done">, number> = {
  service: 1,
  time: 2,
  review: 3,
};

/** Brand-voice copy for create-booking failures (say what happened, no blame). */
function confirmErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.code === "service_not_found") {
      return "This service is no longer available. Choose another one.";
    }
    return error.message;
  }
  return "Something went wrong on our side. Try again in a moment.";
}

function BookingFlow() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { status, session, onTokensRefreshed } = useAuth();
  const tz = useMemo(() => browserTimeZone(), []);

  const [services, setServices] = useState<ServiceSummary[] | null>(null);
  const [servicesError, setServicesError] = useState(false);
  const [step, setStep] = useState<Step>("service");
  const [serviceId, setServiceId] = useState<string | null>(null);
  const [slot, setSlot] = useState<Slot | null>(null);
  const [conflictStartAt, setConflictStartAt] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [booking, setBooking] = useState<Booking | null>(null);
  // Set when the booking succeeded but checkout could not be opened. The
  // session is held either way; this only changes what the client is told
  // and which button leads.
  const [checkoutDeferred, setCheckoutDeferred] = useState(false);
  const initializedRef = useRef(false);

  /* Live catalog — same source as the marketing pages. */
  useEffect(() => {
    let cancelled = false;
    listServices()
      .then((items) => {
        if (!cancelled) setServices(items);
      })
      .catch(() => {
        if (!cancelled) setServicesError(true);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const service = services?.find((item) => item.id === serviceId) ?? null;

  /* Restore state from the URL once the catalog is in: ?service= preselects
   * (work-with-me "Choose" links), ?service=&slot= restores after the login
   * round trip and jumps straight to review. */
  // Runs once, on the first render where the catalogue has loaded — a
  // link with ?service=&slot= has to be applied after the services it
  // refers to exist, and the ref makes it strictly one-shot.
  useEffect(() => {
    if (initializedRef.current || !services) return;
    initializedRef.current = true;

    const serviceParam = searchParams.get("service");
    const slotParam = searchParams.get("slot");
    const match = services.find((item) => item.id === serviceParam);
    if (!match) return;

    // eslint-disable-next-line react-hooks/set-state-in-effect
    setServiceId(match.id);
    if (slotParam && !Number.isNaN(new Date(slotParam).getTime())) {
      const startAt = new Date(slotParam).toISOString();
      setSlot({
        startAt,
        endAt: new Date(
          new Date(startAt).getTime() + match.durationMinutes * 60 * 1000,
        ).toISOString(),
      });
      setStep("review");
    } else {
      setStep("time");
    }
  }, [services, searchParams]);

  function chooseService(next: ServiceSummary) {
    setServiceId(next.id);
    setSlot(null);
    setConflictStartAt(null);
    setSubmitError(null);
    setStep("time");
  }

  async function handleConfirm() {
    if (!service || !slot) return;
    setSubmitError(null);

    // Confirming requires an account — park the choice in the URL and send
    // the guest through sign-in; ?next= brings them back to this review step.
    if (status !== "authenticated" || !session) {
      const next = `/portal/book?service=${service.id}&slot=${encodeURIComponent(slot.startAt)}`;
      router.replace(`/login?next=${encodeURIComponent(next)}`);
      return;
    }

    setSubmitting(true);
    try {
      const created = await createBooking(
        session,
        { onTokensRefreshed },
        { serviceId: service.id, startAt: slot.startAt, tz },
      );
      setBooking(created);
      setStep("done");

      // Hand off to Paystack. The booking already exists and is confirmed,
      // so nothing here may undo it: if checkout cannot be opened, the
      // client keeps the slot and pays from the portal instead. Failing the
      // whole booking because a payment provider was briefly unreachable
      // would be the worse outcome by far.
      try {
        const checkoutUrl = await paymentsApi.initialize(
          session,
          { onTokensRefreshed },
          created.id,
        );
        window.location.assign(checkoutUrl);
        // Navigation is under way; leaving `submitting` set avoids a
        // flash of the confirmation screen behind the redirect.
        return;
      } catch {
        setCheckoutDeferred(true);
      }
    } catch (error) {
      if (error instanceof ApiError && error.code === "slot_unavailable") {
        // Lost the race — back to the picker, which flags the taken chip and
        // refreshes its slots.
        setConflictStartAt(slot.startAt);
        setSlot(null);
        setStep("time");
      } else {
        setSubmitError(confirmErrorMessage(error));
      }
    } finally {
      setSubmitting(false);
    }
  }

  if (servicesError) {
    return (
      <div
        role="alert"
        className="mx-auto max-w-[480px] rounded-xl border border-border bg-surface-raised p-8 text-center"
      >
        <h1 className="font-display text-xl leading-[1.3] font-medium text-ink">
          The service menu didn&rsquo;t load
        </h1>
        <p className="mt-3 text-sm leading-[1.55] text-ink-muted">
          Something interrupted the connection on our side. Try again in a moment.
        </p>
        <div className="mt-6">
          <Link href="/portal/book" className="text-sm font-medium text-primary hover:text-primary-hover">
            Try again
          </Link>
        </div>
      </div>
    );
  }

  if (!services) {
    return (
      <div role="status" aria-busy="true" className="flex flex-col gap-4">
        <span className="sr-only">Loading the service menu…</span>
        {[0, 1, 2].map((index) => (
          <span key={index} aria-hidden="true" className="h-24 rounded-lg bg-surface-sunken" />
        ))}
      </div>
    );
  }

  if (step === "done" && booking && service) {
    const meta = bookingStatusMeta[booking.status];
    return (
      <div className="animate-fade-in mx-auto flex max-w-[560px] flex-col gap-6">
        <Card>
          <div className="flex flex-col items-center py-4 text-center">
            <Badge tone={meta.tone}>{meta.label}</Badge>
            <h1 className="mt-4 font-display text-[2rem] leading-[1.15] font-medium tracking-[-0.01em] text-ink">
              You&rsquo;re booked
            </h1>
            <p className="mt-3 text-base leading-[1.6] text-ink-muted">
              {service.name}
            </p>
            <p className="mt-1 text-base leading-[1.6] tabular-nums text-ink">
              {formatSessionDate(booking.startAt, tz)}
              <span aria-hidden="true" className="mx-2 text-ink-faint">
                ·
              </span>
              {formatTimeRange(booking.startAt, booking.endAt, tz)}
            </p>
            <p className="mt-1 text-sm leading-[1.55] text-ink-faint">
              Times shown in {gmtOffsetLabel(tz)} ({tz})
            </p>
            <p className="mt-4 max-w-[48ch] text-sm leading-[1.55] text-ink-muted">
              A confirmation is on its way to your inbox. Your session lives in
              your portal now — you can reschedule or cancel there up to 24
              hours before.
            </p>
            {checkoutDeferred ? (
              <p
                role="status"
                className="mt-4 max-w-[48ch] rounded-md bg-warning-bg px-4 py-3 text-sm leading-[1.55] text-warning-ink"
              >
                We couldn&rsquo;t open the payment page just now. Your session is
                held — you can pay from your portal whenever you&rsquo;re ready.
              </p>
            ) : null}
            <div className="mt-6 flex flex-wrap justify-center gap-3">
              <Link
                href="/portal/payments"
                className={buttonClasses({ variant: checkoutDeferred ? "primary" : "secondary" })}
              >
                {checkoutDeferred ? "Pay for this session" : "View payments"}
              </Link>
              <Link
                href="/portal"
                className={buttonClasses({ variant: checkoutDeferred ? "secondary" : "primary" })}
              >
                View in your portal
              </Link>
            </div>
          </div>
        </Card>
      </div>
    );
  }

  const currentStep = step as Exclude<Step, "done">;

  return (
    <div data-portal-page="booking" className="animate-fade-in flex flex-col gap-8">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-muted">
          Book a session · Step {stepNumbers[currentStep]} of 3
        </p>
        <h1 className="mt-3 font-display text-[2rem] leading-[1.15] font-medium tracking-[-0.01em] text-ink">
          {stepTitles[currentStep]}
        </h1>
      </div>

      {step === "service" ? (
        services.length === 0 ? (
          <div className="rounded-[1.5rem] border border-border/80 bg-surface-raised shadow-soft">
            <EmptyState
              icon={<ListChecks className="size-8" />}
              title="No sessions are available to book yet"
              description="The booking flow is ready, but the practice has not published its service details, duration and price. You can still ask about care while the menu is being prepared."
              action={
                <a
                  href={`${process.env.NEXT_PUBLIC_WEBSITE_URL ?? "https://terioscoach.com"}/contact`}
                  className={buttonClasses({ variant: "secondary" })}
                >
                  Ask about care
                </a>
              }
            />
          </div>
        ) : (
          <ul className="flex flex-col gap-4">
            {services.map((item, index) => (
            <li key={item.id}>
              <button
                type="button"
                role="radio"
                aria-checked={item.id === serviceId}
                onClick={() => chooseService(item)}
                className={cn(
                  "terios-choice-card group relative grid w-full overflow-hidden border text-left sm:grid-cols-[4.5rem_minmax(0,1fr)_auto]",
                  item.id === serviceId
                    ? "is-selected border-eucalyptus-800 bg-eucalyptus-900 text-sand-0"
                    : "border-border/80 bg-surface-raised text-ink",
                )}
              >
                <span className="terios-choice-index font-display" aria-hidden="true">
                  {String(index + 1).padStart(2, "0")}
                </span>
                <span className="min-w-0 px-5 py-5 sm:px-6 sm:py-7">
                  <span className="flex items-center gap-2">
                    <span className="text-[11px] font-semibold uppercase tracking-[0.12em] text-primary group-[.is-selected]:text-eucalyptus-200">
                      One-to-one care
                    </span>
                    <span className="h-px w-8 bg-eucalyptus-200" aria-hidden="true" />
                  </span>
                  <span className="mt-3 block font-display text-[1.35rem] font-medium leading-[1.2] tracking-[-0.01em]">
                    {item.name}
                  </span>
                  <span className="mt-2 block max-w-[56ch] text-sm leading-[1.55] text-ink-muted group-[.is-selected]:text-eucalyptus-100">
                    {item.description}
                  </span>
                  <span className="mt-4 inline-flex items-center gap-2 rounded-full bg-surface-sunken px-3 py-1.5 text-xs font-semibold text-ink-muted group-[.is-selected]:bg-white/10 group-[.is-selected]:text-sand-100">
                    <Clock3 size={14} aria-hidden="true" />
                    {formatDuration(item.durationMinutes)}
                  </span>
                </span>
                <span className="flex items-center justify-between gap-5 border-t border-border/70 px-5 py-4 sm:flex-col sm:items-end sm:justify-between sm:border-t-0 sm:border-l sm:px-6 sm:py-7 group-[.is-selected]:border-white/15">
                  <span className="font-display text-xl font-medium tabular-nums">
                    {formatMoney(item.priceKobo, item.currency)}
                  </span>
                  <span className="terios-choice-action">
                    <span className="sr-only">Choose {item.name}</span>
                    {item.id === serviceId ? <Check size={16} aria-hidden="true" /> : <ArrowUpRight size={16} aria-hidden="true" />}
                  </span>
                </span>
              </button>
            </li>
            ))}
          </ul>
        )
      ) : null}

      {step === "time" && service ? (
        <div className="flex flex-col gap-6">
          <p className="text-sm leading-[1.55] text-ink-muted">
            {service.name}
            <span aria-hidden="true" className="mx-2 text-ink-faint">
              ·
            </span>
            {formatDuration(service.durationMinutes)}
          </p>
          <SlotPicker
            serviceId={service.id}
            selectedSlot={slot}
            onSelect={setSlot}
            conflictStartAt={conflictStartAt}
            timeZone={tz}
          />
          <div className="flex items-center justify-between gap-3">
            <Button variant="ghost" onClick={() => setStep("service")}>
              <ArrowLeft size={16} aria-hidden="true" />
              Back to services
            </Button>
            <Button disabled={!slot} onClick={() => setStep("review")}>
              Continue
            </Button>
          </div>
        </div>
      ) : null}

      {step === "review" && service && slot ? (
        <div className="mx-auto flex w-full max-w-[560px] flex-col gap-6">
          <Card className="terios-booking-summary p-0">
            <div className="border-b border-border/70 bg-eucalyptus-50 px-6 py-4">
              <p className="text-[11px] font-semibold uppercase tracking-[0.12em] text-primary">Your care plan</p>
            </div>
            <dl className="flex flex-col gap-4 p-6">
              <div className="flex items-baseline justify-between gap-4">
                <dt className="text-sm font-medium text-ink-muted">Service</dt>
                <dd className="text-right text-base font-semibold text-ink">
                  {service.name}
                </dd>
              </div>
              <div className="flex items-baseline justify-between gap-4">
                <dt className="text-sm font-medium text-ink-muted">When</dt>
                <dd className="text-right text-base tabular-nums text-ink">
                  {formatSessionDate(slot.startAt, tz)}
                  <span aria-hidden="true" className="mx-2 text-ink-faint">
                    ·
                  </span>
                  {formatTimeRange(slot.startAt, slot.endAt, tz)}
                </dd>
              </div>
              <div className="flex items-baseline justify-between gap-4">
                <dt className="text-sm font-medium text-ink-muted">Timezone</dt>
                <dd className="text-right text-sm text-ink">
                  {gmtOffsetLabel(tz)} ({tz})
                </dd>
              </div>
              <div className="flex items-baseline justify-between gap-4 border-t border-border pt-4">
                <dt className="text-sm font-medium text-ink-muted">Price</dt>
                <dd className="text-right font-display text-lg font-medium tabular-nums text-ink">
                  {formatMoney(service.priceKobo, service.currency)}
                </dd>
              </div>
            </dl>
            <p className="mx-6 mb-6 rounded-xl bg-surface-sunken px-4 py-3 text-[13px] leading-[1.45] font-medium tracking-[0.01em] text-ink-muted">
              Free rescheduling up to 24 hours before your session.
            </p>
          </Card>

          {submitError ? (
            <div
              role="alert"
              className="flex items-start gap-2 rounded-md bg-danger-bg px-4 py-3 text-sm leading-[1.55] text-danger-ink"
            >
              <CircleAlert size={16} aria-hidden="true" className="mt-0.5 shrink-0" />
              {submitError}
            </div>
          ) : null}

          {status !== "authenticated" ? (
            <p className="text-sm leading-[1.55] text-ink-muted">
              You&rsquo;ll sign in — or create your account in a minute — to
              confirm. Your time stays selected.
            </p>
          ) : null}

          <div className="flex items-center justify-between gap-3">
            <Button variant="secondary" onClick={() => setStep("time")}>
              <ArrowLeft size={16} aria-hidden="true" />
              Back
            </Button>
            {/* Books the session, then hands off to Paystack checkout. The
                slot is held from the moment the booking is created, so an
                abandoned or failed checkout never costs it. */}
            <Button loading={submitting} onClick={handleConfirm}>
              Confirm booking
            </Button>
          </div>
        </div>
      ) : null}
    </div>
  );
}

export default function BookPage() {
  return (
    <Suspense
      fallback={
        <div role="status" className="flex flex-col gap-4">
          <span className="sr-only">Loading booking…</span>
          <span aria-hidden="true" className="h-24 rounded-lg bg-surface-sunken" />
          <span aria-hidden="true" className="h-24 rounded-lg bg-surface-sunken" />
        </div>
      }
    >
      <BookingFlow />
    </Suspense>
  );
}
