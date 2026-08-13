package form

import "errors"

// Domain errors for the forms slice.
var (
	// Lookup errors. A submission that belongs to someone else is reported
	// as missing, never as forbidden — the same isolation rule the rest of
	// the client-scoped API follows.
	ErrFormNotFound       = errors.New("form not found")
	ErrSubmissionNotFound = errors.New("form submission not found")
	ErrFormInactive       = errors.New("form is no longer in use")
	ErrFormMismatch       = errors.New("answers do not belong to this form")
	ErrAlreadySubmitted   = errors.New("this form has already been submitted")
	ErrAlreadyAssigned    = errors.New("this form is already assigned to the client")

	// Form-definition validation.
	ErrInvalidTitle       = errors.New("title is required")
	ErrDescriptionTooLong = errors.New("description is too long")
	ErrNoFields           = errors.New("a form needs at least one field")
	ErrTooManyFields      = errors.New("too many fields")
	ErrInvalidFieldKey    = errors.New("each field needs a key of letters or digits")
	ErrDuplicateFieldKey  = errors.New("field keys must be unique")
	ErrInvalidFieldLabel  = errors.New("each field needs a label")
	ErrInvalidFieldType   = errors.New("unknown field type")
	ErrHelpTextTooLong    = errors.New("help text is too long")
	ErrOptionsRequired    = errors.New("choice fields need at least one option")
	ErrOptionsNotAllowed  = errors.New("only choice fields may carry options")
	ErrTooManyOptions     = errors.New("too many options")
	ErrInvalidOption      = errors.New("each option needs text")
	ErrDuplicateOption    = errors.New("options must be unique")

	// Submission validation.
	ErrInvalidClient        = errors.New("a client is required")
	ErrUnknownField         = errors.New("answer given for a field that is not on this form")
	ErrRequiredFieldMissing = errors.New("a required field was left blank")
	ErrAnswerTooLong        = errors.New("answer is too long")
	ErrInvalidNumber        = errors.New("value must be a number")
	ErrInvalidDate          = errors.New("value must be a date (YYYY-MM-DD)")
	ErrInvalidOptionAnswer  = errors.New("value is not one of the field's options")

	// Signature validation.
	ErrSignatureRequired     = errors.New("this form must be signed")
	ErrInvalidSignatureName  = errors.New("a typed name is required to sign")
	ErrInvalidSignatureImage = errors.New("signature must be an inline PNG image")
	ErrSignatureTooLarge     = errors.New("signature image is too large")
)
