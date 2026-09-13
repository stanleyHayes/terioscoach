import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { TimePicker } from "./TimePicker";

afterEach(() => vi.restoreAllMocks());

function position(top: number, left = 100) {
  return { top, bottom: top + 40, left, right: left + 160, width: 160, height: 40, x: left, y: top, toJSON: () => ({}) };
}

describe("TimePicker", () => {
  it("escapes clipped cards, opens upward near the bottom, and selects the last time", () => {
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue(position(648));
    const onChange = vi.fn();
    const { container } = render(<div style={{ overflow: "hidden", height: 60 }}><TimePicker label="End" value="17:00" onChange={onChange} /></div>);
    const trigger = screen.getByRole("button", { name: "End" });
    fireEvent.click(trigger);
    const menu = screen.getByRole("listbox");
    expect(menu.parentElement).toBe(document.body);
    expect(container.contains(menu)).toBe(false);
    expect(parseFloat(menu.style.top)).toBeLessThan(648);
    expect(screen.getAllByRole("option")).toHaveLength(96);
    expect(document.activeElement).toBe(screen.getByRole("option", { name: "5:00 PM" }));
    fireEvent.keyDown(document.activeElement!, { key: "End" });
    const last = screen.getByRole("option", { name: "11:45 PM" });
    expect(document.activeElement).toBe(last);
    fireEvent.pointerDown(last);
    fireEvent.click(last);
    expect(onChange).toHaveBeenCalledWith("23:45");
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(document.activeElement).toBe(trigger);
  });

  it("tracks scrolling and dismisses on Escape or an outside pointer", () => {
    let top = 100;
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(() => position(top));
    render(<TimePicker label="Start" value="09:00" onChange={vi.fn()} />);
    const trigger = screen.getByRole("button", { name: "Start" });
    fireEvent.keyDown(trigger, { key: "ArrowDown" });
    const menu = screen.getByRole("listbox");
    expect(menu.style.top).toBe("148px");
    top = 150;
    fireEvent.scroll(window);
    expect(menu.style.top).toBe("198px");
    fireEvent.keyDown(menu, { key: "Escape" });
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(document.activeElement).toBe(trigger);
    fireEvent.click(trigger);
    fireEvent.pointerDown(document.body);
    expect(screen.queryByRole("listbox")).toBeNull();
  });
});
