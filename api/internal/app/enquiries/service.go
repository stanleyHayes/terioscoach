// Package enquiries is the application service for the contact-form slice.
// It implements the inbound ports.EnquiryService port purely against
// outbound ports — no framework, driver, or transport imports.
package enquiries

import (
	"context"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/enquiry"
	"github.com/xcreativs/terios/api/internal/ports"
)

// Service orchestrates the enquiry use cases over outbound ports.
type Service struct {
	enquiries ports.EnquiryRepository
	notifier  ports.Notifier
	now       func() time.Time
}

// Compile-time check: Service satisfies the inbound port.
var _ ports.EnquiryService = (*Service)(nil)

// Option customizes a Service.
type Option func(*Service)

// WithNotifications alerts the practice when an enquiry arrives (BE-09).
// The alert goes to the practice inbox, never to the address the sender
// typed — see notifications.Service.EnquiryReceived.
func WithNotifications(notifier ports.Notifier) Option {
	return func(s *Service) { s.notifier = notifier }
}

// NewService wires the use cases to their outbound ports.
func NewService(enquiries ports.EnquiryRepository, opts ...Option) *Service {
	s := &Service{
		enquiries: enquiries,
		now:       func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Submit records an enquiry and alerts the practice.
//
// The alert is queued after the enquiry is stored, and its failure never
// fails the submission: a visitor who filled in the form has done nothing
// wrong, and the message is safe in the inbox whether or not the email goes
// out. The notifications outbox is what makes that safe rather than lossy.
func (s *Service) Submit(ctx context.Context, in ports.EnquiryInput) (enquiry.Enquiry, error) {
	e, err := enquiry.New(in.Name, in.Email, in.Phone, in.Subject, in.Message, s.now())
	if err != nil {
		return enquiry.Enquiry{}, err
	}
	e.SourceIP = in.SourceIP

	e, err = s.enquiries.Create(ctx, e)
	if err != nil {
		return enquiry.Enquiry{}, err
	}

	if s.notifier != nil {
		s.notifier.EnquiryReceived(ctx, ports.EnquiryNotice{
			EnquiryID:   e.ID,
			SenderName:  e.Name,
			SenderEmail: e.Email,
			Message:     e.Message,
		})
	}
	return e, nil
}

// List returns the practice inbox, newest first.
func (s *Service) List(ctx context.Context, filter ports.EnquiryFilter) ([]enquiry.Enquiry, error) {
	if filter.Status != "" && !filter.Status.Valid() {
		return nil, enquiry.ErrInvalidStatus
	}
	return s.enquiries.List(ctx, filter)
}

// UnreadCount is the inbox badge: how many enquiries nobody has opened.
func (s *Service) UnreadCount(ctx context.Context) (int, error) {
	return s.enquiries.CountByStatus(ctx, enquiry.StatusNew)
}

// Get returns one enquiry.
func (s *Service) Get(ctx context.Context, id string) (enquiry.Enquiry, error) {
	return s.enquiries.FindByID(ctx, id)
}

// SetStatus triages one enquiry.
func (s *Service) SetStatus(ctx context.Context, id string, status enquiry.Status) (enquiry.Enquiry, error) {
	e, err := s.enquiries.FindByID(ctx, id)
	if err != nil {
		return enquiry.Enquiry{}, err
	}
	if err := e.SetStatus(status, s.now()); err != nil {
		return enquiry.Enquiry{}, err
	}
	return s.enquiries.Update(ctx, e)
}

// Delete removes an enquiry. Deleting one that is not there is a not-found
// rather than a silent success, so the inbox UI can say so.
func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := s.enquiries.FindByID(ctx, id); err != nil {
		return err
	}
	return s.enquiries.Delete(ctx, id)
}
