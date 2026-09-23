import { useState } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ParticipantFields } from "./ParticipantFields";
import type { Participant } from "@/lib/bookings";
function Form() {
  const [value, setValue] = useState<Participant>({
    name: "Child",
    under18: false,
    accurate: false,
  });
  const [signature, setSignature] = useState("");
  return (
    <ParticipantFields
      value={value}
      onChange={setValue}
      consentBody="Versioned practice consent wording."
      signature={signature}
      onSignature={setSignature}
    />
  );
}
describe("minor participant declaration", () => {
  it("reveals guardian identity and signature fields only when selected", () => {
    render(<Form />);
    expect(screen.queryByLabelText("Guardian full name")).toBeNull();
    fireEvent.click(
      screen.getByRole("checkbox", {
        name: "This appointment is for someone under 18.",
      }),
    );
    expect(screen.getByLabelText(/Guardian full name/)).toBeTruthy();
    expect(
      screen.getByLabelText(/Parent \/ guardian electronic signature/),
    ).toBeTruthy();
    expect(
      screen.getByText("Versioned practice consent wording."),
    ).toBeTruthy();
    fireEvent.click(
      screen.getByRole("checkbox", {
        name: "This appointment is for someone under 18.",
      }),
    );
    expect(
      screen.queryByLabelText(/Parent \/ guardian electronic signature/),
    ).toBeNull();
  });
});
