import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { TextInput } from "./TextInput";

describe("TextInput", () => {
  it("associates the label with the control", () => {
    render(<TextInput label="Email" />);
    expect(screen.getByLabelText("Email")).toBeTruthy();
  });

  it("renders a hint linked via aria-describedby", () => {
    render(<TextInput label="Password" hint="Use at least 12 characters." />);
    const input = screen.getByLabelText("Password");
    const hint = screen.getByText("Use at least 12 characters.");
    expect(input.getAttribute("aria-describedby")).toBe(hint.id);
  });

  it("renders the error with role=alert, aria-invalid, and swaps the hint out", () => {
    render(<TextInput label="Email" hint="We never share it." error="Enter your email" />);
    const input = screen.getByLabelText("Email");
    const error = screen.getByRole("alert");
    expect(error.textContent).toContain("Enter your email");
    expect(input.getAttribute("aria-invalid")).toBe("true");
    expect(input.getAttribute("aria-describedby")).toBe(error.id);
    expect(screen.queryByText("We never share it.")).toBeNull();
  });

  it("marks required fields with an asterisk and sr-only text", () => {
    render(<TextInput label="Email" required />);
    const label = screen.getByText("Email");
    expect(label.textContent).toContain("*");
    expect(label.textContent).toContain("(required)");
    expect(screen.getByLabelText(/Email/).getAttribute("aria-required")).toBe("true");
  });

  it("toggles password visibility with the Eye/EyeOff button", () => {
    render(<TextInput label="Password" type="password" />);
    const input = screen.getByLabelText("Password");
    expect(input.getAttribute("type")).toBe("password");

    fireEvent.click(screen.getByRole("button", { name: "Show password" }));
    expect(input.getAttribute("type")).toBe("text");

    fireEvent.click(screen.getByRole("button", { name: "Hide password" }));
    expect(input.getAttribute("type")).toBe("password");
  });

  it("renders the leading icon and pads the control", () => {
    render(<TextInput label="Email" leadingIcon={<svg data-testid="icon" />} />);
    expect(screen.getByTestId("icon")).toBeTruthy();
    expect(screen.getByLabelText("Email").className).toContain("pl-9");
  });

  it("applies the disabled state", () => {
    render(<TextInput label="Email" disabled />);
    const input = screen.getByLabelText("Email");
    expect(input).toHaveProperty("disabled", true);
    expect(input.className).toContain("bg-surface-sunken");
  });
});
