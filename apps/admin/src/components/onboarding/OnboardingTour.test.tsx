import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { OnboardingTour } from "./OnboardingTour";

describe("OnboardingTour", () => {
  beforeEach(() => localStorage.clear());

  it("walks a first-time owner through every step and remembers completion", () => {
    render(<OnboardingTour />);
    expect(screen.getByRole("heading", { name: "Welcome to your practice" })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Next" }));
    expect(screen.getByRole("heading", { name: "Run your schedule" })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    fireEvent.click(screen.getByRole("button", { name: "Next" }));
    fireEvent.click(screen.getByRole("button", { name: "Next" }));
    fireEvent.click(screen.getByRole("button", { name: "Next" }));
    expect(screen.getByRole("heading", { name: "Know what needs attention" })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Start working" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(localStorage.getItem("terios.admin.onboarding.complete")).toBe("true");
  });

  it("stays hidden for returning users and can be restarted", () => {
    localStorage.setItem("terios.admin.onboarding.complete", "true");
    render(<OnboardingTour />);
    expect(screen.queryByRole("dialog")).toBeNull();
    act(() => window.dispatchEvent(new Event("terios:onboarding")));
    expect(screen.getByRole("dialog")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Skip tour" }));
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});
