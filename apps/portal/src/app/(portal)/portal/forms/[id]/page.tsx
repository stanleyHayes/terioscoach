"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { ArrowLeft, CircleCheck } from "lucide-react";
import { useState } from "react";
import { FormField } from "@/components/portal/FormField";
import { PortalError, PortalLoading, PortalPage } from "@/components/portal/PortalPage";
import { SignaturePad } from "@/components/portal/SignaturePad";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { TextInput } from "@/components/ui/TextInput";
import { formsApi, type Answer, type SubmissionView } from "@/lib/portal";
import { usePortalAction, usePortalData } from "@/lib/use-portal-data";

/**
 * Fill in and sign one form (CX-07).
 *
 * The definition comes from the server with the submission, so what is
 * rendered is the version of the form that was actually sent — not whatever
 * the practice has edited it into since.
 *
 * Validation here mirrors the API's, which is the authority: catching a
 * blank required field before the round trip is a courtesy, not the rule.
 * A submitted form renders read-only, because the API will refuse a second
 * submit and a form that looks editable but is not would be worse than one
 * that plainly is not.
 */
export default function FormPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const submissionId = params.id;

  const view = usePortalData<SubmissionView>(
    (session, callbacks) => formsApi.get(session, callbacks, submissionId),
    [submissionId],
  );
  const action = usePortalAction();

  const [answers, setAnswers] = useState<Record<string, Answer>>({});
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [typedName, setTypedName] = useState("");
  const [signature, setSignature] = useState<string | null>(null);
  const [signatureError, setSignatureError] = useState<string | null>(null);
  const [done, setDone] = useState(false);

  const data = view.data;
  const submitted = data?.submission.status === "submitted";
  const needsSignature = data?.form.fields.some((field) => field.type === "signature") ?? false;

  function answerFor(key: string): Answer {
    return answers[key] ?? data?.submission.answers[key] ?? {};
  }

  async function submit() {
    if (!data) return;
    const found = validate(data, answers);
    setErrors(found);

    let signatureProblem: string | null = null;
    if (needsSignature) {
      if (!typedName.trim()) signatureProblem = "Please type your name to sign.";
      else if (!signature) signatureProblem = "Please draw your signature above.";
    }
    setSignatureError(signatureProblem);

    if (Object.keys(found).length > 0 || signatureProblem) return;

    const merged: Record<string, Answer> = { ...data.submission.answers, ...answers };
    const result = await action.run("submit", (session, callbacks) =>
      formsApi.submit(session, callbacks, submissionId, {
        answers: merged,
        signature:
          needsSignature && signature
            ? { typedName: typedName.trim(), imageData: signature }
            : undefined,
      }),
    );
    if (result) {
      setDone(true);
      // The list is the natural place to land, and it will show this form
      // as completed.
      router.refresh();
    }
  }

  if (done) {
    return (
      <PortalPage title="Thank you">
        <Card>
          <div role="status" className="flex flex-col items-center gap-4 py-10 text-center">
            <span className="flex size-14 items-center justify-center rounded-full bg-success-bg">
              <CircleCheck aria-hidden="true" className="size-7 text-success-ink" />
            </span>
            <h2 className="font-display text-[1.5rem] font-medium text-ink">
              Your form has been sent
            </h2>
            <p className="max-w-[46ch] text-sm leading-[1.55] text-ink-muted">
              Your practitioner has it. You can read it back any time from
              your forms.
            </p>
            <Link
              href="/portal/forms"
              className="text-sm font-medium text-primary transition-colors duration-instant ease-out hover:text-primary-hover"
            >
              Back to your forms
            </Link>
          </div>
        </Card>
      </PortalPage>
    );
  }

  return (
    <PortalPage title={data?.submission.formTitle ?? "Form"}>
      <Link
        href="/portal/forms"
        className="inline-flex w-fit items-center gap-2 text-sm font-medium text-ink-muted transition-colors duration-instant ease-out hover:text-ink"
      >
        <ArrowLeft size={16} aria-hidden="true" />
        All forms
      </Link>

      {view.error ? (
        <PortalError message={view.error} onRetry={view.refresh} />
      ) : data === null ? (
        <PortalLoading label="Loading the form…" rows={3} />
      ) : (
        <Card>
          <div data-portal-page="form-response" className="flex flex-col gap-8">
            {data.form.description ? (
              <p className="max-w-[68ch] text-base leading-[1.6] text-ink-muted">
                {data.form.description}
              </p>
            ) : null}

            {submitted ? (
              <p className="rounded-lg bg-surface-sunken px-4 py-3 text-sm text-ink-muted">
                You completed this form
                {data.submission.submittedAt ? (
                  <>
                    {" on "}
                    <time dateTime={data.submission.submittedAt}>
                      {new Date(data.submission.submittedAt).toLocaleDateString("en-GB", {
                        day: "numeric",
                        month: "short",
                        year: "numeric",
                      })}
                    </time>
                  </>
                ) : null}
                . It is kept as a record and cannot be changed.
              </p>
            ) : null}

            {action.error ? (
              <p role="alert" className="text-sm text-danger-ink">
                {action.error}
              </p>
            ) : null}

            <div className="flex flex-col gap-6">
              {data.form.fields
                .filter((field) => field.type !== "signature")
                .map((field) => (
                  <FormField
                    key={field.key}
                    field={field}
                    answer={answerFor(field.key)}
                    error={errors[field.key]}
                    disabled={submitted}
                    onChange={(answer) =>
                      setAnswers((current) => ({ ...current, [field.key]: answer }))
                    }
                  />
                ))}
            </div>

            {needsSignature ? (
              <div className="flex flex-col gap-4 border-t border-border pt-6">
                {submitted ? (
                  <div>
                    <h2 className="text-sm font-medium text-ink">Signed by</h2>
                    <p className="mt-1 text-base text-ink">
                      {data.submission.signature?.typedName}
                    </p>
                    {data.signatureImage ? (
                      // eslint-disable-next-line @next/next/no-img-element
                      <img
                        src={data.signatureImage}
                        alt="Your signature"
                        className="mt-3 h-24 rounded-lg border border-border bg-surface-raised"
                      />
                    ) : null}
                  </div>
                ) : (
                  <>
                    <TextInput
                      label="Type your full name to sign"
                      required
                      value={typedName}
                      onChange={(event) => setTypedName(event.target.value)}
                      error={
                        signatureError && !typedName.trim() ? signatureError : undefined
                      }
                    />
                    <SignaturePad
                      label="Draw your signature"
                      onChange={setSignature}
                      describedBy="signature-help"
                    />
                    <p id="signature-help" className="text-[13px] text-ink-muted">
                      Use a finger, a stylus or your mouse. Typing your name and
                      drawing your mark together form your signature.
                    </p>
                    {signatureError && typedName.trim() ? (
                      <p role="alert" className="text-[13px] text-danger-ink">
                        {signatureError}
                      </p>
                    ) : null}
                  </>
                )}
              </div>
            ) : null}

            {!submitted ? (
              <div className="flex flex-wrap items-center gap-4 border-t border-border pt-6">
                <Button loading={action.pending === "submit"} onClick={() => void submit()}>
                  Send to your practitioner
                </Button>
                <p className="text-[13px] leading-[1.5] text-ink-faint">
                  Once sent, this becomes a record and cannot be edited.
                </p>
              </div>
            ) : null}
          </div>
        </Card>
      )}
    </PortalPage>
  );
}

/**
 * Mirrors the API's own rules so the common mistakes are caught before the
 * round trip. The server remains the authority — it validates against the
 * form definition it holds, not the one the browser was given.
 */
export function validate(
  view: SubmissionView,
  answers: Record<string, Answer>,
): Record<string, string> {
  const errors: Record<string, string> = {};

  for (const field of view.form.fields) {
    if (field.type === "signature") continue;

    const answer = answers[field.key] ?? view.submission.answers[field.key] ?? {};
    const value = (answer.value ?? "").trim();
    const values = answer.values ?? [];
    const empty = value === "" && values.length === 0;

    if (field.required && empty) {
      errors[field.key] = "This one is needed.";
      continue;
    }
    if (empty) continue;

    if (field.type === "number" && Number.isNaN(Number(value))) {
      errors[field.key] = "Please enter a number.";
    }
    if (field.type === "date" && !/^\d{4}-\d{2}-\d{2}$/.test(value)) {
      errors[field.key] = "Please use the format YYYY-MM-DD.";
    }
    if (
      (field.type === "select" || field.type === "radio") &&
      value !== "" &&
      !field.options.includes(value)
    ) {
      errors[field.key] = "Please choose one of the options.";
    }
  }

  return errors;
}
