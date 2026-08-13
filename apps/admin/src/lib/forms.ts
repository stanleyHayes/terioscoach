/**
 * Typed client for the forms slice (design/api-contract.md §Forms BE-10).
 *
 *   GET/POST         /v1/admin/forms
 *   GET/PATCH/DELETE /v1/admin/forms/{id}
 *   POST             /v1/admin/forms/assign
 *   GET              /v1/admin/forms/submissions[?clientId|formId|bookingId|status]
 *   GET              /v1/admin/forms/submissions/{id}
 *
 * A submission is one-way: there is no route to un-submit one and none to
 * edit an answer after signing. That is the point of a consent record, and
 * it is why this file has no `updateSubmission`.
 */

import { authedRequest, type RefreshCallbacks, type Session } from "@/lib/api";

export type FieldType =
  | "text"
  | "textarea"
  | "number"
  | "date"
  | "select"
  | "radio"
  | "checkbox"
  | "signature";

/** The types the server requires options for; anything else must have none. */
export const CHOICE_TYPES: FieldType[] = ["select", "radio", "checkbox"];

export const FIELD_TYPES: { value: FieldType; label: string; hint: string }[] = [
  { value: "text", label: "Short answer", hint: "A name, a phone number, one line." },
  { value: "textarea", label: "Long answer", hint: "A paragraph or more." },
  { value: "number", label: "Number", hint: "Age, weight, a count." },
  { value: "date", label: "Date", hint: "A day, picked from a calendar." },
  { value: "select", label: "Pick one from a list", hint: "For longer lists of options." },
  { value: "radio", label: "Pick one", hint: "For three or four options, all shown." },
  { value: "checkbox", label: "Pick any", hint: "More than one answer allowed." },
  { value: "signature", label: "Signature", hint: "Draws and types their name to sign." },
];

export interface FormField {
  key: string;
  label: string;
  type: FieldType;
  required: boolean;
  helpText?: string;
  options: string[];
}

export interface FormDefinition {
  id: string;
  title: string;
  description?: string;
  fields: FormField[];
  template: boolean;
  sortOrder: number;
  active: boolean;
  createdAt: string;
  updatedAt: string;
}

export type SubmissionStatus = "assigned" | "submitted";

export interface Answer {
  value?: string;
  values?: string[];
}

export interface FormSubmission {
  id: string;
  formId: string;
  formTitle: string;
  clientId: string;
  bookingId?: string;
  status: SubmissionStatus;
  answers: Record<string, Answer>;
  signature?: { typedName: string; signedAt: string };
  assignedAt: string;
  submittedAt?: string;
}

/**
 * One submission with the definition it was answered against.
 *
 * `integrityOk` is recomputed server-side from the answers, the typed name
 * and the timestamp. A record altered in the database after signing comes
 * back false, which is what makes this worth keeping at all.
 */
export interface SubmissionView {
  submission: FormSubmission;
  form: FormDefinition;
  integrityOk: boolean;
  signatureImage?: string;
}

export interface FormDraft {
  title: string;
  description?: string;
  fields: FormField[];
  template?: boolean;
  sortOrder?: number;
}

export type FormPatch = Partial<FormDraft & { active: boolean }>;

export const formsApi = {
  async list(session: Session, callbacks: RefreshCallbacks): Promise<FormDefinition[]> {
    const { items } = await authedRequest<{ items: FormDefinition[] }>(
      "/v1/admin/forms",
      session,
      callbacks,
    );
    return items;
  },

  async create(
    session: Session,
    callbacks: RefreshCallbacks,
    draft: FormDraft,
  ): Promise<FormDefinition> {
    const { form } = await authedRequest<{ form: FormDefinition }>(
      "/v1/admin/forms",
      session,
      callbacks,
      { method: "POST", body: draft },
    );
    return form;
  },

  async update(
    session: Session,
    callbacks: RefreshCallbacks,
    id: string,
    patch: FormPatch,
  ): Promise<FormDefinition> {
    const { form } = await authedRequest<{ form: FormDefinition }>(
      `/v1/admin/forms/${id}`,
      session,
      callbacks,
      { method: "PATCH", body: patch },
    );
    return form;
  },

  remove(session: Session, callbacks: RefreshCallbacks, id: string): Promise<void> {
    return authedRequest<void>(`/v1/admin/forms/${id}`, session, callbacks, { method: "DELETE" });
  },

  /** Gives a client a form to fill in, optionally tied to one booking. */
  async assign(
    session: Session,
    callbacks: RefreshCallbacks,
    input: { formId: string; clientId: string; bookingId?: string },
  ): Promise<FormSubmission> {
    const { submission } = await authedRequest<{ submission: FormSubmission }>(
      "/v1/admin/forms/assign",
      session,
      callbacks,
      { method: "POST", body: input },
    );
    return submission;
  },

  async listSubmissions(
    session: Session,
    callbacks: RefreshCallbacks,
    filter: { clientId?: string; formId?: string; bookingId?: string; status?: SubmissionStatus } = {},
  ): Promise<FormSubmission[]> {
    const query = new URLSearchParams();
    for (const [key, value] of Object.entries(filter)) {
      if (value) query.set(key, value);
    }
    const suffix = query.size > 0 ? `?${query}` : "";
    const { items } = await authedRequest<{ items: FormSubmission[] }>(
      `/v1/admin/forms/submissions${suffix}`,
      session,
      callbacks,
    );
    return items;
  },

  getSubmission(
    session: Session,
    callbacks: RefreshCallbacks,
    id: string,
  ): Promise<SubmissionView> {
    return authedRequest<SubmissionView>(
      `/v1/admin/forms/submissions/${id}`,
      session,
      callbacks,
    );
  },
};

/**
 * Derives a field key from its label, matching `form.NormalizeKey` exactly.
 *
 * Only spaces, hyphens and underscores become separators. Everything else
 * unrecognised is dropped without one, so "Name/Address" is `nameaddress`
 * — the server's rule, not a tidier one. The client only proposes the key;
 * the server normalizes whatever it receives, and a builder that showed a
 * different key from the one stored would be lying about where the answer
 * lives.
 *
 * The key is what an answer is stored under, so it must survive a label
 * being reworded — which is why the builder assigns it once, on creation,
 * and never recomputes it for a field that already exists.
 */
export function fieldKey(label: string): string {
  let out = "";
  let lastUnderscore = true;
  for (const character of label.toLowerCase().trim()) {
    if (/[a-z0-9]/.test(character)) {
      out += character;
      lastUnderscore = false;
    } else if (/[-_ ]/.test(character) && !lastUnderscore) {
      out += "_";
      lastUnderscore = true;
    }
  }
  return out.replace(/^_+|_+$/g, "");
}

/** A key that doesn't collide with one already in the form. */
export function uniqueFieldKey(label: string, taken: string[]): string {
  const base = fieldKey(label) || "field";
  if (!taken.includes(base)) return base;
  let n = 2;
  while (taken.includes(`${base}_${n}`)) n += 1;
  return `${base}_${n}`;
}

/**
 * What is wrong with a field, in the order the practitioner would fix it.
 *
 * This mirrors the domain's own rules rather than guessing at them: a
 * choice field with no options, and an options list on a field that has no
 * choices, are both rejected server-side.
 */
export function fieldProblem(field: FormField): string | null {
  if (!field.label.trim()) return "Every question needs a label";
  if (CHOICE_TYPES.includes(field.type) && field.options.filter((o) => o.trim()).length === 0) {
    return "Add at least one option";
  }
  return null;
}
