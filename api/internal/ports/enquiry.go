package ports

import (
	"context"

	"github.com/xcreativs/terios/api/internal/domain/enquiry"
)

// EnquiryFilter narrows the practice inbox listing.
type EnquiryFilter struct {
	// Status, when set, narrows to one triage state.
	Status enquiry.Status
}

// EnquiryRepository is the outbound port for the enquiry inbox.
type EnquiryRepository interface {
	Create(ctx context.Context, e enquiry.Enquiry) (enquiry.Enquiry, error)
	Update(ctx context.Context, e enquiry.Enquiry) (enquiry.Enquiry, error)
	Delete(ctx context.Context, id string) error
	// FindByID misses return enquiry.ErrEnquiryNotFound.
	FindByID(ctx context.Context, id string) (enquiry.Enquiry, error)
	// List returns the inbox newest-first.
	List(ctx context.Context, filter EnquiryFilter) ([]enquiry.Enquiry, error)
	// CountByStatus backs the inbox's unread badge.
	CountByStatus(ctx context.Context, status enquiry.Status) (int, error)
}

// EnquiryInput is the contact-form payload. SourceIP is filled in by the
// transport, never by the sender.
type EnquiryInput struct {
	Name     string
	Email    string
	Phone    string
	Subject  string
	Message  string
	SourceIP string
}

// EnquiryService is the inbound port for the enquiries slice (BE-13).
type EnquiryService interface {
	// Submit records a contact-form enquiry and alerts the practice. It is
	// the only method an anonymous caller can reach.
	Submit(ctx context.Context, in EnquiryInput) (enquiry.Enquiry, error)
	// List returns the practice inbox — practitioner-only.
	List(ctx context.Context, filter EnquiryFilter) ([]enquiry.Enquiry, error)
	// UnreadCount is the inbox badge — practitioner-only.
	UnreadCount(ctx context.Context) (int, error)
	// Get returns one enquiry — practitioner-only.
	Get(ctx context.Context, id string) (enquiry.Enquiry, error)
	// SetStatus triages one enquiry — practitioner-only.
	SetStatus(ctx context.Context, id string, status enquiry.Status) (enquiry.Enquiry, error)
	// Delete removes an enquiry — practitioner-only.
	Delete(ctx context.Context, id string) error
}
