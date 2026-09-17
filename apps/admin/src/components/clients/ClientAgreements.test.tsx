import { render, screen, within } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { ClientAgreements } from "./ClientAgreements";

const { signaturesForClient, auth } = vi.hoisted(() => ({
  signaturesForClient: vi.fn(),
  auth: {
    session: { accessToken: "token", refreshToken: "refresh" },
    user: { name: "Practitioner" },
    refreshCallbacks: {},
  },
}));
vi.mock("@/lib/auth", () => ({ useAuth: () => auth }));
vi.mock("@/lib/agreements", () => ({
  agreementsApi: { signaturesForClient },
  signedAgreementPath: (id: string) => `/signatures/${id}/pdf`,
}));

it("offers countersigning only for the two coaching agreements in a complete document bundle", async () => {
  const documents = [
    ["Holistic Coaching Agreement", true],
    ["Nurse Coaching Agreement", true],
    ["Holistic Statement of Work", false],
    ["Nurse Statement of Work", false],
    ["Holistic Confidentiality and HIPAA Consent", false],
    ["Nurse Confidentiality and HIPAA Consent", false],
    ["Holistic Liability Release", false],
    ["Nurse Liability Release", false],
  ] as const;
  signaturesForClient.mockResolvedValue(documents.map(([title, required], index) => ({
    id: `sig-${index}`, agreementId: `agreement-${index}`,
    agreementTitle: title, agreementVersion: 1,
    clientId: "client-1", clientName: "Daniel Baah", signedName: "Daniel Baah",
    signedAt: "2026-09-17T10:00:00Z", requiresCountersignature: required,
  })));
  render(<ClientAgreements clientId="client-1" />);
  await screen.findByText("Holistic Coaching Agreement");
  expect(screen.getAllByRole("button", { name: "Countersign" })).toHaveLength(2);
  for (const [title, required] of documents) {
    const row = within(screen.getByText(title).closest("li")!);
    expect(Boolean(row.queryByRole("button", { name: "Countersign" }))).toBe(required);
    expect(row.getByText(required ? "Countersignature pending" : "Signed")).toBeTruthy();
    expect(row.getByText("Daniel Baah")).toBeTruthy();
    expect(row.getByRole("button", { name: "Open PDF" })).toBeTruthy();
  }
});
