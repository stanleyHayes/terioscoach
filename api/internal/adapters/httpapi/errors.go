package httpapi

import (
	"errors"
	"net/http"

	reportsapp "github.com/xcreativs/terios/api/internal/app/reports"
	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/catalog"
	"github.com/xcreativs/terios/api/internal/domain/client"
	"github.com/xcreativs/terios/api/internal/domain/cms"
	"github.com/xcreativs/terios/api/internal/domain/document"
	"github.com/xcreativs/terios/api/internal/domain/enquiry"
	"github.com/xcreativs/terios/api/internal/domain/form"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/note"
	"github.com/xcreativs/terios/api/internal/domain/payment"
	"github.com/xcreativs/terios/api/internal/domain/review"
	"github.com/xcreativs/terios/api/internal/domain/scheduling"
	domainsignaling "github.com/xcreativs/terios/api/internal/domain/signaling"
	"github.com/xcreativs/terios/api/internal/ports"
)

// errorBody is the consistent error shape every endpoint returns:
// {"error": {"code": "...", "message": "..."}}.
type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// writeError renders the standard error envelope.
func writeError(w http.ResponseWriter, status int, code, message string) {
	var body errorBody
	body.Error.Code = code
	body.Error.Message = message
	writeJSON(w, status, body)
}

// apiError is one mapped domain error: the status, the stable code the
// contract publishes, and the human-readable message.
type apiError struct {
	status  int
	code    string
	message string
}

// validationError is the shared 400 shape. The domain error's own text is
// the message: every one of them is written for a person to read.
func validationError(err error) (apiError, bool) {
	return apiError{http.StatusBadRequest, "validation_error", err.Error()}, true
}

// errorMappers is the per-slice mapping chain. Each function reports
// whether it recognised the error; the first that does wins.
//
// Splitting the mapping this way keeps each slice's error vocabulary
// together and readable as the API grows — a single switch over every
// domain error in the system stops being reviewable long before it stops
// compiling.
var errorMappers = []func(error) (apiError, bool){
	mapIdentityError,
	mapCatalogError,
	mapSchedulingError,
	mapBookingError,
	mapPaymentError,
	mapClientError,
	mapNoteError,
	mapCMSError,
	mapEnquiryError,
	mapReviewError,
	mapFormError,
	mapDocumentError,
	mapReportError,
	mapSignalingError,
}

// writeDomainError maps domain errors onto status codes and stable codes.
// Unknown errors collapse to a generic 500 — internals never leak.
func writeDomainError(w http.ResponseWriter, err error) {
	// A domain error may carry a cooldown (brute-force lockout, BE-02).
	// The header goes on before the body is written.
	var retryErr *identity.RetryAfterError
	if errors.As(err, &retryErr) {
		writeRetryAfter(w, retryErr.RetryAfter)
	}

	for _, mapper := range errorMappers {
		if mapped, ok := mapper(err); ok {
			writeError(w, mapped.status, mapped.code, mapped.message)
			return
		}
	}

	// A gateway failure is the one cross-slice case: any outbound provider
	// (Stripe, Resend) reports through the same typed error.
	var gatewayErr *ports.GatewayError
	if errors.As(err, &gatewayErr) {
		writeError(w, http.StatusBadGateway, "payment_gateway_error", "the payment gateway could not complete the request")
		return
	}
	writeError(w, http.StatusInternalServerError, "internal_error", "something went wrong")
}

