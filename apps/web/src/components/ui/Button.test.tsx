import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Button } from "./Button";

describe("Button", () => {
  it("renders the primary variant by default", () => {
    render(<Button>Book a session</Button>);
    const button = screen.getByRole("button", { name: "Book a session" });
    expect(button.className).toContain("bg-primary");
    expect(button.className).toContain("text-on-primary");
  });

  it("renders the secondary variant", () => {
    render(<Button variant="secondary">Save notes</Button>);
    const button = screen.getByRole("button", { name: "Save notes" });
    expect(button.className).toContain("border-border-strong");
    expect(button.className).toContain("text-ink");
  });

  it("renders the ghost variant", () => {
    render(<Button variant="ghost">Sign in</Button>);
    const button = screen.getByRole("button", { name: "Sign in" });
    expect(button.className).toContain("text-primary");
    expect(button.className).toContain("bg-transparent");
  });

  it("renders the danger variant", () => {
    render(<Button variant="danger">Cancel session</Button>);
    const button = screen.getByRole("button", { name: "Cancel session" });
    expect(button.className).toContain("bg-danger");
  });

  it("applies size classes", () => {
    render(
      <>
        <Button size="sm">Small</Button>
        <Button size="lg">Large</Button>
      </>,
    );
    expect(screen.getByRole("button", { name: "Small" }).className).toContain("h-8");
    expect(screen.getByRole("button", { name: "Large" }).className).toContain("h-12");
  });

  it("is disabled when disabled", () => {
    render(<Button disabled>Book</Button>);
    expect(screen.getByRole("button", { name: "Book" }).hasAttribute("disabled")).toBe(
      true,
    );
  });

  it("shows a spinner, locks the label and sets aria-busy when loading", () => {
    const { container } = render(<Button loading>Book a session</Button>);
    const button = screen.getByRole("button", { name: "Book a session" });
    expect(button.hasAttribute("disabled")).toBe(true);
    expect(button.getAttribute("aria-busy")).toBe("true");
    // Visible label hidden from AT, duplicate kept for screen readers.
    expect(button.querySelector("span.sr-only")?.textContent).toBe("Book a session");
    // Spinner is present and hidden from AT.
    const spinner = container.querySelector("svg.animate-spin");
    expect(spinner).not.toBeNull();
    expect(spinner?.getAttribute("aria-hidden")).toBe("true");
  });

  it("activates on click when enabled", () => {
    let clicked = 0;
    render(<Button onClick={() => (clicked += 1)}>Book</Button>);
    fireEvent.click(screen.getByRole("button", { name: "Book" }));
    expect(clicked).toBe(1);
  });
});
