import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Section } from "./Section";

describe("Section", () => {
  it("uses the light surface by default", () => {
    render(<Section>Default surface</Section>);
    expect(screen.getByText("Default surface").parentElement?.className).toContain("bg-surface");
  });

  it("uses the explicit night surface without retaining the light background", () => {
    render(<Section background="night">Night surface</Section>);
    const section = screen.getByText("Night surface").parentElement;
    expect(section?.className).toContain("bg-eucalyptus-900");
    expect(section?.className.split(" ")).not.toContain("bg-surface");
  });
});
