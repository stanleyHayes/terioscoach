import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { FormField as Field } from "@/lib/portal";
import { FormField } from "./FormField";

function field(overrides: Partial<Field> = {}): Field {
  return {
    key: "full_name",
    label: "Full name",
    type: "text",
    required: false,
    options: [],
    ...overrides,
  };
}

describe("FormField", () => {
  it("uses no native form controls anywhere", () => {
    // The platform rule: every interactive element is built, not borrowed.
    const { container } = render(
      <>
        <FormField field={field({ type: "select", key: "a", options: ["One", "Two"] })} answer={{}} onChange={vi.fn()} />
        <FormField field={field({ type: "checkbox", key: "b", options: ["X", "Y"] })} answer={{}} onChange={vi.fn()} />
        <FormField field={field({ type: "radio", key: "c", options: ["P", "Q"] })} answer={{}} onChange={vi.fn()} />
      </>,
    );

    expect(container.querySelector("select")).toBeNull();
    expect(container.querySelector('input[type="checkbox"]')).toBeNull();
    expect(container.querySelector('input[type="radio"]')).toBeNull();
    expect(container.querySelector('input[type="date"]')).toBeNull();
  });

  it("renders a text field and reports what was typed", () => {
    const onChange = vi.fn();
    render(<FormField field={field()} answer={{}} onChange={onChange} />);

    fireEvent.change(screen.getByLabelText(/full name/i), { target: { value: "Ama" } });

    expect(onChange).toHaveBeenCalledWith({ value: "Ama" });
  });

  it("marks a required field for both sighted and screen-reader users", () => {
    render(<FormField field={field({ required: true })} answer={{}} onChange={vi.fn()} />);

    expect(screen.getByText("(required)")).toBeTruthy();
  });

  it("renders a short choice list as radio options", () => {
    const onChange = vi.fn();
    render(
      <FormField
        field={field({ key: "pressure", label: "Pressure", type: "select", options: ["Light", "Firm"] })}
        answer={{}}
        onChange={onChange}
      />,
    );

    const options = screen.getAllByRole("radio");
    expect(options).toHaveLength(2);

    fireEvent.click(screen.getByRole("radio", { name: "Firm" }));
    expect(onChange).toHaveBeenCalledWith({ value: "Firm" });
  });

  it("renders a long choice list as a custom listbox", () => {
    const onChange = vi.fn();
    render(
      <FormField
        field={field({
          key: "reason",
          label: "Reason",
          type: "select",
          options: ["A", "B", "C", "D", "E", "F"],
        })}
        answer={{}}
        onChange={onChange}
      />,
    );

    // Five cards would be worse than a menu — but it is still not a
    // native select.
    const trigger = screen.getByRole("button", { name: /reason/i });
    expect(trigger.getAttribute("aria-haspopup")).toBe("listbox");
    expect(screen.queryByRole("listbox")).toBeNull();

    fireEvent.click(trigger);
    expect(screen.getByRole("listbox")).toBeTruthy();

    fireEvent.click(screen.getByRole("option", { name: "C" }));
    expect(onChange).toHaveBeenCalledWith({ value: "C" });
    expect(screen.queryByRole("listbox")).toBeNull();
  });

  it("toggles multiple values in a checkbox group", () => {
    const onChange = vi.fn();
    render(
      <FormField
        field={field({ key: "areas", label: "Areas", type: "checkbox", options: ["Neck", "Back"] })}
        answer={{ values: ["Neck"] }}
        onChange={onChange}
      />,
    );

    expect(screen.getByRole("checkbox", { name: "Neck" }).getAttribute("aria-checked")).toBe("true");

    fireEvent.click(screen.getByRole("checkbox", { name: "Back" }));
    expect(onChange).toHaveBeenCalledWith({ values: ["Neck", "Back"] });

    fireEvent.click(screen.getByRole("checkbox", { name: "Neck" }));
    expect(onChange).toHaveBeenCalledWith({ values: [] });
  });

  it("renders an optionless checkbox as a single true/false toggle", () => {
    const onChange = vi.fn();
    render(
      <FormField
        field={field({ key: "consent", label: "I agree", type: "checkbox" })}
        answer={{}}
        onChange={onChange}
      />,
    );

    fireEvent.click(screen.getByRole("checkbox", { name: "I agree" }));
    expect(onChange).toHaveBeenCalledWith({ value: "true" });
  });

  it("shows an error against the field it belongs to", () => {
    render(
      <FormField field={field()} answer={{}} error="This one is needed." onChange={vi.fn()} />,
    );

    expect(screen.getByRole("alert").textContent).toBe("This one is needed.");
  });

  it("asks for a date in the format the API accepts", () => {
    render(
      <FormField
        field={field({ key: "last_treatment", label: "Last treatment", type: "date" })}
        answer={{}}
        onChange={vi.fn()}
      />,
    );

    expect(screen.getByPlaceholderText("YYYY-MM-DD")).toBeTruthy();
  });

  it("locks every control when the form is already submitted", () => {
    render(
      <FormField
        field={field({ key: "areas", label: "Areas", type: "checkbox", options: ["Neck"] })}
        answer={{ values: [] }}
        disabled
        onChange={vi.fn()}
      />,
    );

    expect(screen.getByRole("checkbox", { name: "Neck" })).toHaveProperty("disabled", true);
  });
});
