import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Modal } from "./Modal";

/**
 * The Modal's behaviour, not its appearance.
 *
 * Everything asserted here is something a keyboard or screen-reader user
 * depends on and a sighted mouse user never notices — which is why it
 * needs a test rather than a look.
 *
 * Note on `<dialog>`: this app builds on it deliberately, for `showModal`'s
 * focus trap, Esc handling and page inertness, with every visible surface
 * restyled. `apps/admin` reimplements the same behaviour by hand. The two
 * should agree; the customer-facing one is the version with real
 * inertness, so it is the one to converge on.
 */

function open(props: Partial<React.ComponentProps<typeof Modal>> = {}) {
  const onClose = vi.fn();
  const view = render(
    <Modal open onClose={onClose} title="Cancel this session?" {...props}>
      <p>This frees the slot for someone else.</p>
      <button type="button">Inside</button>
    </Modal>,
  );
  return { ...view, onClose, dialog: view.container.querySelector("dialog")! };
}

describe("Modal", () => {
  it("is a labelled, modal dialog", () => {
    const { dialog } = open({ description: "It cannot be undone." });

    expect(dialog.getAttribute("aria-modal")).toBe("true");
    // Labelled by its own heading, so a screen reader announces what this
    // is about rather than just "dialog".
    expect(within(dialog).getByRole("heading", { name: /cancel this session/i })).toBeTruthy();
    expect(within(dialog).getByText("It cannot be undone.")).toBeTruthy();
  });

  it("opens and closes the underlying dialog rather than only hiding it", () => {
    const { rerender, dialog } = open();
    expect(dialog.hasAttribute("open")).toBe(true);

    rerender(
      <Modal open={false} onClose={vi.fn()} title="Cancel this session?">
        <p>Body</p>
      </Modal>,
    );
    // Left open, the rest of the page would stay inert and unreachable.
    expect(dialog.hasAttribute("open")).toBe(false);
  });

  it("closes on Escape, which the browser delivers as cancel", () => {
    const { onClose, dialog } = open();

    fireEvent(dialog, new Event("cancel", { bubbles: false, cancelable: true }));

    expect(onClose).toHaveBeenCalled();
  });

  it("closes on a scrim click but not on a click inside the panel", () => {
    const { onClose, dialog } = open();

    fireEvent.click(screen.getByText(/frees the slot/i));
    expect(onClose).not.toHaveBeenCalled();

    // The dialog element itself is the scrim; the panel fills the rest.
    fireEvent.click(dialog);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("closes on the close button", () => {
    const { onClose } = open();

    fireEvent.click(screen.getByRole("button", { name: /close/i }));

    expect(onClose).toHaveBeenCalled();
  });

  it("renders a footer's actions after the content", () => {
    open({
      footer: (
        <button type="button">Confirm</button>
      ),
    });

    const buttons = screen.getAllByRole("button").map((button) => button.textContent);
    // Confirm last: the destructive action is not the first thing a
    // keyboard user lands on.
    expect(buttons[buttons.length - 1]).toBe("Confirm");
  });

  it("hides the decorative drag handle from assistive technology", () => {
    const { dialog } = open();
    const handle = dialog.querySelector('[aria-hidden="true"]');
    expect(handle).not.toBeNull();
    // It exists for the bottom-sheet look and means nothing spoken aloud.
    expect(handle!.textContent).toBe("");
  });

  it("respects a reduced-motion preference in its own stylesheet", () => {
    const { dialog } = open();
    const styles = dialog.querySelector("style")!.textContent ?? "";
    expect(styles).toContain("prefers-reduced-motion");
  });
});
