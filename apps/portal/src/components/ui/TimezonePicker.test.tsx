import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { TimezonePicker } from "./TimezonePicker";

function open() {
  fireEvent.click(screen.getByRole("combobox", { name: "Appointment timezone" }));
}

describe("TimezonePicker", () => {
  it("shows the chosen zone and lists United States zones first", () => {
    render(<TimezonePicker label="Appointment timezone" value="America/New_York" onChange={() => {}} />);
    expect(screen.getByRole("combobox", { name: "Appointment timezone" }).textContent).toContain(
      "Eastern Time (US)",
    );
    open();
    const options = screen.getAllByRole("option").map((o) => o.textContent ?? "");
    expect(options.slice(0, 7).map((o) => o.split(" (US)")[0])).toEqual([
      "Eastern Time",
      "Central Time",
      "Mountain Time",
      "Arizona Time",
      "Pacific Time",
      "Alaska Time",
      "Hawaii Time",
    ]);
    expect(options.length).toBeGreaterThan(100);
  });

  it("filters as you type in the one search box and saves the pick", () => {
    const onChange = vi.fn();
    render(<TimezonePicker label="Appointment timezone" value="America/New_York" onChange={onChange} />);
    open();
    fireEvent.change(screen.getByRole("combobox", { name: "Search time zones" }), {
      target: { value: "accra" },
    });
    const options = screen.getAllByRole("option");
    expect(options).toHaveLength(1);
    expect(within(options[0]).getByText("Accra")).toBeTruthy();
    fireEvent.click(options[0]);
    expect(onChange).toHaveBeenCalledWith("Africa/Accra");
  });

  it("finds US zones by everyday names and says when nothing matches", () => {
    render(<TimezonePicker label="Appointment timezone" value="America/New_York" onChange={() => {}} />);
    open();
    const search = screen.getByRole("combobox", { name: "Search time zones" });
    fireEvent.change(search, { target: { value: "pacific" } });
    expect(screen.getAllByRole("option")[0].textContent).toContain("Pacific Time (US)");
    fireEvent.change(search, { target: { value: "zzzz" } });
    expect(screen.queryAllByRole("option")).toHaveLength(0);
    expect(screen.getByText("No time zone matches that search.")).toBeTruthy();
  });
});
