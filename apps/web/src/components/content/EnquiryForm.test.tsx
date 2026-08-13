import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { EnquiryForm } from "./EnquiryForm";

const submitEnquiry = vi.hoisted(() => vi.fn());
vi.mock("@/lib/content", () => ({ submitEnquiry }));

/** Fills the form with a valid message. */
function fillValid() {
  fireEvent.change(screen.getByLabelText(/your name/i), { target: { value: "Ama Serwaa" } });
  fireEvent.change(screen.getByLabelText(/email/i), { target: { value: "ama@example.com" } });
  fireEvent.change(screen.getByLabelText(/your message/i), {
    target: { value: "Do you offer prenatal massage?" },
  });
}

describe("EnquiryForm", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    submitEnquiry.mockResolvedValue(undefined);
  });

  it("sends a valid enquiry and confirms it landed", async () => {
    render(<EnquiryForm />);
    fillValid();
    fireEvent.change(screen.getByLabelText(/phone/i), { target: { value: " +233201234567 " } });

    fireEvent.click(screen.getByRole("button", { name: /send message/i }));

    await waitFor(() => {
      expect(submitEnquiry).toHaveBeenCalledTimes(1);
    });
    expect(submitEnquiry).toHaveBeenCalledWith({
      name: "Ama Serwaa",
      email: "ama@example.com",
      phone: "+233201234567",
      subject: undefined,
      message: "Do you offer prenatal massage?",
    });

    expect(await screen.findByRole("status")).toBeTruthy();
    expect(screen.getByText(/it has arrived/i)).toBeTruthy();
    // The confirmation names the address, so a typo is visible.
    expect(screen.getByText("ama@example.com")).toBeTruthy();
  });

  it("catches the common mistakes before any round trip", async () => {
    render(<EnquiryForm />);

    fireEvent.click(screen.getByRole("button", { name: /send message/i }));

    expect(await screen.findByText(/please tell us your name/i)).toBeTruthy();
    expect(screen.getByText(/we need an address to reply to/i)).toBeTruthy();
    expect(screen.getByText(/tell us a little about what you need/i)).toBeTruthy();
    expect(submitEnquiry).not.toHaveBeenCalled();
  });

  it("rejects an address with no dotted domain", async () => {
    render(<EnquiryForm />);
    fireEvent.change(screen.getByLabelText(/your name/i), { target: { value: "Ama" } });
    fireEvent.change(screen.getByLabelText(/email/i), { target: { value: "ama@example" } });
    fireEvent.change(screen.getByLabelText(/your message/i), { target: { value: "Hello" } });

    fireEvent.click(screen.getByRole("button", { name: /send message/i }));

    expect(await screen.findByText(/doesn't look like an email address/i)).toBeTruthy();
    expect(submitEnquiry).not.toHaveBeenCalled();
  });

  it("explains the rate limit in the practice's own voice", async () => {
    submitEnquiry.mockRejectedValueOnce(new ApiError(429, "rate_limited", "too many requests"));
    render(<EnquiryForm />);
    fillValid();

    fireEvent.click(screen.getByRole("button", { name: /send message/i }));

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toMatch(/few messages in a short time/i);
    // The form stays filled in so nobody retypes their message.
    expect(screen.getByLabelText(/your message/i)).toHaveProperty(
      "value",
      "Do you offer prenatal massage?",
    );
  });

  it("explains an unreachable server without blaming the visitor", async () => {
    submitEnquiry.mockRejectedValueOnce(new ApiError(0, "network_error", "offline"));
    render(<EnquiryForm />);
    fillValid();

    fireEvent.click(screen.getByRole("button", { name: /send message/i }));

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toMatch(/can't reach the practice/i);
  });

  it("surfaces a server validation message verbatim", async () => {
    submitEnquiry.mockRejectedValueOnce(
      new ApiError(400, "validation_error", "message is required"),
    );
    render(<EnquiryForm />);
    fillValid();

    fireEvent.click(screen.getByRole("button", { name: /send message/i }));

    expect((await screen.findByRole("alert")).textContent).toContain("message is required");
  });

  it("turns off the browser's own validation bubbles", () => {
    const { container } = render(<EnquiryForm />);
    // Native validation UI is not ours to style, so the form opts out and
    // every message comes from the design system instead.
    expect(container.querySelector("form")?.hasAttribute("novalidate")).toBe(true);
  });
});
