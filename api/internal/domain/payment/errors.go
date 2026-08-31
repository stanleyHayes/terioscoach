package payment

import "errors"

// Domain errors for the payments slice. Adapters and the HTTP layer map
// these to storage results and status codes via errors.Is.
var (
	// ErrPaymentNotFound means no payment matches the lookup key (unknown
	// id, or owned by someone else — no cross-tenant leak).
	ErrPaymentNotFound = errors.New("payment not found")
	// ErrAlreadyPaid means the booking's payment already succeeded — a
	// second initialize is refused.
	ErrAlreadyPaid = errors.New("booking is already paid")
	// ErrInvalidWebhookSignature means the Stripe-Signature header did
	// not match the HMAC-SHA512 of the raw body under the secret key.
	ErrInvalidWebhookSignature = errors.New("invalid webhook signature")
	// ErrInvalidTransition means a lifecycle change was attempted from a
	// state that does not allow it (e.g. refunding a non-success payment).
	ErrInvalidTransition = errors.New("payment cannot change status from its current state")
	// ErrInvalidAmount means a negative amount in minor units was supplied.
	ErrInvalidAmount = errors.New("amount must be non-negative minor units")
	// ErrInvalidCurrency means a non ISO 4217 alphabetic code was supplied.
	ErrInvalidCurrency = errors.New("currency must be a 3-letter code")
	// ErrReferenceRequired means a payment was built without its
	// server-generated provider reference.
	ErrReferenceRequired = errors.New("payment reference is required")
)
