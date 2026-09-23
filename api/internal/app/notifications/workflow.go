package notifications

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/notification"
	"github.com/xcreativs/terios/api/internal/ports"
)

// reconcile replays committed mutations into independently deduplicated channel jobs.
// A multi-recipient failure leaves the event pending: successful recipients are
// preserved by the unique event/recipient/kind index, including their read state.
func (s *Service) reconcile(ctx context.Context, limit int) {
	events, err := s.events.Pending(ctx, limit)
	if err != nil {
		s.report(err)
		return
	}
	for _, event := range events {
		replay := *s
		replay.journalOnly = false
		var failures []error
		replay.report = func(err error) { failures = append(failures, err); s.report(err) }
		if err := replay.replayMutation(ctx, event); err != nil {
			failures = append(failures, err)
		}
		if (event.Collection == "agreement_executions" || event.Collection == "agreement_signatures") && s.archiveExecution != nil {
			if err := s.archiveExecution(ctx, event.EntityID); err != nil {
				failures = append(failures, err)
				s.report(err)
			}
		}
		if len(failures) > 0 {
			_ = s.events.Failed(ctx, event.ID, "recipient or queue unavailable")
			slog.Warn("workflow retry", "eventId", event.ID, "failures", len(failures))
			continue
		}
		if err := s.events.Complete(ctx, event.ID); err != nil {
			s.report(err)
		} else {
			slog.Info("workflow queued", "eventId", event.ID, "entityId", event.EntityID)
		}
	}
}

