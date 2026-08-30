import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { RouteFeedback } from "./RouteFeedback";
import Link from "next/link";

const prefetch = vi.hoisted(() => vi.fn());
vi.mock("next/navigation", () => ({ usePathname: () => "/portal", useRouter: () => ({ prefetch }) }));

describe("RouteFeedback", () => {
  it("warms routes and starts progress on an internal click", () => {
    render(<><RouteFeedback/><Link href="/portal/forms">Forms</Link></>);
    const link = screen.getByRole("link", { name: "Forms" });
    fireEvent.focus(link);
    expect(prefetch).toHaveBeenCalledWith("/portal/forms");
    fireEvent.click(link);
    expect(document.querySelector(".terios-route-progress")?.hasAttribute("data-active")).toBe(true);
  });
});