func mapIdentityError(err error) (apiError, bool) {
	switch {
	case errors.Is(err, identity.ErrTooManyAttempts):
		// Deliberately identical for real and unknown emails: the lockout
		// must not become an account-existence oracle.
		return apiError{http.StatusTooManyRequests, "too_many_attempts",
			"too many failed sign-in attempts — please wait before trying again"}, true
	case errors.Is(err, identity.ErrInvalidCredentials):
		// Uniform message on purpose: no user enumeration.
		return apiError{http.StatusUnauthorized, "invalid_credentials", "invalid email or password"}, true
	case errors.Is(err, identity.ErrAccountDisabled):
		return apiError{http.StatusForbidden, "account_disabled", "this staff account has been disabled"}, true
	case errors.Is(err, identity.ErrLastOwner):
		return apiError{http.StatusConflict, "owner_protected", err.Error()}, true
	case errors.Is(err, identity.ErrUserNotFound):
		return apiError{http.StatusNotFound, "user_not_found", "staff member not found"}, true
	case errors.Is(err, identity.ErrMFARequired):
		return apiError{http.StatusUnauthorized, "mfa_required", "enter the code from your authenticator app"}, true
	case errors.Is(err, identity.ErrMFAInvalid):
		return apiError{http.StatusUnauthorized, "mfa_invalid", "that authenticator code is invalid or expired"}, true
	case errors.Is(err, identity.ErrMFANotPending):
		return apiError{http.StatusConflict, "mfa_not_pending", "start MFA enrollment before confirming it"}, true
	case errors.Is(err, identity.ErrPasswordResetInvalid):
		return apiError{http.StatusBadRequest, "password_reset_invalid", "this password reset link is invalid or has expired"}, true
	case errors.Is(err, identity.ErrEmailTaken):
		return apiError{http.StatusConflict, "email_taken", "an account with this email already exists"}, true
	case errors.Is(err, identity.ErrTokenExpired):
		return apiError{http.StatusUnauthorized, "token_expired", "token has expired"}, true
	case errors.Is(err, identity.ErrTokenInvalid):
		return apiError{http.StatusUnauthorized, "token_invalid", "token is invalid"}, true
	case errors.Is(err, identity.ErrInvalidEmail):
		return apiError{http.StatusBadRequest, "validation_error", "a valid email address is required"}, true
	case errors.Is(err, identity.ErrPasswordTooShort):
		return apiError{http.StatusBadRequest, "validation_error", "password must be at least 12 characters"}, true
	case errors.Is(err, identity.ErrNameRequired):
		return apiError{http.StatusBadRequest, "validation_error", "name is required"}, true
	case errors.Is(err, identity.ErrCurrentPassword):
		return apiError{http.StatusBadRequest, "current_password_invalid", "current password is incorrect"}, true
	case errors.Is(err, identity.ErrInvalidRole), errors.Is(err, identity.ErrInvalidPermission):
		return validationError(err)
	}
	return apiError{}, false
}

func mapCatalogError(err error) (apiError, bool) {
	switch {
	case errors.Is(err, catalog.ErrServiceNotFound):
		return apiError{http.StatusNotFound, "service_not_found", "service not found"}, true
	case errors.Is(err, catalog.ErrInvalidName),
		errors.Is(err, catalog.ErrInvalidDuration),
		errors.Is(err, catalog.ErrInvalidPrice),
		errors.Is(err, catalog.ErrInvalidCurrency),
		errors.Is(err, catalog.ErrInvalidImageURL):
		return validationError(err)
	}
	return apiError{}, false
}

func mapSchedulingError(err error) (apiError, bool) {
	switch {
	case errors.Is(err, scheduling.ErrInvalidTimezone):
		return apiError{http.StatusBadRequest, "invalid_timezone", "tz must be a valid IANA timezone name"}, true
	case errors.Is(err, scheduling.ErrInvalidWeekday),
		errors.Is(err, scheduling.ErrInvalidWindow),
		errors.Is(err, scheduling.ErrInvalidBuffer),
		errors.Is(err, scheduling.ErrDuplicateWeekday),
		errors.Is(err, scheduling.ErrInvalidTimeOffRange),
		errors.Is(err, scheduling.ErrInvalidDuration),
		errors.Is(err, scheduling.ErrInvalidRange):
		return validationError(err)
	}
	return apiError{}, false
}

func mapBookingError(err error) (apiError, bool) {
	switch {
	case errors.Is(err, booking.ErrBookingNotFound):
		return apiError{http.StatusNotFound, "booking_not_found", "booking not found"}, true
	case errors.Is(err, booking.ErrSlotUnavailable):
		return apiError{http.StatusConflict, "slot_unavailable", "that slot is no longer available"}, true
	case errors.Is(err, booking.ErrCutoffPassed):
		return apiError{http.StatusUnprocessableEntity, "cutoff_passed", "the modification cutoff for this booking has passed"}, true
	case errors.Is(err, booking.ErrInvalidTransition), errors.Is(err, booking.ErrTooEarly):
		return apiError{http.StatusConflict, "invalid_status", err.Error()}, true
	case errors.Is(err, booking.ErrInvalidDuration):
		return validationError(err)
	}
	return apiError{}, false
}

func mapPaymentError(err error) (apiError, bool) {
	switch {
	case errors.Is(err, payment.ErrPaymentNotFound):
		return apiError{http.StatusNotFound, "payment_not_found", "payment not found"}, true
	case errors.Is(err, payment.ErrAlreadyPaid):
		return apiError{http.StatusConflict, "already_paid", "booking is already paid"}, true
	case errors.Is(err, payment.ErrInvalidWebhookSignature):
		return apiError{http.StatusUnauthorized, "invalid_signature", "webhook signature is invalid"}, true
	case errors.Is(err, payment.ErrInvalidTransition):
		return apiError{http.StatusConflict, "invalid_status", err.Error()}, true
	case errors.Is(err, payment.ErrInvalidAmount),
		errors.Is(err, payment.ErrInvalidCurrency),
		errors.Is(err, payment.ErrReferenceRequired):
		return validationError(err)
	}
	return apiError{}, false
}

