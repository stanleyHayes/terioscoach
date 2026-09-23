import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { StatementOfWorkStep } from "./StatementOfWorkStep";
import type { Agreement } from "@/lib/agreements";
const agreement: Agreement = {
  id: "sow",
  key: "holistic_sow",
  title: "Statement of Work",
  body: "Terms to review.",
  version: 2,
  active: true,
  updatedAt: "2026-09-23T00:00:00Z",
};
describe("signed Statement of Work", () => {
  it.each(["holistic_sow", "nurse_sow"])(
    "submits %s answers with acknowledged signature and immutable catalog fee",
    async (key) => {
      const submit = vi.fn().mockResolvedValue(undefined);
      render(
        <StatementOfWorkStep
          agreement={{ ...agreement, key }}
          clientName="Participant"
          signerName="Guardian"
          fee="USD 250.00"
          onSubmitted={submit}
          onBack={() => {}}
        />,
      );
      expect(
        screen.getByLabelText(/Monthly fee/).hasAttribute("readonly"),
      ).toBe(true);
      fireEvent.change(
        screen.getByLabelText(
          key === "nurse_sow" ? /Service start date/ : /Effective date/,
        ),
        { target: { value: "2026-10-01" } },
      );
      fireEvent.change(
        screen.getByLabelText(
          key === "nurse_sow" ? /^Package/ : /Initial term/,
        ),
        { target: { value: key === "nurse_sow" ? "Three sessions" : "3" } },
      );
      fireEvent.click(
        screen.getByRole("button", { name: "Sign and continue" }),
      );
      expect(submit).not.toHaveBeenCalled();
      fireEvent.click(
        screen.getByRole("checkbox", { name: /I have reviewed these answers/ }),
      );
      fireEvent.click(
        screen.getByRole("button", { name: "Sign and continue" }),
      );
      await waitFor(() =>
        expect(submit).toHaveBeenCalledWith(
          expect.objectContaining({
            clientName: "Participant",
            monthlyFee: "USD 250.00",
            effectiveDate: "2026-10-01",
            ...(key === "nurse_sow"
              ? { package: "Three sessions" }
              : { initialTermMonths: 3 }),
          }),
          "Guardian",
        ),
      );
    },
  );
});
