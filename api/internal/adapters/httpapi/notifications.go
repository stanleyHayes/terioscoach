package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xcreativs/terios/api/internal/domain/notification"
	"github.com/xcreativs/terios/api/internal/ports"
)

// WithInAppNotifications mounts the in-app notification feed — the bell in
// both the client portal and the practice dashboard.
//
//	GET  /v1/notifications                → {items, unreadCount}
//	POST /v1/notifications/{id}/read      → {} marks one read
//	POST /v1/notifications/read-all       → {} marks every unread read
//
// Every route resolves the recipient from the authenticated principal's own
// email, the same address the matching event was emailed to — so a person
// can only ever see and act on their own feed.
func WithInAppNotifications(repo ports.InAppNotificationRepository, auth ports.AuthService) Option {
	return func(s *Server) {
		if repo == nil {
			s.Router.HandleFunc("/v1/notifications", handleNotificationsUnavailable)
			s.Router.HandleFunc("/v1/notifications/*", handleNotificationsUnavailable)
			return
		}
		h := &inAppNotificationHandler{repo: repo, auth: auth}
		s.Router.Group(func(r chi.Router) {
			r.Use(RequireAuth(auth))
			r.Get("/v1/notifications", h.list)
			r.Post("/v1/notifications/{id}/read", h.markRead)
			r.Post("/v1/notifications/read-all", h.markAllRead)
		})
	}
}

func handleNotificationsUnavailable(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusServiceUnavailable, "service_unavailable",
		"notifications are unavailable: database not connected")
}

type inAppNotificationHandler struct {
	repo ports.InAppNotificationRepository
	auth ports.AuthService
}

// recipientEmail resolves the authenticated principal's own email, which is
// the recipient key the notification feed is stored under.
func (h *inAppNotificationHandler) recipientEmail(w http.ResponseWriter, r *http.Request) (string, bool) {
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return "", false
	}
	user, err := h.auth.CurrentUser(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "account no longer exists")
		return "", false
	}
	return user.Email, true
}

type notificationBody struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Title     string    `json:"title"`
	Body      string    `json:"body,omitempty"`
	Link      string    `json:"link,omitempty"`
	BookingID string    `json:"bookingId,omitempty"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"createdAt"`
}

func newNotificationBody(n notification.InApp) notificationBody {
	return notificationBody{
		ID:        n.ID,
		Kind:      string(n.Kind),
		Title:     n.Title,
		Body:      n.Body,
		Link:      n.Link,
		BookingID: n.BookingID,
		Read:      n.Read,
		CreatedAt: n.CreatedAt,
	}
}

// list handles GET /v1/notifications.
func (h *inAppNotificationHandler) list(w http.ResponseWriter, r *http.Request) {
	email, ok := h.recipientEmail(w, r)
	if !ok {
		return
	}
	items, err := h.repo.ListForRecipient(r.Context(), email, 50)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	unread, err := h.repo.UnreadCount(r.Context(), email)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	bodies := make([]notificationBody, 0, len(items))
	for _, item := range items {
		bodies = append(bodies, newNotificationBody(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":       bodies,
		"unreadCount": unread,
	})
}

// markRead handles POST /v1/notifications/{id}/read.
func (h *inAppNotificationHandler) markRead(w http.ResponseWriter, r *http.Request) {
	email, ok := h.recipientEmail(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.repo.MarkRead(r.Context(), id, email); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// markAllRead handles POST /v1/notifications/read-all.
func (h *inAppNotificationHandler) markAllRead(w http.ResponseWriter, r *http.Request) {
	email, ok := h.recipientEmail(w, r)
	if !ok {
		return
	}
	if err := h.repo.MarkAllRead(r.Context(), email); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
