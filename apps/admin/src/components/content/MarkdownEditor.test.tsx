import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { MarkdownEditor } from "./MarkdownEditor";

function Desk({ initial = "" }: { initial?: string }) {
  const [value, setValue] = useState(initial);
  return <MarkdownEditor value={value} onChange={setValue} />;
}
describe("MarkdownEditor", () => {
  it("preserves source when switching modes and renders GFM preview", async () => {
    const source =
      "## Care\n\n- Rest\n- Hydrate\n\n~~Old~~\n\n| A | B |\n| --- | --- |\n| One | Two |";
    render(<Desk initial={source} />);
    await screen.findByRole("textbox", { name: "Body" });
    fireEvent.click(screen.getByRole("tab", { name: "Markdown" }));
    expect(
      (screen.getByRole("textbox", { name: /^Body/ }) as HTMLTextAreaElement)
        .value,
    ).toBe(source);
    fireEvent.click(screen.getByRole("tab", { name: "Preview" }));
    const preview = screen.getByRole("article", { name: "Body preview" });
    expect(within(preview).getByRole("heading", { name: "Care" })).toBeTruthy();
    expect(within(preview).getByRole("table")).toBeTruthy();
    expect(preview.querySelector("del")?.textContent).toBe("Old");
  });
  it("loads Markdown edits back into rich text", async () => {
    render(<Desk />);
    fireEvent.click(screen.getByRole("tab", { name: "Markdown" }));
    fireEvent.change(screen.getByRole("textbox", { name: /^Body/ }), {
      target: { value: "**Care**" },
    });
    fireEvent.click(screen.getByRole("tab", { name: "Rich text" }));
    await waitFor(() =>
      expect(
        screen.getByRole("textbox", { name: "Body" }).querySelector("strong")
          ?.textContent,
      ).toBe("Care"),
    );
  });
  it("shows errors and an empty preview", () => {
    render(
      <MarkdownEditor value="" onChange={vi.fn()} error="Body is required" />,
    );
    expect(screen.getByRole("alert").textContent).toBe("Body is required");
    fireEvent.click(screen.getByRole("tab", { name: "Preview" }));
    expect(screen.getByText("Nothing to preview yet.")).toBeTruthy();
  });
});
