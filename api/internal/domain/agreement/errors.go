package agreement

import "errors"

// Domain errors for the agreements slice.
var (
	// Lookup.
	ErrAgreementNotFound = errors.New("agreement not found")
	ErrAgreementInactive = errors.New("this agreement is no longer in use")
	// ErrAlreadySigned is not a failure the client ever sees: signing is
	// idempotent, and a repeat returns the signature already on file. It
	// exists for the storage layer to report a unique-index collision with.
	ErrAlreadySigned = errors.New("this agreement has already been signed")

	// ErrAgreementRequired is the booking gate: the service being booked is
	// covered by an agreement this client has not signed yet.
	ErrAgreementRequired = errors.New("this service requires a signed agreement")

	// Validation.
	ErrInvalidStatementOfWork = errors.New("complete the statement of work with client name, effective date (YYYY-MM-DD), initial term (1-1200 months), and monthly fee; no signature is required")
	ErrInvalidAgreement       = errors.New("a practitioner and key are required")
	ErrInvalidTitle           = errors.New("title is required")
	ErrTitleTooLong           = errors.New("title is too long")
	ErrInvalidBody            = errors.New("agreement text is required")
	ErrBodyTooLong            = errors.New("agreement text is too long")
	ErrInvalidClient          = errors.New("a client is required")
	ErrInvalidSignedName      = errors.New("type your full name to sign")
	ErrSignedNameTooLong      = errors.New("that name is too long")

	// Countersignature & Collections
	ErrCountersignatureNotRequired = errors.New("only holistic and nurse coaching agreements require a practitioner countersignature")
	ErrAlreadyCountersigned        = errors.New("this agreement has already been countersigned")
	ErrSignatureNotRequired        = errors.New("statement of work documents are fill-in forms and do not require a client signature")
	ErrSignatureNotFound           = errors.New("signature not found")
	ErrCollectionNotFound          = errors.New("agreement collection not found")
	ErrInvalidCollection           = errors.New("invalid agreement collection")
)
