import { authedRequest, type Session, type RefreshCallbacks } from "./api";

export interface FeedNotification {
  id: string; kind: string; title: string; body?: string; link?: string;
  read: boolean; createdAt: string;
}
export interface NotificationFeed { items: FeedNotification[]; unreadCount: number }
export const notificationFeedApi = {
  list: (session: Session, callbacks: RefreshCallbacks) =>
    authedRequest<NotificationFeed>("/v1/notifications", session, callbacks),
  markRead: (session: Session, callbacks: RefreshCallbacks, id: string) =>
    authedRequest<void>(`/v1/notifications/${encodeURIComponent(id)}/read`, session, callbacks, { method: "POST" }),
  markAllRead: (session: Session, callbacks: RefreshCallbacks) =>
    authedRequest<void>("/v1/notifications/read-all", session, callbacks, { method: "POST" }),
};
// Only local app destinations are navigable, even if an old record is malformed.
export function notificationHref(link?: string): string | null {
  return link?.startsWith("/") && !link.startsWith("//") && !link.includes("\\") ? link : null;
}
