import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import FormsPage from "./page";

const usePortalData = vi.hoisted(() => vi.fn());
vi.mock("@/lib/use-portal-data", () => ({ usePortalData }));

describe("FormsPage", () => {
  const refresh = vi.fn();
  beforeEach(() => {
    vi.clearAllMocks();
    usePortalData.mockReturnValue({ data: [], error: null, refresh });
  });

  it("distinguishes loading, failure, and an empty inbox", () => {
    usePortalData.mockReturnValueOnce({ data: null, error: null, refresh });
    const view = render(<FormsPage />);
    expect(screen.getByRole("status")).toBeTruthy();
    usePortalData.mockReturnValueOnce({ data: null, error: "Forms are unavailable", refresh });
    view.rerender(<FormsPage />);
    fireEvent.click(screen.getByRole("button", { name: /try again/i }));
    expect(refresh).toHaveBeenCalled();
    usePortalData.mockReturnValueOnce({ data: [], error: null, refresh });
    view.rerender(<FormsPage />);
    expect(screen.getByRole("heading", { name: /no forms right now/i })).toBeTruthy();
  });

  it("separates assigned and submitted forms", () => {
    usePortalData.mockReturnValue({ data: [
      { id: "a", formTitle: "Intake", status: "assigned", assignedAt: "2026-08-01T00:00:00Z" },
      { id: "b", formTitle: "Consent", status: "submitted", assignedAt: "2026-08-01T00:00:00Z", submittedAt: "2026-08-02T00:00:00Z", signature: "signed" },
    ], error: null, refresh });
    render(<FormsPage />);
    expect(screen.getByRole("heading", { name: "To complete" })).toBeTruthy();
    expect(screen.getByRole("link", { name: "Complete it" }).getAttribute("href")).toBe("/portal/forms/a");
    expect(screen.getByRole("heading", { name: "Completed" })).toBeTruthy();
    expect(screen.getByText(/signed/i)).toBeTruthy();
  });
});
