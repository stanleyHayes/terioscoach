import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { PortalNotificationCenter } from "./PortalNotificationCenter";
import { notificationHref } from "@/lib/notification-feed";
const { resource, action, refresh, markRead, markAllRead } = vi.hoisted(() => ({ resource: vi.fn(), action: vi.fn(), refresh: vi.fn(), markRead: vi.fn(), markAllRead: vi.fn() }));
vi.mock("@/lib/use-portal-data", () => ({ usePortalData: resource, usePortalAction: action }));
vi.mock("@/lib/notification-feed", async (original) => ({ ...await original<typeof import("@/lib/notification-feed")>(), notificationFeedApi: { list: vi.fn(), markRead, markAllRead } }));
const item = { id: "n1", title: "A form is ready", link: "/portal/forms", kind: "form_assigned", read: false, createdAt: "2026-09-17T12:00:00Z" };
beforeEach(() => {
 vi.clearAllMocks();
 resource.mockReturnValue({ data: { items: [item], unreadCount: 12 }, error: null, refresh });
 action.mockReturnValue({ run: (_key: string, fn: (s: object,c: object) => Promise<boolean>) => fn({},{}), pending: null, error: null });
 markRead.mockResolvedValue(undefined); markAllRead.mockResolvedValue(undefined);
});
function open() { fireEvent.click(screen.getByRole("button", { name: /^Notifications/ })); }
it("shows persistent unread counts and the correct destination", () => {
 render(<PortalNotificationCenter />); open();
 expect(screen.getByText("9+")).toBeTruthy();
 expect(screen.getByRole("dialog", { name: "Client notifications" })).toBeTruthy();
 expect(screen.getByRole("link", { name: /A form is ready/ }).getAttribute("href")).toBe("/portal/forms");
});
it("persists individual and bulk read actions then reloads", async () => {
 render(<PortalNotificationCenter />); open();
 fireEvent.click(screen.getByRole("button", { name: "Mark as read: A form is ready" }));
 await waitFor(() => expect(markRead).toHaveBeenCalledWith({}, {}, "n1"));
 fireEvent.click(screen.getByRole("button", { name: "Mark all as read" }));
 await waitFor(() => expect(markAllRead).toHaveBeenCalledOnce());
 await waitFor(() => expect(refresh).toHaveBeenCalledTimes(3));
});
it("keeps read history without unread controls", () => {
 resource.mockReturnValue({ data: { items: [{ ...item, read: true }], unreadCount: 0 }, error: null, refresh });
 render(<PortalNotificationCenter />); open();
 expect(screen.getByRole("link", { name: /A form is ready/ })).toBeTruthy();
 expect(screen.queryByRole("button", { name: /Mark/ })).toBeNull();
});
it("shows loading, errors and an honest empty state", () => {
 resource.mockReturnValue({ data: null, error: null, refresh });
 const view = render(<PortalNotificationCenter />); open();
 expect(screen.getByLabelText("Loading notifications")).toBeTruthy();
 resource.mockReturnValue({ data: null, error: "offline", refresh }); view.rerender(<PortalNotificationCenter />);
 expect(screen.getByRole("alert")).toBeTruthy();
 resource.mockReturnValue({ data: { items: [], unreadCount: 0 }, error: null, refresh }); view.rerender(<PortalNotificationCenter />);
 expect(screen.getByText("You’re all caught up")).toBeTruthy();
});
it("surfaces read failures without hiding unread rows", () => {
 action.mockReturnValue({ run: vi.fn(), pending: null, error: "Could not save read status" });
 render(<PortalNotificationCenter />); open();
 expect(screen.getByRole("alert").textContent).toContain("Could not save");
 expect(screen.getByLabelText("Unread")).toBeTruthy();
});
it("closes on Escape and returns focus to the bell", () => {
 render(<PortalNotificationCenter />); open(); fireEvent.keyDown(document, { key: "Escape" });
 expect(screen.queryByRole("dialog")).toBeNull();
 expect(document.activeElement).toBe(screen.getByRole("button", { name: /^Notifications/ }));
});
it("refuses external, protocol-relative and backslash links", () => {
 expect(notificationHref("https://example.com")).toBeNull();
 expect(notificationHref("//example.com")).toBeNull();
 expect(notificationHref("/\\example.com")).toBeNull();
 expect(notificationHref("/portal/forms")).toBe("/portal/forms");
});
