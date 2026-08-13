package enquiry

import "errors"

// Domain errors for the enquiries slice.
var (
	// ErrEnquiryNotFound means no enquiry matches the lookup.
	ErrEnquiryNotFound = errors.New("enquiry not found")
	// ErrInvalidStatus means a triage state outside the known set.
	ErrInvalidStatus = errors.New("invalid enquiry status")

	// Validation errors.
	ErrInvalidName    = errors.New("name is required")
	ErrInvalidEmail   = errors.New("a valid email address is required")
	ErrPhoneTooLong   = errors.New("phone number is too long")
	ErrSubjectTooLong = errors.New("subject is too long")
	ErrInvalidMessage = errors.New("message is required")
)
