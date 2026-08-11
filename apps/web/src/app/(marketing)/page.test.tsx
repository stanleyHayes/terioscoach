import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import Home from "./page";

describe("Home page", () => {
  it("renders the hero headline and lead", () => {
    render(<Home />);
    expect(
      screen.getByRole("heading", {
        level: 1,
        name: /clinical calm, wherever you are/i,
      }),
    ).toBeTruthy();
  });

  it("links hero CTAs to the right routes", () => {
    render(<Home />);
    const bookLinks = screen.getAllByRole("link", { name: /book a session/i });
    expect(bookLinks.length).toBeGreaterThanOrEqual(1);
    for (const link of bookLinks) {
      expect(link.getAttribute("href")).toBe("/work-with-me");
    }
    expect(
      screen.getByRole("link", { name: /explore services/i }).getAttribute("href"),
    ).toBe("/services");
  });

  it("renders the trust strip proof points", () => {
    render(<Home />);
    const trust = screen.getByRole("region", {
      name: /why clients trust terios/i,
    });
    for (const label of [
      /sessions by video/i,
      /clients worldwide/i,
      /secure client portal/i,
    ]) {
      expect(within(trust).getByText(label)).toBeTruthy();
    }
  });

  it("renders the services preview with cards linking to /services", () => {
    render(<Home />);
    const services = screen.getByRole("region", {
      name: /care that fits the season you are in/i,
    });
    expect(within(services).getByText(/wellness coaching/i)).toBeTruthy();
    expect(within(services).getByText(/nursing consultations/i)).toBeTruthy();
    const cardLinks = within(services).getAllByRole("link");
    expect(cardLinks).toHaveLength(3);
    for (const link of cardLinks) {
      expect(link.getAttribute("href")).toBe("/services");
    }
  });

  it("links the approach teaser to /about", () => {
    render(<Home />);
    expect(
      screen.getByRole("link", { name: /about the practice/i }).getAttribute("href"),
    ).toBe("/about");
  });

  it("renders the testimonial placeholders", () => {
    render(<Home />);
    expect(screen.getAllByRole("figure")).toHaveLength(2);
  });

  it("ends with a closing CTA to /work-with-me", () => {
    render(<Home />);
    const cta = screen.getByRole("region", { name: /begin where you are/i });
    expect(
      within(cta).getByRole("link", { name: /book a session/i }).getAttribute("href"),
    ).toBe("/work-with-me");
  });
});
