package httpapi

import (
	"github.com/xcreativs/terios/api/internal/domain/notification"
	"github.com/xcreativs/terios/api/internal/ports"
	"net/http"
	"testing"
	"time"
)

func TestNotificationFeedOwnershipAndReadPersistence(t *testing.T) {
	r := newJourneyRig(t)
	r.notifications.FormAssigned(t.Context(), ports.FormAssignedNotice{ClientEmail: r.clientEmail, SubmissionID: "form-1", FormTitle: "Intake"})
	foreign, _ := notification.NewInApp("someone@example.com", notification.KindActivity, "Private update", "", "/portal/forms", "", time.Now())
	foreign, _ = r.inApp.Create(t.Context(), foreign)
	var feed struct {
		Items       []notificationBody `json:"items"`
		UnreadCount int                `json:"unreadCount"`
	}
	r.mustJSON(t, http.MethodGet, "/v1/notifications?recipientEmail=someone@example.com", nil, r.clientToken, http.StatusOK, &feed)
	if len(feed.Items) != 1 || feed.UnreadCount != 1 || feed.Items[0].Title != "Please complete Intake" {
		t.Fatalf("wrong feed: %+v", feed)
	}
	r.mustJSON(t, http.MethodPost, "/v1/notifications/"+foreign.ID+"/read", nil, r.clientToken, http.StatusNotFound, nil)
	r.mustJSON(t, http.MethodPost, "/v1/notifications/"+feed.Items[0].ID+"/read", nil, r.clientToken, http.StatusNoContent, nil)
	r.mustJSON(t, http.MethodGet, "/v1/notifications", nil, r.clientToken, http.StatusOK, &feed)
	if feed.UnreadCount != 0 || !feed.Items[0].Read {
		t.Fatalf("read not persisted: %+v", feed)
	}
	r.notifications.FormAssigned(t.Context(), ports.FormAssignedNotice{ClientEmail: r.clientEmail, SubmissionID: "form-2", FormTitle: "Follow-up"})
	r.mustJSON(t, http.MethodPost, "/v1/notifications/read-all", nil, r.clientToken, http.StatusNoContent, nil)
	r.mustJSON(t, http.MethodGet, "/v1/notifications", nil, r.clientToken, http.StatusOK, &feed)
	if feed.UnreadCount != 0 || len(feed.Items) != 2 {
		t.Fatal(feed)
	}
	unread, _ := r.inApp.UnreadCount(t.Context(), "someone@example.com")
	if unread != 1 {
		t.Fatal("mark-all changed another account")
	}
	r.mustJSON(t, http.MethodGet, "/v1/notifications", nil, "", http.StatusUnauthorized, nil)
}
