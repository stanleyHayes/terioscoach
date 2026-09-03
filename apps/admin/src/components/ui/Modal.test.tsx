import { fireEvent, render, screen } from "@testing-library/react";
import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";
import { Modal } from "./Modal";

/**
 * The modal is custom chrome, not a native `<dialog>` — which means every
 * behaviour a browser would have provided is code here, and every one of
 * them can regress silently. Focus trapping and Esc handling are the two
 * that a practitioner notices only by losing work.
 */

function open(props: Partial<React.ComponentProps<typeof Modal>> = {}) {
  const onClose = vi.fn();
  const result = render(
    <Modal open onClose={onClose} title="Edit service" {...props}>
      <input aria-label="Name" />
      <button type="button">Middle</button>
      <button type="button">Last</button>
    </Modal>,
  );
  return { onClose, ...result };
}

describe("Modal", () => {
  // The focus trap only considers elements that occupy space, so it asks
  // for their client rects. jsdom does no layout and answers "none" for
  // everything, which would make the trap a no-op and these assertions
  // vacuous. Stub a box so the filter sees real candidates.
  const noLayout = Element.prototype.getClientRects;
  beforeAll(() => {
    Element.prototype.getClientRects = function getClientRects() {
      return [{ width: 10, height: 10 }] as unknown as DOMRectList;
    };
  });
  afterAll(() => {
    Element.prototype.getClientRects = noLayout;
  });

  it("renders nothing at all when closed", () => {
    const onClose = vi.fn();
    render(
      <Modal open={false} onClose={onClose} title="Edit service">
        <p>Body</p>
      </Modal>,
    );

    // Not merely hidden: a closed modal's fields must not be findable, or a
    // form submit can pick up values from a dialog nobody opened.
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.queryByText("Body")).toBeNull();
  });

  it("announces itself as a modal dialog labelled by its title", () => {
    open({ description: "Pricing is public immediately." });

    const dialog = screen.getByRole("dialog");
    expect(dialog.getAttribute("aria-modal")).toBe("true");
    const labelledBy = dialog.getAttribute("aria-labelledby");
    expect(document.getElementById(String(labelledBy))?.textContent).toBe("Edit service");
    expect(screen.getByText("Pricing is public immediately.")).toBeTruthy();
  });

  it("omits the description paragraph when there is none", () => {
    open();
    expect(screen.getByRole("dialog").querySelectorAll("p")).toHaveLength(0);
  });

  it("focuses the first focusable element on open", () => {
    open();
    // The close button is first in the panel's DOM order, so an
    // unannotated dialog opens with focus on the way out — which is the
    // right default for a dialog that only reports something.
    expect(document.activeElement).toBe(screen.getByRole("button", { name: /close/i }));
  });

  it("prefers an element marked data-autofocus", () => {
    const onClose = vi.fn();
    render(
      <Modal open onClose={onClose} title="Edit service">
        <input aria-label="First" />
        <input aria-label="Second" data-autofocus />
      </Modal>,
    );

    // Forms mark the field the practitioner actually came to change, which
    // is rarely the first one in the markup.
    expect(document.activeElement).toBe(screen.getByLabelText("Second"));
  });

  it("closes on Escape", () => {
    const { onClose } = open();
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("swallows Escape while the form is dirty", () => {
    const { onClose } = open({ dirty: true });
    fireEvent.keyDown(document, { key: "Escape" });

    // Losing half-typed session notes to a stray Esc is the exact failure
    // this exists to prevent.
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog")).toBeTruthy();
  });

  it("closes on a scrim click, but not while dirty", () => {
    const { onClose, rerender } = open();
    const scrim = document.querySelector(".bg-overlay")!;

    fireEvent.click(scrim);
    expect(onClose).toHaveBeenCalledTimes(1);

    rerender(
      <Modal open dirty onClose={onClose} title="Edit service">
        <input aria-label="Name" />
      </Modal>,
    );
    fireEvent.click(document.querySelector(".bg-overlay")!);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("closes from the close button even when dirty", () => {
    const { onClose } = open({ dirty: true });

    // Esc is ambiguous; clicking Close is not. Deliberate discard has to
    // stay possible or the practitioner is stuck in the dialog.
    fireEvent.click(screen.getByRole("button", { name: /close/i }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("wraps Tab from the last element back to the first", () => {
    open();
    screen.getByRole("button", { name: "Last" }).focus();

    fireEvent.keyDown(document, { key: "Tab" });

    // Without the trap, Tab walks out of the dialog and into the page
    // behind it, which a screen-reader user cannot see is still there.
    expect(document.activeElement).toBe(screen.getByRole("button", { name: /close/i }));
  });

  it("wraps Shift+Tab from the first element back to the last", () => {
    open();
    screen.getByRole("button", { name: /close/i }).focus();

    fireEvent.keyDown(document, { key: "Tab", shiftKey: true });

    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Last" }));
  });

  it("leaves Tab alone in the middle of the dialog", () => {
    open();
    const middle = screen.getByRole("button", { name: "Middle" });
    middle.focus();

    fireEvent.keyDown(document, { key: "Tab" });

    // The browser moves focus itself; intercepting here would double-step.
    expect(document.activeElement).toBe(middle);
  });

  it("ignores keys it has no business handling", () => {
    const { onClose } = open();
    fireEvent.keyDown(document, { key: "a" });
    fireEvent.keyDown(document, { key: "Enter" });
    expect(onClose).not.toHaveBeenCalled();
  });

  it("restores focus to whatever was focused before it opened", () => {
    const trigger = document.createElement("button");
    document.body.append(trigger);
    trigger.focus();

    const { unmount } = open();
    expect(document.activeElement).not.toBe(trigger);

    unmount();

    // Leaving focus on <body> after closing drops a keyboard user back to
    // the top of the page.
    expect(document.activeElement).toBe(trigger);
    trigger.remove();
  });

  it("renders a footer only when given one", () => {
    const { unmount } = open({ footer: <button type="button">Save</button> });
    expect(screen.getByRole("button", { name: "Save" })).toBeTruthy();
    unmount();

    open();
    expect(screen.queryByRole("button", { name: "Save" })).toBeNull();
  });

  it("widens for form content", () => {
    const { unmount } = open({ size: "form" });
    expect(screen.getByRole("dialog").className).toContain("sm:max-w-[640px]");
    unmount();

    const wide = open({ size: "wide" });
    expect(screen.getByRole("dialog").className).toContain("sm:max-w-[980px]");
    wide.unmount();

    open();
    expect(screen.getByRole("dialog").className).toContain("sm:max-w-[480px]");
  });
});
