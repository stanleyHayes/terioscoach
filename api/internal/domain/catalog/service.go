// Package catalog is the domain core for the services a practitioner
// offers. It imports nothing outside the standard library — no frameworks,
// no drivers.
package catalog

import (
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// DefaultCurrency is applied on create when none is supplied.
	DefaultCurrency = "USD"

	maxNameLength      = 200
	minDurationMinutes = 5
	maxDurationMinutes = 480
)

// Service is a bookable treatment. PriceKobo is money in integer minor
// units; the domain never sees floating-point money. DeletedAt is the
// soft-delete marker: set when bookings exist and the record must be
// retained for history; soft-deleted services are invisible to every list.
type Service struct {
	ID              string
	PractitionerID  string
	Name            string
	Description     string
	ImageURL        string
	DurationMinutes int
	PriceKobo       int64
	Currency        string
	Active          bool
	SortOrder       int
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}

// NewService validates input and builds an active Service. An empty
// currency falls back to DefaultCurrency.
func NewService(
	practitionerID, name, description string,
	durationMinutes int,
	priceKobo int64,
	currency string,
	sortOrder int,
	now time.Time,
) (Service, error) {
	name = strings.TrimSpace(name)
	if err := ValidateName(name); err != nil {
		return Service{}, err
	}
	if err := ValidateDuration(durationMinutes); err != nil {
		return Service{}, err
	}
	if err := ValidatePrice(priceKobo); err != nil {
		return Service{}, err
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		currency = DefaultCurrency
	}
	if err := ValidateCurrency(currency); err != nil {
		return Service{}, err
	}
	now = now.UTC()
	return Service{
		PractitionerID:  practitionerID,
		Name:            name,
		Description:     strings.TrimSpace(description),
		DurationMinutes: durationMinutes,
		PriceKobo:       priceKobo,
		Currency:        currency,
		Active:          true,
		SortOrder:       sortOrder,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

// ValidateName enforces the required bounded name.
func ValidateName(name string) error {
	if n := utf8.RuneCountInString(name); n == 0 || n > maxNameLength {
		return ErrInvalidName
	}
	return nil
}

// ValidateDuration enforces the session-length bounds.
func ValidateDuration(minutes int) error {
	if minutes < minDurationMinutes || minutes > maxDurationMinutes {
		return ErrInvalidDuration
	}
	return nil
}

// ValidatePrice enforces non-negative minor units.
func ValidatePrice(priceKobo int64) error {
	if priceKobo < 0 {
		return ErrInvalidPrice
	}
	return nil
}

// ValidateCurrency enforces a 3-letter ASCII code (ISO 4217 alphabetic).
func ValidateCurrency(currency string) error {
	if len(currency) != 3 {
		return ErrInvalidCurrency
	}
	for i := 0; i < len(currency); i++ {
		if currency[i] < 'A' || currency[i] > 'Z' {
			return ErrInvalidCurrency
		}
	}
	return nil
}

// ValidateImageURL accepts a local public asset path or an absolute HTTP(S)
// delivery URL. Other schemes must never reach a public image element.
func ValidateImageURL(value string) error {
	value = strings.TrimSpace(value)
	if value == "" || (strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//")) {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ErrInvalidImageURL
	}
	return nil
}
