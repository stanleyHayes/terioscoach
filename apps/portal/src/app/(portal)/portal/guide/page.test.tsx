import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import PortalGuidePage from "./page";

describe("Portal user guide", () => {
  it("collects the client tasks with contextual guidance", () => {
    render(<PortalGuidePage />);
    expect(screen.getByRole("heading", { name: "User guide" })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "Book a session" })).toBeTruthy();
    expect(screen.getByText(/publish both a service and future availability/i)).toBeTruthy();
  });
});
