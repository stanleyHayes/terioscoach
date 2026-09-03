package catalog

import "errors"

// Domain errors for the catalog slice. Adapters and the HTTP layer map
// these to storage results and status codes via errors.Is.
var (
	// ErrServiceNotFound means no service matches the lookup key (unknown,
	// soft-deleted, or owned by another practitioner — no cross-tenant leak).
	ErrServiceNotFound = errors.New("service not found")
	// ErrInvalidName means the name was empty or over the length limit.
	ErrInvalidName = errors.New("name is required (1-200 characters)")
	// ErrInvalidDuration means the duration fell outside the allowed bounds.
	ErrInvalidDuration = errors.New("duration must be between 5 and 480 minutes")
	// ErrInvalidPrice means a negative price in minor units was supplied.
	ErrInvalidPrice = errors.New("price must be zero or greater")
	// ErrInvalidCurrency means a non ISO 4217 currency code was supplied.
	ErrInvalidCurrency = errors.New("currency must be a 3-letter ISO 4217 code")
	// ErrInvalidImageURL means the service image is neither local nor HTTP(S).
	ErrInvalidImageURL = errors.New("image URL must be a local path or an HTTP(S) URL")
)
