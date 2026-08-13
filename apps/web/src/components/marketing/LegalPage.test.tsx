import { render, screen } from "@testing-library/react";
import { ShieldCheck } from "lucide-react";
import { describe, expect, it } from "vitest";
import { LegalPage } from "./LegalPage";

describe("LegalPage", () => {
  it("renders an accessible contents rail, policy chapters and next actions", () => {
    render(
      <LegalPage
        eyebrow="Privacy"
        title="Your information stays part of your care."
        description="A plain-language overview."
        summary="The important part."
        notice="Review before launch."
        sections={[
          { title: "What we collect", body: "Only what the practice needs.", icon: ShieldCheck },
          { title: "Your choices", body: "Ask us about your records.", icon: ShieldCheck },
        ]}
        relatedHref="/terms"
        relatedLabel="Read our terms"
      />,
    );

    const contents = screen.getByRole("navigation", { name: "Privacy contents" });
    expect(contents.textContent).toContain("What we collect");
    expect(screen.getByRole("heading", { name: "Your choices" })).toBeTruthy();
    expect(screen.getByText("12 August 2026")).toBeTruthy();
    expect(screen.getByRole("link", { name: /read our terms/i }).getAttribute("href")).toBe("/terms");
    expect(screen.getByRole("link", { name: /contact the practice/i }).getAttribute("href")).toBe("/contact");
  });
});