func mapClientError(err error) (apiError, bool) {
	switch {
	case errors.Is(err, client.ErrClientNotFound), errors.Is(err, client.ErrProfileNotFound):
		return apiError{http.StatusNotFound, "client_not_found", "client not found"}, true
	case errors.Is(err, client.ErrPhoneTooLong),
		errors.Is(err, client.ErrPracticeNotesTooLong),
		errors.Is(err, client.ErrTooManyTags),
		errors.Is(err, client.ErrTagTooLong):
		return validationError(err)
	}
	return apiError{}, false
}

func mapNoteError(err error) (apiError, bool) {
	switch {
	case errors.Is(err, note.ErrNoteNotFound):
		return apiError{http.StatusNotFound, "note_not_found", "session note not found"}, true
	case errors.Is(err, note.ErrNoteExists):
		return apiError{http.StatusConflict, "note_exists", "a session note already exists for this booking"}, true
	case errors.Is(err, note.ErrPrivateNotesTooLong),
		errors.Is(err, note.ErrSharedFeedbackTooLong),
		errors.Is(err, note.ErrTooManyResources),
		errors.Is(err, note.ErrResourceTooLong):
		return validationError(err)
	}
	return apiError{}, false
}

func mapCMSError(err error) (apiError, bool) {
	switch {
	case errors.Is(err, cms.ErrPageNotFound):
		return apiError{http.StatusNotFound, "page_not_found", "page not found"}, true
	case errors.Is(err, cms.ErrPostNotFound):
		return apiError{http.StatusNotFound, "post_not_found", "post not found"}, true
	case errors.Is(err, cms.ErrFAQNotFound):
		return apiError{http.StatusNotFound, "faq_not_found", "faq not found"}, true
	case errors.Is(err, cms.ErrTestimonialNotFound):
		return apiError{http.StatusNotFound, "testimonial_not_found", "testimonial not found"}, true
	case errors.Is(err, cms.ErrSlugTaken):
		return apiError{http.StatusConflict, "slug_taken", "that slug is already in use"}, true
	case errors.Is(err, cms.ErrInvalidSlug),
		errors.Is(err, cms.ErrSlugTooLong),
		errors.Is(err, cms.ErrInvalidTitle),
		errors.Is(err, cms.ErrTitleTooLong),
		errors.Is(err, cms.ErrExcerptTooLong),
		errors.Is(err, cms.ErrBodyTooLong),
		errors.Is(err, cms.ErrCategoryTooLong),
		errors.Is(err, cms.ErrTooManyTags),
		errors.Is(err, cms.ErrTagTooLong),
		errors.Is(err, cms.ErrInvalidQuestion),
		errors.Is(err, cms.ErrInvalidAnswer),
		errors.Is(err, cms.ErrInvalidAuthor),
		errors.Is(err, cms.ErrInvalidQuote),
		errors.Is(err, cms.ErrInvalidURL),
		errors.Is(err, cms.ErrURLTooLong):
		return validationError(err)
	}
	return apiError{}, false
}

func mapEnquiryError(err error) (apiError, bool) {
	switch {
	case errors.Is(err, enquiry.ErrEnquiryNotFound):
		return apiError{http.StatusNotFound, "enquiry_not_found", "enquiry not found"}, true
	case errors.Is(err, enquiry.ErrInvalidStatus):
		return apiError{http.StatusBadRequest, "validation_error",
			"status must be new, read, replied, or archived"}, true
	case errors.Is(err, enquiry.ErrInvalidName),
		errors.Is(err, enquiry.ErrInvalidEmail),
		errors.Is(err, enquiry.ErrPhoneTooLong),
		errors.Is(err, enquiry.ErrSubjectTooLong),
		errors.Is(err, enquiry.ErrInvalidMessage):
		return validationError(err)
	}
	return apiError{}, false
}

func mapReviewError(err error) (apiError, bool) {
	switch {
	case errors.Is(err, review.ErrReviewNotFound):
		return apiError{http.StatusNotFound, "review_not_found", "review not found"}, true
	case errors.Is(err, review.ErrReviewExists):
		return apiError{http.StatusConflict, "review_exists", "this session has already been reviewed"}, true
	case errors.Is(err, review.ErrAlreadyModerated):
		return apiError{http.StatusConflict, "already_moderated",
			"this review has been moderated and can no longer be edited"}, true
	case errors.Is(err, review.ErrSessionNotComplete):
		return apiError{http.StatusUnprocessableEntity, "session_not_complete",
			"only a completed session can be reviewed"}, true
	case errors.Is(err, review.ErrInvalidRating),
		errors.Is(err, review.ErrCommentTooLong),
		errors.Is(err, review.ErrInvalidBooking),
		errors.Is(err, review.ErrInvalidStatus):
		return validationError(err)
	}
	return apiError{}, false
}

