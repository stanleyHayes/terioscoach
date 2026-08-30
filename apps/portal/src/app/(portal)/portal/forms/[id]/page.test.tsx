import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { SubmissionView } from "@/lib/portal";
import FormPage, { validate } from "./page";

const get = vi.hoisted(() => vi.fn());
const submit = vi.hoisted(() => vi.fn());

vi.mock("@/lib/portal", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/portal")>();
  return { ...original, formsApi: { get, submit, listMine: vi.fn() } };
});

vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "submission-1" }),
  useRouter: () => ({ refresh: vi.fn(), push: vi.fn() }),
}));

vi.mock("@/lib/auth", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/auth")>();
  return {
    ...original,
    useAuth: () => ({
      status: "authenticated",
      user: { id: "u1", email: "ama@example.com", role: "client", name: "Ama Serwaa" },
      accessToken: "a1",
      session: { accessToken: "a1", accessTokenExpiresAt: "2099-01-01T00:00:00Z", refreshToken: "r1" },
      onTokensRefreshed: vi.fn(),
      login: vi.fn(),
      register: vi.fn(),
      logout: vi.fn(),
    }),
  };
});

function view(overrides: Partial<SubmissionView> = {}): SubmissionView {
  return {
    submission: {
      id: "submission-1",
      formId: "form-1",
      formTitle: "Intake and consent",
      status: "assigned",
      answers: {},
      assignedAt: "2026-08-10T09:00:00Z",
    },
    form: {
      id: "form-1",
      title: "Intake and consent",
      description: "Please complete before your first session.",
      fields: [
        { key: "full_name", label: "Full name", type: "text", required: true, options: [] },
        { key: "age", label: "Age", type: "number", required: false, options: [] },
        { key: "consent", label: "Signature", type: "signature", required: true, options: [] },
      ],
    },
    integrityOk: true,
    ...overrides,
  };
}

