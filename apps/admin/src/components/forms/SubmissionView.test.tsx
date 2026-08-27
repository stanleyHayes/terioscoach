import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { SubmissionView as View } from "@/lib/forms";
import { SubmissionModal } from "./SubmissionView";

/**
 * A signed submission is a consent record, so what this screen says about
 * it is the practitioner's only evidence that it still means what it meant
 * when it was signed.
 */

function view(overrides: Partial<View> = {}): View {
  return {
    submission: {
      id: "sub-1",
      formId: "form-1",
      formTitle: "Intake and consent",
      clientId: "client-1",
      status: "submitted",
      answers: {
        full_name: { value: "Ama Serwaa" },
        conditions: { values: ["asthma", "eczema"] },
        started: { value: "2026-03-02" },
        optional: { value: "   " },
      },
      assignedAt: "2026-08-01T09:00:00.000Z",
      submittedAt: "2026-08-02T10:30:00.000Z",
      signature: { typedName: "Ama Serwaa", signedAt: "2026-08-02T10:30:00.000Z" },
    },
    form: {
      id: "form-1",
      title: "Intake and consent",
      fields: [
        { key: "full_name", label: "Full name", type: "text", required: true, options: [] },
        { key: "conditions", label: "Conditions", type: "checkbox", required: false, options: [] },
        { key: "started", label: "Symptoms began", type: "date", required: false, options: [] },
        { key: "optional", label: "Anything else", type: "textarea", required: false, options: [] },
        { key: "missing", label: "Never answered", type: "text", required: false, options: [] },
        { key: "sig", label: "Signature", type: "signature", required: true, options: [] },
      ],
    },
    integrityOk: true,
    ...overrides,
  } as View;
}

describe("SubmissionModal", () => {
  it("vouches for a record that still matches its signature", () => {
    render(<SubmissionModal view={view()} onClose={vi.fn()} />);

    expect(screen.getByText(/nothing has been altered since it was signed/i)).toBeTruthy();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("raises an alert when the record no longer matches its signature", () => {
    render(<SubmissionModal view={view({ integrityOk: false })} onClose={vi.fn()} />);

    // This is the one state on the screen that must be impossible to miss:
    // the row was altered in the database after signing, so the consent it
    // displays is not the consent that was given.
    const alert = screen.getByRole("alert");
    expect(alert.textContent).toMatch(/does not match its signature/i);
    expect(alert.textContent).toMatch(/should not be relied on/i);
  });

  it("says who it is waiting on when the form is unfilled", () => {
    render(
      <SubmissionModal
        view={view({
          submission: { ...view().submission, status: "assigned", submittedAt: undefined, signature: undefined },
        })}
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText(/waiting on the client/i)).toBeTruthy();
    // No integrity verdict either way: there is nothing signed to verify,
    // and a green tick here would be a claim about nothing.
    expect(screen.queryByText(/nothing has been altered/i)).toBeNull();
    expect(screen.getByText(/not filled in yet/i)).toBeTruthy();
  });

  it("renders each kind of answer in its own terms", () => {
    render(<SubmissionModal view={view()} onClose={vi.fn()} />);

    expect(screen.getByText("Ama Serwaa", { selector: "dd" })).toBeTruthy();
    expect(screen.getByText("asthma, eczema")).toBeTruthy();
    expect(screen.getByText("2 March 2026")).toBeTruthy();
  });

  it('reads an unanswered question as "Not answered", never as blank', () => {
    render(<SubmissionModal view={view()} onClose={vi.fn()} />);

    // Blank is indistinguishable from a rendering failure, and on a consent
    // form the difference matters.
    expect(screen.getAllByText("Not answered")).toHaveLength(2);
  });

  it("leaves a malformed date exactly as stored", () => {
    render(
      <SubmissionModal
        view={view({
          submission: {
            ...view().submission,
            answers: { ...view().submission.answers, started: { value: "not-a-date" } },
          },
        })}
        onClose={vi.fn()}
      />,
    );

    // Showing "Invalid Date" would hide what the client actually submitted.
    expect(screen.getByText("not-a-date")).toBeTruthy();
  });

  it("does not list the signature field among the answers", () => {
    render(<SubmissionModal view={view()} onClose={vi.fn()} />);

    // The signature has its own block below; repeating it as an empty
    // answer row reads as a missing signature.
    const labels = [...document.querySelectorAll("dt")].map((dt) => dt.textContent);
    expect(labels).not.toContain("Signature");
    expect(labels).toContain("Full name");
  });

  it("shows the drawn signature when one was captured", () => {
    render(
      <SubmissionModal
        view={view({ signatureImage: "data:image/png;base64,iVBORw0KGgo=" } as Partial<View>)}
        onClose={vi.fn()}
      />,
    );

    const image = screen.getByRole("img", { name: /signature of ama serwaa/i });
    expect(image.getAttribute("src")).toMatch(/^data:image\/png/);
    expect(screen.getByText("Ama Serwaa", { selector: "p" })).toBeTruthy();
  });

  it("still shows the typed name when there is no drawn image", () => {
    render(<SubmissionModal view={view()} onClose={vi.fn()} />);

    expect(screen.queryByRole("img")).toBeNull();
    expect(screen.getByText("Ama Serwaa", { selector: "p" })).toBeTruthy();
  });

  it("marks a submission that belongs to a session", () => {
    const { unmount } = render(<SubmissionModal view={view()} onClose={vi.fn()} />);
    expect(screen.queryByText(/tied to a session/i)).toBeNull();
    unmount();

    render(
      <SubmissionModal
        view={view({ submission: { ...view().submission, bookingId: "bk-1" } })}
        onClose={vi.fn()}
      />,
    );
    expect(screen.getByText(/tied to a session/i)).toBeTruthy();
  });

  it("offers no way to change an answer", () => {
    render(<SubmissionModal view={view()} onClose={vi.fn()} />);

    // A consent record that can be edited after signing is not a consent
    // record; the only controls here close the dialog.
    expect(screen.queryByRole("textbox")).toBeNull();
    expect(screen.queryByRole("checkbox")).toBeNull();
    // Both the dialog's icon control and the footer button are named
    // "Close"; nothing else is a control at all.
    expect(screen.getAllByRole("button", { name: /close/i })).toHaveLength(
      screen.getAllByRole("button").length,
    );
  });

  it("closes from the footer", () => {
    const onClose = vi.fn();
    render(<SubmissionModal view={view()} onClose={onClose} />);

    // The footer button, not the dialog's own icon control — both close,
    // and this asserts the one the practitioner is pointed at.
    const footerClose = screen
      .getAllByRole("button", { name: /close/i })
      .find((button) => button.textContent?.trim() === "Close")!;
    fireEvent.click(footerClose);
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
