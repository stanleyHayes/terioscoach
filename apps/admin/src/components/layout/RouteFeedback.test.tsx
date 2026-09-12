import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { RouteFeedback } from "./RouteFeedback";
import Link from "next/link";

const pathname = vi.hoisted(() => vi.fn(() => "/"));
const prefetch = vi.hoisted(() => vi.fn());
vi.mock("next/navigation", () => ({ usePathname: pathname, useRouter: () => ({ prefetch }) }));

describe("RouteFeedback", () => {
  it("prefetches on intent and acknowledges navigation immediately", () => {
    render(<><RouteFeedback/><Link href="/clients">Clients</Link></>);
    const link = screen.getByRole("link", { name: "Clients" });
    fireEvent.pointerOver(link);
    expect(prefetch).toHaveBeenCalledWith("/clients");
    fireEvent.click(link);
    expect(document.querySelector(".terios-route-progress")?.hasAttribute("data-active")).toBe(true);
  });
  it("does not revive a completed transition when a save returns programmatically", () => {
    pathname.mockReturnValue("/content");
    const { rerender } = render(<><RouteFeedback /><Link href="/content/posts/1/edit">Edit post</Link></>);
    fireEvent.click(screen.getByRole("link", { name: "Edit post" }));
    expect(document.querySelector(".terios-route-progress")?.hasAttribute("data-active")).toBe(true);
    pathname.mockReturnValue("/content/posts/1/edit");
    rerender(<RouteFeedback />);
    expect(document.querySelector(".terios-route-progress")?.hasAttribute("data-active")).toBe(false);
    pathname.mockReturnValue("/content");
    rerender(<RouteFeedback />);
    expect(document.querySelector(".terios-route-progress")?.hasAttribute("data-active")).toBe(false);
  });

});
