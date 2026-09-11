import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { AgreementStep } from "./AgreementStep";
import type { Agreement } from "@/lib/agreements";

const agreement: Agreement = {
  id: "agr-1",
  key: "holistic_coaching",
  title: "Holistic Coaching Agreement",
  body: "## Terms\n\n1. The Client must be punctual.\n\nThe Coach is not a licensed nurse.",
  version: 1,
  active: true,
  updatedAt: "2026-09-11T10:00:00Z",
};

function renderStep(onSigned = vi.fn().mockResolvedValue(undefined)) {
  const onBack = vi.fn();
  render(
    <AgreementStep
      agreement={agreement}
      clientName="Daniel Baah"
      onSigned={onSigned}
      onBack={onBack}
    />,
  );
  return { onSigned, onBack };
}

describe("AgreementStep", () => {
  it("shows the whole agreement, not a link to it", () => {
    renderStep();
    expect(screen.getByText("Terms")).toBeTruthy();
    expect(screen.getByText(/must be punctual/)).toBeTruthy();
    expect(screen.getByText(/not a licensed nurse/)).toBeTruthy();
  });

  it("offers the account name but lets the client sign as they choose", async () => {
    const { onSigned } = renderStep();
    const field = screen.getByLabelText(/type your full name/i) as HTMLInputElement;
    expect(field.value).toBe("Daniel Baah");

    fireEvent.change(field, { target: { value: "Daniel K. Baah" } });
    fireEvent.click(screen.getByRole("checkbox"));
    fireEvent.click(screen.getByRole("button", { name: /sign and continue/i }));

    await waitFor(() => expect(onSigned).toHaveBeenCalledWith("Daniel K. Baah"));
  });

  // Signing is a deliberate act: a ticked box and a typed name, not one
  // button that means "I agree" to text nobody read.
  it("will not sign without the confirmation ticked", () => {
    const { onSigned } = renderStep();
    fireEvent.click(screen.getByRole("button", { name: /sign and continue/i }));
    expect(onSigned).not.toHaveBeenCalled();
  });

  it("will not sign with an empty name", () => {
    const { onSigned } = renderStep();
    fireEvent.change(screen.getByLabelText(/type your full name/i), {
      target: { value: " " },
    });
    fireEvent.click(screen.getByRole("checkbox"));
    fireEvent.click(screen.getByRole("button", { name: /sign and continue/i }));
    expect(onSigned).not.toHaveBeenCalled();
  });

  it("surfaces a failure instead of pretending the signature landed", async () => {
    const onSigned = vi.fn().mockRejectedValue(new Error("network"));
    renderStep(onSigned);
    fireEvent.click(screen.getByRole("checkbox"));
    fireEvent.click(screen.getByRole("button", { name: /sign and continue/i }));
    expect(await screen.findByRole("alert")).toBeTruthy();
  });
});
