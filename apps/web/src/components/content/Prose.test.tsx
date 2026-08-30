import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Prose } from "./Prose";

describe("Prose", () => {
  it("renders Markdown structure without enabling raw HTML", () => {
    const { container } = render(<Prose body={'## A calmer week\n\n- Rest\n- Breathe\n\n**Begin gently.**\n\n<script>alert("x")</script>'} />);
    expect(screen.getByRole("heading", { name: "A calmer week" })).toBeTruthy();
    expect(screen.getByRole("list").children).toHaveLength(2);
    expect(screen.getByText("Begin gently.").tagName).toBe("STRONG");
    expect(container.querySelector("script")).toBeNull();
    expect(screen.getByText(/<script>/)).toBeTruthy();
  });

  it("renders nothing for an empty body", () => {
    const { container } = render(<Prose body="   " />);
    expect(container.innerHTML).toBe("");
  });
});
