package httpapi

import (
	"errors"
	"net/http"

	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/catalog"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/payment"
	"github.com/xcreativs/terios/api/internal/domain/scheduling"
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

// writeDomainError maps domain errors onto status codes and stable codes.
// Unknown errors collapse to a generic 500 — internals never leak.
func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, identity.ErrInvalidCredentials):
		// Uniform message on purpose: no user enumeration.
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
	case errors.Is(err, identity.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email_taken", "an account with this email already exists")
	case errors.Is(err, identity.ErrTokenExpired):
		writeError(w, http.StatusUnauthorized, "token_expired", "token has expired")
	case errors.Is(err, identity.ErrTokenInvalid):
		writeError(w, http.StatusUnauthorized, "token_invalid", "token is invalid")
	case errors.Is(err, identity.ErrInvalidEmail):
		writeError(w, http.StatusBadRequest, "validation_error", "a valid email address is required")
	case errors.Is(err, identity.ErrPasswordTooShort):
		writeError(w, http.StatusBadRequest, "validation_error", "password must be at least 12 characters")
	case errors.Is(err, catalog.ErrServiceNotFound):
		writeError(w, http.StatusNotFound, "service_not_found", "service not found")
	case errors.Is(err, catalog.ErrInvalidName),
		errors.Is(err, catalog.ErrInvalidDuration),
		errors.Is(err, catalog.ErrInvalidPrice),
		errors.Is(err, catalog.ErrInvalidCurrency),
		errors.Is(err, scheduling.ErrInvalidWeekday),
		errors.Is(err, scheduling.ErrInvalidWindow),
		errors.Is(err, scheduling.ErrInvalidBuffer),
		errors.Is(err, scheduling.ErrDuplicateWeekday),
		errors.Is(err, scheduling.ErrInvalidTimeOffRange),
		errors.Is(err, scheduling.ErrInvalidDuration),
		errors.Is(err, scheduling.ErrInvalidRange):
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
	case errors.Is(err, scheduling.ErrInvalidTimezone):
		writeError(w, http.StatusBadRequest, "invalid_timezone", "tz must be a valid IANA timezone name")
	case errors.Is(err, booking.ErrBookingNotFound):
		writeError(w, http.StatusNotFound, "booking_not_found", "booking not found")
	case errors.Is(err, booking.ErrSlotUnavailable):
		writeError(w, http.StatusConflict, "slot_unavailable", "that slot is no longer available")
	case errors.Is(err, booking.ErrCutoffPassed):
		writeError(w, http.StatusUnprocessableEntity, "cutoff_passed", "the modification cutoff for this booking has passed")
	case errors.Is(err, booking.ErrInvalidTransition),
		errors.Is(err, booking.ErrTooEarly):
		writeError(w, http.StatusConflict, "invalid_status", err.Error())
	case errors.Is(err, booking.ErrInvalidDuration):
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
	case errors.Is(err, payment.ErrPaymentNotFound):
		writeError(w, http.StatusNotFound, "payment_not_found", "payment not found")
	case errors.Is(err, payment.ErrAlreadyPaid):
		writeError(w, http.StatusConflict, "already_paid", "booking is already paid")
	case errors.Is(err, payment.ErrInvalidWebhookSignature):
		writeError(w, http.StatusUnauthorized, "invalid_signature", "webhook signature is invalid")
	case errors.Is(err, payment.ErrInvalidTransition):
		writeError(w, http.StatusConflict, "invalid_status", err.Error())
	case errors.Is(err, payment.ErrInvalidAmount),
		errors.Is(err, payment.ErrInvalidCurrency),
		errors.Is(err, payment.ErrReferenceRequired):
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
	default:
		var gatewayErr *ports.GatewayError
		if errors.As(err, &gatewayErr) {
			writeError(w, http.StatusBadGateway, "payment_gateway_error", "the payment gateway could not complete the request")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "something went wrong")
	}
}
