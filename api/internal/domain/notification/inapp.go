package notification

import "time"

// InApp is a durable recipient-scoped feed entry. EventID makes delivery
// retries idempotent. Email delivery and read state are independent.
type InApp struct {
	EventID        string
	ID             string
	RecipientEmail string
	Kind           Kind
	Title          string
	Body           string
	Link           string
	BookingID      string
	Read           bool
	CreatedAt      time.Time
}

// NewInApp builds an unread notification for recipientEmail. A kind outside
// the known set or a missing title are both refused: either means the
// caller built the entry from a job it does not know how to present.
func NewInApp(recipientEmail string, kind Kind, title, body, link, bookingID string, now time.Time) (InApp, error) {
	if recipientEmail == "" {
		return InApp{}, ErrRecipientRequired
	}
	if !kind.Valid() {
		return InApp{}, ErrInvalidKind
	}
	if title == "" {
		return InApp{}, ErrTitleRequired
	}
	return InApp{
		RecipientEmail: recipientEmail,
		Kind:           kind,
		Title:          title,
		Body:           body,
		Link:           link,
		BookingID:      bookingID,
		Read:           false,
		CreatedAt:      now.UTC(),
	}, nil
}

// MarkRead flips the notification to read. It is idempotent — reading an
// already-read notification again is not an error.
func (n *InApp) MarkRead() {
	n.Read = true
}