func mapFormError(err error) (apiError, bool) {
	switch {
	case errors.Is(err, form.ErrFormNotFound):
		return apiError{http.StatusNotFound, "form_not_found", "form not found"}, true
	case errors.Is(err, form.ErrSubmissionNotFound):
		return apiError{http.StatusNotFound, "submission_not_found", "form submission not found"}, true
	case errors.Is(err, form.ErrAlreadySubmitted):
		return apiError{http.StatusConflict, "already_submitted", "this form has already been submitted"}, true
	case errors.Is(err, form.ErrAlreadyAssigned):
		return apiError{http.StatusConflict, "already_assigned", "this form is already waiting for the client"}, true
	case errors.Is(err, form.ErrFormInactive):
		return apiError{http.StatusConflict, "form_inactive", "this form is no longer in use"}, true
	case errors.Is(err, form.ErrSignatureRequired):
		return apiError{http.StatusUnprocessableEntity, "signature_required", "this form must be signed"}, true
	case errors.Is(err, form.ErrRequiredFieldMissing),
		errors.Is(err, form.ErrUnknownField),
		errors.Is(err, form.ErrInvalidOptionAnswer),
		errors.Is(err, form.ErrInvalidNumber),
		errors.Is(err, form.ErrInvalidDate),
		errors.Is(err, form.ErrAnswerTooLong),
		errors.Is(err, form.ErrInvalidSignatureName),
		errors.Is(err, form.ErrInvalidSignatureImage),
		errors.Is(err, form.ErrSignatureTooLarge),
		errors.Is(err, form.ErrFormMismatch),
		errors.Is(err, form.ErrInvalidClient),
		errors.Is(err, form.ErrInvalidTitle),
		errors.Is(err, form.ErrDescriptionTooLong),
		errors.Is(err, form.ErrNoFields),
		errors.Is(err, form.ErrTooManyFields),
		errors.Is(err, form.ErrInvalidFieldKey),
		errors.Is(err, form.ErrDuplicateFieldKey),
		errors.Is(err, form.ErrInvalidFieldLabel),
		errors.Is(err, form.ErrInvalidFieldType),
		errors.Is(err, form.ErrHelpTextTooLong),
		errors.Is(err, form.ErrOptionsRequired),
		errors.Is(err, form.ErrOptionsNotAllowed),
		errors.Is(err, form.ErrTooManyOptions),
		errors.Is(err, form.ErrInvalidOption),
		errors.Is(err, form.ErrDuplicateOption):
		return validationError(err)
	}
	return apiError{}, false
}

func mapDocumentError(err error) (apiError, bool) {
	switch {
	case errors.Is(err, document.ErrDocumentNotFound):
		return apiError{http.StatusNotFound, "document_not_found", "document not found"}, true
	case errors.Is(err, document.ErrUnsupportedFileType):
		return apiError{http.StatusUnsupportedMediaType, "unsupported_file_type",
			"only pdf, jpg, png, and webp files are accepted"}, true
	case errors.Is(err, document.ErrFileTooLarge):
		return apiError{http.StatusRequestEntityTooLarge, "file_too_large", "file is too large"}, true
	case errors.Is(err, document.ErrInvalidKind),
		errors.Is(err, document.ErrClientRequired),
		errors.Is(err, document.ErrUploaderRequired),
		errors.Is(err, document.ErrInvalidPublicID),
		errors.Is(err, document.ErrInvalidFilename),
		errors.Is(err, document.ErrInvalidTitle):
		return validationError(err)
	}
	return apiError{}, false
}

func mapReportError(err error) (apiError, bool) {
	switch {
	case errors.Is(err, reportsapp.ErrInvalidRange),
		errors.Is(err, reportsapp.ErrRangeTooLong),
		errors.Is(err, reportsapp.ErrInvalidGranularity):
		return validationError(err)
	}
	return apiError{}, false
}

func mapSignalingError(err error) (apiError, bool) {
	switch {
	case errors.Is(err, domainsignaling.ErrRoomNotOpenYet):
		return apiError{http.StatusForbidden, "room_not_open", "the video room is not open yet"}, true
	case errors.Is(err, domainsignaling.ErrRoomClosed):
		return apiError{http.StatusForbidden, "room_closed", "the video room for this session has closed"}, true
	case errors.Is(err, domainsignaling.ErrSessionNotActive):
		return apiError{http.StatusConflict, "invalid_status", "this session is not active"}, true
	case errors.Is(err, domainsignaling.ErrRoomFull):
		return apiError{http.StatusConflict, "room_full", "this session already has both participants"}, true
	case errors.Is(err, domainsignaling.ErrTicketInvalid):
		return apiError{http.StatusUnauthorized, "ticket_invalid", "connection ticket is invalid or expired"}, true
	}
	return apiError{}, false
}
