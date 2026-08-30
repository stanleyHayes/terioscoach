import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { AppSplash } from "./AppSplash";

describe("AppSplash", () => {
  it("uses the practice label by default", () => {
    render(<AppSplash />);
    expect(screen.getByText("Practice workspace")).toBeTruthy();
  });

  it("supports contextual loading labels", () => {
    render(<AppSplash label="Preparing your consultation" />);
    expect(screen.getByText("Preparing your consultation")).toBeTruthy();
  });
});
