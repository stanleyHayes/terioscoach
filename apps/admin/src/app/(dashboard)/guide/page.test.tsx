import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import UserGuidePage from "./page";

describe("Admin user guide", () => {
  it("collects the practice workflows with goals and steps", () => {
    render(<UserGuidePage />);
    expect(screen.getByRole("heading", { name: "User guide" })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "Services" })).toBeTruthy();
    expect(
      screen.getByText(/must be active and availability must contain future working hours/i),
    ).toBeTruthy();
    expect(screen.getAllByText("Good to know:").length).toBeGreaterThan(0);
  });
});
