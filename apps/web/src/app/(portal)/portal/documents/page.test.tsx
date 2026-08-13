import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ClientDocument } from "@/lib/portal";
import DocumentsPage from "./page";

const listMine = vi.hoisted(() => vi.fn());
const downloadUrl = vi.hoisted(() => vi.fn());

vi.mock("@/lib/portal", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/portal")>();
  return { ...original, documentsApi: { listMine, downloadUrl } };
});

vi.mock("@/lib/auth", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/auth")>();
  return {
    ...original,
    useAuth: () => ({
      status: "authenticated",
      user: { id: "u1", email: "ama@example.com", role: "client", name: "Ama Serwaa" },
      accessToken: "a1",
      session: { accessToken: "a1", accessTokenExpiresAt: "2099-01-01T00:00:00Z", refreshToken: "r1" },
      onTokensRefreshed: vi.fn(),
      login: vi.fn(),
      register: vi.fn(),
      logout: vi.fn(),
    }),
  };
});

function document(overrides: Partial<ClientDocument> = {}): ClientDocument {
  return {
    id: "doc-1",
    title: "Your wellness plan",
    filename: "wellness-plan.pdf",
    format: "pdf",
    bytes: 204800,
    createdAt: "2026-08-03T09:00:00Z",
    ...overrides,
  };
}

describe("DocumentsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listMine.mockResolvedValue([document()]);
    downloadUrl.mockResolvedValue("https://delivery.test/signed");
    vi.stubGlobal("open", vi.fn());
  });

  it("lists shared documents with their size and date", async () => {
    render(<DocumentsPage />);

    expect(await screen.findByText("Your wellness plan")).toBeTruthy();
    expect(screen.getByText(/200 KB/)).toBeTruthy();
    expect(screen.getByText(/PDF/)).toBeTruthy();
  });

  it("fetches the signed link only when a download is asked for", async () => {
    render(<DocumentsPage />);
    await screen.findByText("Your wellness plan");

    // A short-lived signed URL has no business sitting in the page.
    expect(downloadUrl).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: /download/i }));

    await waitFor(() => {
      expect(downloadUrl).toHaveBeenCalledWith(expect.anything(), expect.anything(), "doc-1");
    });
    expect(window.open).toHaveBeenCalledWith(
      "https://delivery.test/signed",
      "_blank",
      "noopener,noreferrer",
    );
  });

  it("explains a failed download without losing the list", async () => {
    const { ApiError } = await import("@/lib/api");
    downloadUrl.mockRejectedValueOnce(new ApiError(404, "document_not_found", "document not found"));
    render(<DocumentsPage />);
    await screen.findByText("Your wellness plan");

    fireEvent.click(screen.getByRole("button", { name: /download/i }));

    expect((await screen.findByRole("alert")).textContent).toContain("document not found");
    expect(screen.getByText("Your wellness plan")).toBeTruthy();
  });

  it("says what will fill an empty library", async () => {
    listMine.mockResolvedValue([]);

    render(<DocumentsPage />);

    expect(await screen.findByRole("heading", { name: /nothing here yet/i })).toBeTruthy();
    expect(screen.getByText(/when your practitioner shares a document/i)).toBeTruthy();
  });

  it("keeps loading distinct from empty", async () => {
    let resolve: (value: ClientDocument[]) => void = () => {};
    listMine.mockReturnValue(new Promise<ClientDocument[]>((r) => (resolve = r)));

    render(<DocumentsPage />);

    // A slow network must never read as "you have no documents".
    expect(screen.getByRole("status")).toBeTruthy();
    expect(screen.queryByRole("heading", { name: /nothing here yet/i })).toBeNull();

    resolve([]);
    expect(await screen.findByRole("heading", { name: /nothing here yet/i })).toBeTruthy();
  });

  it("offers a retry when the library will not load", async () => {
    const { ApiError } = await import("@/lib/api");
    listMine.mockRejectedValue(new ApiError(0, "network_error", "offline"));

    render(<DocumentsPage />);

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toMatch(/can't reach the practice/i);
    expect(screen.getByRole("button", { name: /try again/i })).toBeTruthy();
  });
});
