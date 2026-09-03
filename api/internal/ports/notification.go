package ports

import (
	"context"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/notification"
)

// EmailMessage is one rendered message handed to a mail provider. The body
// is final HTML: templating happened before this boundary, so swapping the
// provider never touches the templates.
type EmailMessage struct {
	To      string
	Subject string
	HTML    string
	// Text is the plain-text alternative. Providers that support
	// multipart use it; the rest ignore it.
	Text string
}

// Mailer is the outbound port for a transactional email provider (Resend).
type Mailer interface {
	// Send delivers one message. A transient provider failure should be
	// reported as a *GatewayError so the caller can retry it.
	Send(ctx context.Context, msg EmailMessage) error
}

// EmailRenderer is the outbound port for turning a queued job into a
// message. It owns the brand templates and the subject lines.
type EmailRenderer interface {
	// Render produces the message for a job. An unknown kind returns
	// notification.ErrTemplateNotFound.
	Render(job notification.Job) (EmailMessage, error)
}

// NotificationJobRepository is the outbound port for the delivery outbox.
type NotificationJobRepository interface {
	// Create persists a new job, assigning its ID.
	Create(ctx context.Context, job notification.Job) (notification.Job, error)
	// Update persists lifecycle changes (sent, failed, rescheduled).
	Update(ctx context.Context, job notification.Job) (notification.Job, error)
	// ClaimDue atomically takes up to limit pending jobs that are due,
	// so two dispatcher instances never send the same email twice. The
	// returned jobs are already marked in-flight for this caller.
	ClaimDue(ctx context.Context, now time.Time, limit int) ([]notification.Job, error)
	// PendingByBooking lists a booking's undelivered jobs of one kind —
	// how a reschedule finds the reminder it must move.
	PendingByBooking(ctx context.Context, bookingID string, kind notification.Kind) ([]notification.Job, error)
}

// BookingNotice is everything the booking emails need, resolved by the
// caller: names and times rather than ids, because the message may be sent
// long after the booking that caused it.
type BookingNotice struct {
	BookingID   string
	ClientName  string
	ClientEmail string
	ServiceName string
	StartAt     time.Time
	// PreviousStartAt is set only on a reschedule notice.
	PreviousStartAt time.Time
	// Timezone is the IANA name the times are presented in.
	Timezone string
	// PaymentURL is populated only for the payment-required message.
	PaymentURL string
}

// FeedbackNotice is what the post-session feedback email needs.
type FeedbackNotice struct {
	BookingID        string
	ClientName       string
	ClientEmail      string
	PractitionerName string
	ServiceName      string
	SessionDate      time.Time
	HasResources     bool
	Timezone         string
}

// EnquiryNotice is what the practitioner's new-enquiry alert needs.
type EnquiryNotice struct {
	EnquiryID   string
	SenderName  string
	SenderEmail string
	Message     string
}

// Notifier is the inbound port the other slices call when something
// happened that a person should hear about.
//
// The methods return no error on purpose. Queueing a message is a
// side-effect of a business event that has already succeeded: a booking is
// booked whether or not its confirmation email queues, and failing the
// booking because the mail outbox hiccupped would be worse than the
// missing email. The implementation is therefore responsible for its own
// durability and for reporting its own failures (see notifications.Service,
// which reports through an injected sink).
type Notifier interface {
	BookingPaymentRequired(ctx context.Context, notice BookingNotice)
	BookingConfirmed(ctx context.Context, notice BookingNotice)
	BookingRescheduled(ctx context.Context, notice BookingNotice)
	BookingCancelled(ctx context.Context, notice BookingNotice)
	FeedbackShared(ctx context.Context, notice FeedbackNotice)
	EnquiryReceived(ctx context.Context, notice EnquiryNotice)
}

// DispatchResult reports one pass of the dispatcher.
type DispatchResult struct {
	Sent   int
	Failed int
}

// Dispatcher is the inbound port the scheduler drives: deliver whatever is
// due. It is separate from Notifier so the process that queues messages and
// the process that sends them can be different ones.
type Dispatcher interface {
	// DispatchDue delivers up to limit due jobs and reports the outcome.
	DispatchDue(ctx context.Context, limit int) (DispatchResult, error)
}
