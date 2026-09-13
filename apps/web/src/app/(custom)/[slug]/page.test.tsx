import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import CustomPage, { generateMetadata } from "./page";
const getPage = vi.hoisted(() => vi.fn());
vi.mock("@/lib/content", () => ({ getPage }));
vi.mock("next/navigation", () => ({ notFound: () => { throw new Error("NEXT_NOT_FOUND"); } }));
beforeEach(() => { getPage.mockReset(); });
describe("custom published pages", () => {
  it("renders the published body, image and search metadata", async () => {
    getPage.mockResolvedValue({ slug: "testing", title: "Testing", body: "## A custom section\n\nPublished content.", coverImage: "/images/test.webp", metaTitle: "Testing at Terios", metaDescription: "Page summary" });
    const props = { params: Promise.resolve({ slug: "testing" }) };
    render(await CustomPage(props));
    expect(screen.getByRole("heading", { name: "Testing" })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "A custom section" })).toBeTruthy();
    expect(screen.getByAltText("Testing").getAttribute("src")).toBe("/images/test.webp");
    expect(await generateMetadata(props)).toMatchObject({ title: "Testing at Terios", alternates: { canonical: "/testing" } });
  });
  it("keeps drafts, missing pages and internal content records out of public routes", async () => {
    getPage.mockRejectedValue(new ApiError(404, "page_not_found", "Not found"));
    await expect(CustomPage({ params: Promise.resolve({ slug: "draft" }) })).rejects.toThrow("NEXT_NOT_FOUND");
    getPage.mockClear();
    await expect(CustomPage({ params: Promise.resolve({ slug: "website-content-home" }) })).rejects.toThrow("NEXT_NOT_FOUND");
    expect(getPage).not.toHaveBeenCalled();
  });
  it("does not disguise service failures as missing pages", async () => {
    getPage.mockRejectedValue(new Error("Service unavailable"));
    await expect(CustomPage({ params: Promise.resolve({ slug: "testing" }) })).rejects.toThrow("Service unavailable");
  });
});