func (s *Service) replayMutation(ctx context.Context, e ports.WorkflowEvent) error {
	if e.Collection == "agreements" || e.Collection == "services" {
		if len(e.Before) == 0 || (e.Collection == "agreements" && e.After["version"] == e.Before["version"]) || (e.Collection == "services" && e.After["agreementId"] == e.Before["agreementId"] && e.After["agreementIds"] == e.Before["agreementIds"] && e.After["agreementCollectionId"] == e.Before["agreementCollectionId"]) {
			return nil
		}
		if s.bookings == nil || s.catalog == nil {
			return fmt.Errorf("document request routing unavailable")
		}
		bookings, err := s.bookings.ListByPractitioner(ctx, e.After["practitionerId"], ports.BookingFilter{IncludePendingPayment: true})
		if err != nil {
			return err
		}
		for _, b := range bookings {
			if !b.StartAt.After(s.now()) || (b.Status != "confirmed" && b.Status != "pending_payment") {
				continue
			}
			svc, err := s.catalog.FindByID(ctx, b.ServiceID)
			if err != nil {
				return err
			}
			required := (e.Collection == "services" && b.ServiceID == e.EntityID && len(svc.RequiredAgreementIDs()) > 0) || (e.After["key"] == "guardian_consent" && b.Participant != nil && b.Participant.Under18)
			for _, id := range svc.RequiredAgreementIDs() {
				if id == e.EntityID {
					required = true
				}
			}
			if required {
				child := e
				child.Collection = "document_requested"
				child.ID = e.ID + ":" + b.ID
				child.After = map[string]string{"clientId": b.ClientID, "practitionerId": b.PractitionerID, "bookingId": b.ID}
				if err = s.replayMutation(ctx, child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	a, b := e.After, e.Before
	if e.Collection == "bookings" && (a["status"] != "confirmed" || a["startAt"] != b["startAt"]) {
		jobs, err := s.jobs.PendingByBooking(ctx, e.EntityID, notification.KindSessionReminder)
		if err != nil {
			return err
		}
		for _, job := range jobs {
			if job.Data["startAt"] == a["startAt"] && a["status"] == "confirmed" {
				continue
			}
			if err = job.Cancel(s.now()); err != nil {
				return err
			}
			if _, err = s.jobs.Update(ctx, job); err != nil {
				return err
			}
		}
	}
	clientID, practitionerID, bookingID := a["clientId"], a["practitionerId"], a["bookingId"]
	if e.Collection == "bookings" {
		bookingID = e.EntityID
	}
	if bookingID != "" && s.bookings != nil {
		booking, err := s.bookings.FindByID(ctx, bookingID)
		if err != nil {
			return err
		}
		if clientID == "" {
			clientID = booking.ClientID
		}
		practitionerID = booking.PractitionerID
	}
	clientLink, practiceLink := "/portal/sessions", "/calendar"
	if bookingID != "" {
		clientLink = "/portal/sessions/" + bookingID + "/documents"
		practiceLink = "/calendar?booking=" + bookingID
	}
	title, body := "", ""
	clientEmail, practiceEmail := false, false
	switch e.Collection {
	case "document_requested":
		title = "Updated document requires your signature"
		clientEmail = true
	case "bookings":
		switch {
		case a["changeRequestedAt"] != "" && a["changeRequestedAt"] != b["changeRequestedAt"]:
			title = "Session change requested"
			practiceEmail = true
		case a["status"] == "cancelled" && b["status"] != "cancelled":
			title = "Session cancelled"
			if a["paymentExpired"] == "true" {
				title = "Unpaid appointment request expired"
			}
			clientEmail = true
			practiceEmail = true
		case a["status"] == "confirmed" && (b["status"] != "confirmed" || a["startAt"] != b["startAt"]):
			title = "Session confirmed"
			if b["status"] == "confirmed" {
				title = "Session rescheduled"
			}
			clientEmail = true
			practiceEmail = true
		case len(b) == 0 && a["status"] == "pending_payment":
			title = "Appointment awaiting payment"
			clientEmail = true
			clientLink = "/portal/payments#booking-" + bookingID
		case a["status"] != b["status"] && (a["status"] == "completed" || a["status"] == "no_show"):
			title = "Session marked " + strings.ReplaceAll(a["status"], "_", " ")
		case a["participantRevision"] != b["participantRevision"]:
			title = "Participant details updated — signatures required"
			clientEmail = true
		default:
			return nil
		}
	case "agreement_executions", "agreement_signatures":
		clientLink = "/portal/documents#execution-" + e.EntityID
		practiceLink = "/clients/" + clientID + "#execution-" + e.EntityID
		if len(b) == 0 {
			title = "Document signed"
			if e.Collection == "agreement_signatures" && a["submittedAt"] != "" {
				title = "Statement of Work submitted — unsigned"
			}
			practiceEmail = true
		} else if a["practitionerSignedAt"] != "" && a["practitionerSignedAt"] != b["practitionerSignedAt"] {
			title = "Agreement countersigned"
			clientEmail = true
		} else {
			return nil
		}
	case "form_submissions":
		clientLink = "/portal/forms/" + e.EntityID
		practiceLink = "/clients/" + clientID + "#submission-" + e.EntityID
		if a["status"] == "submitted" && b["status"] != "submitted" {
			title = "Form submitted"
			practiceEmail = true
		} else if len(b) == 0 {
			title = "A form requires your attention"
			clientEmail = true
		} else {
			return nil
		}
	case "documents":
		if a["visibleToClient"] != "true" && b["visibleToClient"] != "true" {
			return nil
		}
		title = "A document is ready"
		clientLink = "/portal/documents#document-" + e.EntityID
		practiceLink = "/clients/" + clientID
		if a["deleted"] == "true" || a["visibleToClient"] != "true" {
			title = "Document access updated"
			clientLink = "/portal/documents"
		} else if len(b) > 0 {
			title = "A shared document was updated"
		}
	case "session_notes":
		if a["sharedAt"] == "" || a["sharedAt"] == b["sharedAt"] {
			return nil
		}
		title = "Session feedback and resources are ready"
		clientLink = "/portal/sessions"
		practiceLink = "/clients/" + clientID
	case "session_recordings":
		if len(b) > 0 {
			return nil
		}
		title = "Session recording is ready"
		clientLink = "/portal/sessions"
		practiceLink = "/clients/" + clientID
	case "payments":
		if a["status"] == b["status"] {
			return nil
		}
		switch a["status"] {
		case "failed", "refunded":
			title = "Payment " + a["status"]
			clientEmail = true
			practiceEmail = true
		case "pending":
			title = "Complete payment to confirm your appointment"
			clientEmail = true
		default:
			return nil
		}
		clientLink = "/portal/payments#payment-" + e.EntityID
		practiceLink = "/payments#payment-" + e.EntityID
	case "enquiries":
		if len(b) > 0 {
			return nil
		}
		title = "A new enquiry requires attention"
		practiceEmail = true
		practiceLink = "/enquiries"
		clientLink = ""
	case "reviews":
		title = "A client submitted a review"
		practiceEmail = len(b) == 0
		if len(b) > 0 {
			if a["status"] != b["status"] {
				title = "Review " + strings.ReplaceAll(a["status"], "_", " ")
			} else {
				title = "A review was updated"
			}
		}
		practiceLink = "/reviews"
		clientLink = "/portal/reviews"
	default:
		return nil
	}
	if practitionerID == "" && e.Collection == "documents" && a["uploadedBy"] != "" && s.users != nil {
		u, err := s.users.FindByID(ctx, a["uploadedBy"])
		if err != nil {
			return err
		}
		if u.Role == identity.RolePractitioner {
			practitionerID = u.ID
		}
	}
	// Generic practice fallback is used only for unassigned practice-wide work.
	// A booking's assigned practitioner is always resolved by ID above.
	if practitionerID == "" && s.users != nil {
		u, err := s.users.FindFirstByRole(ctx, identity.RolePractitioner)
		if err != nil {
			return err
		}
		practitionerID = u.ID
	}
	send := func(userID, link, audience string, email bool) error {
		if userID == "" || link == "" {
			return nil
		}
		u, err := s.users.FindByID(ctx, userID)
		if err != nil {
			return err
		}
		zone := u.Timezone
		if zone == "" {
			zone = "UTC"
		}
		body = "Open your account to review this update."
		start, _ := time.Parse(time.RFC3339Nano, a["startAt"])
		old, _ := time.Parse(time.RFC3339Nano, b["startAt"])
		if !start.IsZero() {
			body = s.formatTime(start, zone) + " (" + zone + ")"
			if !old.IsZero() && !old.Equal(start) {
				body = "Previously " + s.formatTime(old, zone) + "; now " + body
			}
		}
		kind := notification.KindActivity
		if email {
			kind = notification.KindActionRequired
		}
		data := map[string]string{"eventId": "workflow:" + e.ID, "title": title, "body": body, "link": link, "audience": audience, "recipientId": u.ID, "timezone": zone}
		s.queue(ctx, kind, u.Email, bookingID, data, s.now())
		if e.Collection == "bookings" && a["status"] == "confirmed" && (b["status"] != "confirmed" || a["startAt"] != b["startAt"]) {
			// Replacement reminder identity includes the committed transition. Dispatch
			// verifies the current booking to reject stale/cancelled reminders.
			if due, ok := notification.ReminderDueAt(start, s.lead, s.now()); ok {
				reminder := map[string]string{"eventId": "workflow:" + e.ID + ":reminder", "recipientId": u.ID, "audience": audience, "startAt": a["startAt"], "timezone": zone, "timeUntil": humanLead(s.lead), "startTime": s.formatTime(start, zone), "clientName": u.Name, "serviceName": "Your consultation"}
				s.queue(ctx, notification.KindSessionReminder, u.Email, bookingID, reminder, due)
			}
		}
		return nil
	}
	if s.users == nil {
		return fmt.Errorf("workflow user repository unavailable")
	}
	if err := send(clientID, clientLink, "client", clientEmail); err != nil {
		return err
	}
	return send(practitionerID, practiceLink, "practitioner", practiceEmail)
}
