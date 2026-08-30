import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { MarkdownEditor } from "./MarkdownEditor";

describe("MarkdownEditor", () => {
  it("edits Markdown with the toolbar and renders a preview", () => {
    let value = "hello";
    const onChange = vi.fn((next: string) => { value = next; });
    const { rerender } = render(<MarkdownEditor value={value} onChange={onChange} />);
    const body = screen.getByRole("textbox", { name: /body/i }) as HTMLTextAreaElement;
    body.setSelectionRange(0, 5);
    fireEvent.click(screen.getByRole("button", { name: "Bold" }));
    expect(onChange).toHaveBeenCalledWith("**hello**");

    const markdown = ["## Care", "", "- Rest", "- Hydrate"].join("\n");
    rerender(<MarkdownEditor value={markdown} onChange={onChange} />);
    fireEvent.click(screen.getByRole("tab", { name: /preview/i }));
    expect(screen.getByRole("heading", { name: "Care" })).toBeTruthy();
    expect(screen.getByText("Hydrate")).toBeTruthy();
  });

  it("inserts fallback text, announces errors and supports undo and redo", () => {
    const onChange = vi.fn();
    const exec = vi.fn();
    Object.defineProperty(document, "execCommand", { value: exec, configurable: true });
    render(<MarkdownEditor value="" onChange={onChange} error="Body is required" />);
    fireEvent.click(screen.getByRole("button", { name: "Link" }));
    expect(onChange).toHaveBeenCalledWith("[link text](https://)");
    fireEvent.click(screen.getByRole("button", { name: "Undo" }));
    fireEvent.click(screen.getByRole("button", { name: "Redo" }));
    expect(exec).toHaveBeenNthCalledWith(1, "undo");
    expect(exec).toHaveBeenNthCalledWith(2, "redo");
    expect(screen.getByRole("alert").textContent).toContain("Body is required");
  });

  it("shows a useful empty preview", () => {
    render(<MarkdownEditor value="   " onChange={vi.fn()} />);
    fireEvent.click(screen.getByRole("tab", { name: /preview/i }));
    expect(screen.getByText("Nothing to preview yet.")).toBeTruthy();
  });
});
