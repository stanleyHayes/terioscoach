import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import About, { metadata } from "./page";

describe("About page", () => {
  it("sets the page title metadata", () => {
    expect(metadata).toMatchObject({ title: "About" });
  });

  it("renders the page headline", () => {
    render(<About />);
    expect(
      screen.getByRole("heading", {
        level: 1,
        name: /a practice built on calm, clinical care/i,
      }),
    ).toBeTruthy();
  });

  it("renders the practitioner story section", () => {
    render(<About />);
    const story = screen.getByRole("region", { name: /the practitioner/i });
    expect(within(story).getByText(/terios began/i)).toBeTruthy();
  });

  it("renders four numbered philosophy principles", () => {
    render(<About />);
    const philosophy = screen.getByRole("region", {
      name: /how i approach care/i,
    });
    for (const title of [
      /listen first/i,
      /clinical grounding/i,
      /small, sustainable steps/i,
      /your pace, your place/i,
    ]) {
      expect(within(philosophy).getByText(title)).toBeTruthy();
    }
  });

  it("renders the credentials strip", () => {
    render(<About />);
    const credentials = screen.getByRole("region", { name: /credentials/i });
    expect(within(credentials).getByText(/registered nurse/i)).toBeTruthy();
    expect(within(credentials).getByText(/wellness coach/i)).toBeTruthy();
  });

  it("renders the three how-sessions-work steps", () => {
    render(<About />);
    const how = screen.getByRole("region", { name: /from booking to follow-up/i });
    expect(within(how).getByText(/book a time/i)).toBeTruthy();
    expect(within(how).getByText(/meet by video/i)).toBeTruthy();
    expect(within(how).getByText(/follow up in your portal/i)).toBeTruthy();
  });

  it("ends with a CTA to /work-with-me", () => {
    render(<About />);
    const cta = screen.getByRole("region", {
      name: /care, one conversation at a time/i,
    });
    expect(
      within(cta).getByRole("link", { name: /work with me/i }).getAttribute("href"),
    ).toBe("/work-with-me");
  });
});
