package notifications

import (
	"context"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/notification"
	"github.com/xcreativs/terios/api/internal/ports"
)

// Activity writes durable jobs for both affected audiences. These jobs never
// send email; they retry feed persistence through the existing dispatcher.
func (s *Service) Activity(ctx context.Context, notice ports.ActivityNotice) {
	if s.inApp == nil {
		return
	}
	send := func(recipient, link string) {
		if recipient == "" || link == "" {
			return
		}
		s.queue(ctx, notification.KindActivity, recipient, "", map[string]string{"eventId": notice.EventID, "title": notice.Title, "link": link}, s.now())
	}
	if notice.ClientID != "" && s.users != nil && notice.ClientLink != "" {
		user, err := s.users.FindByID(ctx, notice.ClientID)
		if err != nil {
			s.report(err)
		} else {
			send(user.Email, notice.ClientLink)
		}
	}
	if notice.PracticeLink != "" {
		if notice.PractitionerID != "" && s.users != nil {
			user, err := s.users.FindByID(ctx, notice.PractitionerID)
			if err != nil {
				s.report(err)
			} else {
				send(user.Email, notice.PracticeLink)
			}
		} else {
			send(s.practiceAddress(ctx), notice.PracticeLink)
		}
	}
}

// The in-app practice inbox remains available without an email-routing setting.
func (s *Service) practiceAddress(ctx context.Context) string {
	if s.practiceEmail != "" {
		return s.practiceEmail
	}
	if s.users != nil {
		if user, err := s.users.FindFirstByRole(ctx, identity.RolePractitioner); err == nil {
			return user.Email
		}
	}
	return ""
}
