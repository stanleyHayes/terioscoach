package ports

import (
	"context"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/identity"
)

// BookingFilter narrows the practitioner's calendar view. Zero values mean
// "unbounded" / "all statuses".
type BookingFilter struct {
	From   *time.Time
	To     *time.Time
	Status booking.Status // empty = all statuses
}

// BookingService is the inbound port for the booking slice.
type BookingService interface {
	// CreateBooking books a slot for a client. startAt must match a slot
	// the availability engine would generate for the service; anything
	// else — including a slot lost to a concurrent race — returns
	// booking.ErrSlotUnavailable. Unknown/inactive services return
	// catalog.ErrServiceNotFound.
	CreateBooking(ctx context.Context, clientID, serviceID string, startAt time.Time, tz string) (booking.Booking, error)
	// ListMine returns the client's own bookings, upcoming and past,
	// ordered by startAt ascending.
	ListMine(ctx context.Context, clientID string) ([]booking.Booking, error)
	// ListForPractitioner returns the practitioner's bookings, optionally
	// narrowed by time range and status, ordered by startAt ascending.
	ListForPractitioner(ctx context.Context, practitionerID string, filter BookingFilter) ([]booking.Booking, error)
	// GetBooking returns one booking to its owner or its practitioner.
	// Cross-owner access returns booking.ErrBookingNotFound (isolation).
	GetBooking(ctx context.Context, id identity.Identity, bookingID string) (booking.Booking, error)
	// RescheduleBooking moves a booking to a new slot (validated exactly
	// like CreateBooking), freeing the old one. Clients are bound by the
	// practice cutoff (booking.ErrCutoffPassed); practitioners are not.
	RescheduleBooking(ctx context.Context, id identity.Identity, bookingID string, startAt time.Time, tz string) (booking.Booking, error)
	// CancelBooking cancels a booking, freeing its slot. Same cutoff rules
	// as RescheduleBooking.
	CancelBooking(ctx context.Context, id identity.Identity, bookingID string) (booking.Booking, error)
	// CompleteBooking marks a booking completed — practitioner-only, and
	// only after the appointment has ended (booking.ErrTooEarly).
	CompleteBooking(ctx context.Context, practitionerID, bookingID string) (booking.Booking, error)
	// MarkNoShow marks a booking no_show — practitioner-only, and only
	// after the appointment has started (booking.ErrTooEarly).
	MarkNoShow(ctx context.Context, practitionerID, bookingID string) (booking.Booking, error)
}

// BookingRepository is the outbound port for booking persistence.
type BookingRepository interface {
	// Create persists a new booking, assigning its ID. A confirmed booking
	// already holding the same practitioner+startAt returns
	// booking.ErrSlotUnavailable (the storage-layer race guard).
	Create(ctx context.Context, b booking.Booking) (booking.Booking, error)
	// FindByID looks up a booking by id; misses return
	// booking.ErrBookingNotFound.
	FindByID(ctx context.Context, id string) (booking.Booking, error)
	// Update persists a booking's mutable state (slot, status, timestamps).
	// Misses return booking.ErrBookingNotFound; a confirmed-slot collision
	// on reschedule returns booking.ErrSlotUnavailable.
	Update(ctx context.Context, b booking.Booking) (booking.Booking, error)
	// ListByClient returns the client's bookings ordered by startAt
	// ascending (upcoming first).
	ListByClient(ctx context.Context, clientID string) ([]booking.Booking, error)
	// ListByPractitioner returns the practitioner's bookings matching the
	// filter, ordered by startAt ascending.
	ListByPractitioner(ctx context.Context, practitionerID string, filter BookingFilter) ([]booking.Booking, error)
}
