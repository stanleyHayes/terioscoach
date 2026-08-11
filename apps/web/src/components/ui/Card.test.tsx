import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Card } from "./Card";

describe("Card", () => {
  it("renders children with the default surface/border/radius treatment", () => {
    render(<Card>Body copy</Card>);
    const card = screen.getByText("Body copy");
    expect(card.className).toContain("rounded-lg");
    expect(card.className).toContain("border-border");
    expect(card.className).toContain("bg-surface-raised");
    expect(card.className).not.toContain("hover:-translate-y-0.5");
  });

  it("adds the hover treatment when hoverable", () => {
    render(<Card hoverable>Clickable</Card>);
    const card = screen.getByText("Clickable");
    expect(card.className).toContain("hover:-translate-y-0.5");
    expect(card.className).toContain("hover:border-border-strong");
    expect(card.className).toContain("hover:shadow-sm");
  });

  it("merges a custom className", () => {
    render(<Card className="p-8">Spacious</Card>);
    expect(screen.getByText("Spacious").className).toContain("p-8");
  });
});
