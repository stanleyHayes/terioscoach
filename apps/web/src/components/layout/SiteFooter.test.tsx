import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { SiteFooter } from "./SiteFooter";

describe("SiteFooter", () => {
  it("renders the wordmark, nav columns and small print", () => {
    render(<SiteFooter />);
    expect(screen.getByRole("link", { name: "Terios Wellness" })).toBeTruthy();

    expect(screen.getByRole("navigation", { name: "Explore" })).toBeTruthy();
    expect(screen.getByRole("navigation", { name: "Practice" })).toBeTruthy();

    for (const label of ["Services", "Work with me", "Contact"]) {
      expect(screen.getByRole("link", { name: label })).toBeTruthy();
    }

    expect(
      screen.getByText(`© ${new Date().getFullYear()} Terios Wellness Spa`),
    ).toBeTruthy();
    const credit = screen.getByRole("link", { name: "DEVELOPED BY XCREATIVS TECHNOLOGIES" });
    expect(credit.getAttribute("href")).toBe("https://xcreativs.com");
    expect(credit.getAttribute("rel")).toContain("noopener");
  });
});