describe("FormPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    get.mockResolvedValue(view());
    submit.mockResolvedValue(view().submission);
  });

  it("renders the definition that was sent, not a guess at it", async () => {
    render(<FormPage />);

    expect(await screen.findByRole("textbox", { name: /^full name/i })).toBeTruthy();
    expect(screen.getByText(/please complete before your first session/i)).toBeTruthy();
  });

  it("asks for a signature when the form has a signature field", async () => {
    render(<FormPage />);

    expect(await screen.findByLabelText(/type your full name to sign/i)).toBeTruthy();
    expect(screen.getByRole("img", { name: /draw your signature/i })).toBeTruthy();
  });

  it("catches a blank required field before any round trip", async () => {
    render(<FormPage />);
    await screen.findByRole("textbox", { name: /^full name/i });

    fireEvent.click(screen.getByRole("button", { name: /send to your practitioner/i }));

    expect(await screen.findByText(/this one is needed/i)).toBeTruthy();
    expect(submit).not.toHaveBeenCalled();
  });

  it("will not send an unsigned consent form", async () => {
    render(<FormPage />);
    fireEvent.change(await screen.findByRole("textbox", { name: /^full name/i }), {
      target: { value: "Ama Serwaa" },
    });

    fireEvent.click(screen.getByRole("button", { name: /send to your practitioner/i }));

    expect(await screen.findByText(/please type your name to sign/i)).toBeTruthy();
    expect(submit).not.toHaveBeenCalled();
  });

  it("shows a submitted form as a record rather than an editable form", async () => {
    get.mockResolvedValue(
      view({
        submission: {
          ...view().submission,
          status: "submitted",
          answers: { full_name: { value: "Ama Serwaa" } },
          signature: { typedName: "Ama Serwaa", signedAt: "2026-08-11T09:00:00Z" },
          submittedAt: "2026-08-11T09:00:00Z",
        },
        signatureImage: "data:image/png;base64,iVBORw0KGgo=",
      }),
    );

    render(<FormPage />);

    expect(await screen.findByText(/cannot be changed/i)).toBeTruthy();
    // No submit button at all: the API would refuse a second submit, and a
    // button that looks live but is not is worse than none.
    expect(screen.queryByRole("button", { name: /send to your practitioner/i })).toBeNull();
    expect(screen.getByRole("img", { name: /your signature/i })).toBeTruthy();
    expect(screen.getByRole("textbox", { name: /^full name/i })).toHaveProperty("disabled", true);
  });

  it("surfaces a rejection from the server", async () => {
    const { ApiError } = await import("@/lib/api");
    get.mockResolvedValue(
      view({
        form: {
          ...view().form,
          fields: [{ key: "full_name", label: "Full name", type: "text", required: true, options: [] }],
        },
      }),
    );
    submit.mockRejectedValueOnce(
      new ApiError(409, "already_submitted", "This form has already been submitted."),
    );

    render(<FormPage />);
    fireEvent.change(await screen.findByRole("textbox", { name: /^full name/i }), { target: { value: "Ama" } });
    fireEvent.click(screen.getByRole("button", { name: /send to your practitioner/i }));

    expect((await screen.findByRole("alert")).textContent).toContain(
      "This form has already been submitted.",
    );
  });

  it("confirms plainly once the form is sent", async () => {
    get.mockResolvedValue(
      view({
        form: {
          ...view().form,
          fields: [{ key: "full_name", label: "Full name", type: "text", required: true, options: [] }],
        },
      }),
    );

    render(<FormPage />);
    fireEvent.change(await screen.findByRole("textbox", { name: /^full name/i }), { target: { value: "Ama" } });
    fireEvent.click(screen.getByRole("button", { name: /send to your practitioner/i }));

    await waitFor(() => {
      expect(submit).toHaveBeenCalled();
    });
    expect(await screen.findByRole("heading", { name: /your form has been sent/i })).toBeTruthy();
  });

  it("offers a retry when the form will not load", async () => {
    const { ApiError } = await import("@/lib/api");
    get.mockRejectedValue(new ApiError(0, "network_error", "offline"));

    render(<FormPage />);

    expect(await screen.findByRole("button", { name: /try again/i })).toBeTruthy();
  });
});

describe("validate", () => {
  const base = view({
    form: {
      id: "form-1",
      title: "T",
      fields: [
        { key: "name", label: "Name", type: "text", required: true, options: [] },
        { key: "age", label: "Age", type: "number", required: false, options: [] },
        { key: "seen", label: "Last seen", type: "date", required: false, options: [] },
        {
          key: "pressure",
          label: "Pressure",
          type: "select",
          required: false,
          options: ["Light", "Firm"],
        },
      ],
    },
  });

  it("requires what the definition marks required", () => {
    expect(validate(base, {})).toEqual({ name: "This one is needed." });
  });

  it("treats whitespace as blank", () => {
    expect(validate(base, { name: { value: "   " } })).toEqual({
      name: "This one is needed.",
    });
  });

  it("checks numbers, dates and choices against the definition", () => {
    const errors = validate(base, {
      name: { value: "Ama" },
      age: { value: "thirty" },
      seen: { value: "11/08/2026" },
      pressure: { value: "Bone-crushing" },
    });

    expect(errors.age).toMatch(/number/i);
    expect(errors.seen).toMatch(/YYYY-MM-DD/);
    expect(errors.pressure).toMatch(/one of the options/i);
  });

  it("passes a complete, valid form", () => {
    expect(
      validate(base, {
        name: { value: "Ama" },
        age: { value: "34" },
        seen: { value: "2026-08-11" },
        pressure: { value: "Firm" },
      }),
    ).toEqual({});
  });

  it("never validates the signature field as an answer", () => {
    // The signature travels separately; treating it as an answer would
    // make a signed form fail its own required check.
    expect(validate(view(), { full_name: { value: "Ama" } })).toEqual({});
  });
});
