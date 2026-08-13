import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { FAQ } from "@/lib/content";
import { FAQSearch } from "./FAQSearch";

const faqs: FAQ[] = [
  {
    id: "1",
    question: "Do you offer prénatal massage?",
    answer: "Yes, from the second trimester.",
    category: "Sessions",
    sortOrder: 1,
  },
  {
    id: "2",
    question: "How do I pay?",
    answer: "Card or mobile money, at the time of booking.",
    category: "Booking",
    sortOrder: 2,
  },
  {
    id: "3",
    question: "Can I reschedule?",
    answer: "Up to 24 hours before your session.",
    sortOrder: 3,
  },
];

describe("FAQSearch", () => {
  it("groups questions by category with the uncategorised group last", () => {
    render(<FAQSearch faqs={faqs} />);

    const headings = screen.getAllByRole("heading", { level: 2 }).map((h) => h.textContent);
    expect(headings).toEqual(["Sessions", "Booking", "More questions"]);
  });

  it("keeps answers collapsed until their question is opened", () => {
    render(<FAQSearch faqs={faqs} />);

    const question = screen.getByRole("button", { name: /how do i pay/i });
    expect(question.getAttribute("aria-expanded")).toBe("false");
    expect(screen.getByText(/card or mobile money/i).closest("[hidden]")).not.toBeNull();

    fireEvent.click(question);

    expect(question.getAttribute("aria-expanded")).toBe("true");
    expect(screen.getByText(/card or mobile money/i).closest("[hidden]")).toBeNull();
  });

  it("opens one answer at a time", () => {
    render(<FAQSearch faqs={faqs} />);

    const first = screen.getByRole("button", { name: /how do i pay/i });
    const second = screen.getByRole("button", { name: /can i reschedule/i });

    fireEvent.click(first);
    fireEvent.click(second);

    expect(first.getAttribute("aria-expanded")).toBe("false");
    expect(second.getAttribute("aria-expanded")).toBe("true");
  });

  it("closes an open answer when its question is clicked again", () => {
    render(<FAQSearch faqs={faqs} />);
    const question = screen.getByRole("button", { name: /how do i pay/i });

    fireEvent.click(question);
    fireEvent.click(question);

    expect(question.getAttribute("aria-expanded")).toBe("false");
  });

  it("filters as you type, ignoring case and accents", () => {
    render(<FAQSearch faqs={faqs} />);
    const search = screen.getByLabelText(/search the questions/i);

    fireEvent.change(search, { target: { value: "PRENATAL" } });

    expect(screen.getByRole("button", { name: /prénatal massage/i })).toBeTruthy();
    expect(screen.queryByRole("button", { name: /how do i pay/i })).toBeNull();
  });

  it("announces the match count politely", () => {
    render(<FAQSearch faqs={faqs} />);

    fireEvent.change(screen.getByLabelText(/search the questions/i), {
      target: { value: "pay" },
    });

    expect(screen.getByText(/1 question match/i)).toBeTruthy();
  });

  it("offers a way forward when nothing matches", () => {
    render(<FAQSearch faqs={faqs} />);

    fireEvent.change(screen.getByLabelText(/search the questions/i), {
      target: { value: "helicopter" },
    });

    expect(screen.getByRole("heading", { name: /nothing matches that/i })).toBeTruthy();
    expect(screen.queryByRole("button", { name: /how do i pay/i })).toBeNull();
  });

  it("restores everything when the search is cleared", () => {
    render(<FAQSearch faqs={faqs} />);
    const search = screen.getByLabelText(/search the questions/i);

    fireEvent.change(search, { target: { value: "pay" } });
    fireEvent.change(search, { target: { value: "" } });

    expect(screen.getAllByRole("heading", { level: 2 })).toHaveLength(3);
  });

  it("uses no native disclosure elements", () => {
    const { container } = render(<FAQSearch faqs={faqs} />);

    // The platform rule: every interactive element is built, not borrowed.
    expect(container.querySelector("details")).toBeNull();
    expect(container.querySelector("summary")).toBeNull();
  });
});
