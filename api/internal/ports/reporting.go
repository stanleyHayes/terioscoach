package ports

import (
	"context"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/reporting"
	"github.com/xcreativs/terios/api/internal/domain/review"
)

// ReportRequest is one report's window and shape.
type ReportRequest struct {
	From        time.Time
	To          time.Time
	Granularity reporting.Granularity
}

// PracticeReport is everything the reporting dashboard shows for one
// window, assembled in a single call so the numbers on one screen are all
// computed from the same data and cannot disagree with each other.
type PracticeReport struct {
	From        time.Time
	To          time.Time
	Summary     reporting.Summary
	ByService   []reporting.ServiceIncome
	Series      []reporting.PeriodIncome
	Granularity reporting.Granularity
	// Reviews is the practice's approved-review aggregate — the closest
	// thing to a content-engagement number the platform holds honestly.
	Reviews review.Summary
}

// ReportService is the inbound port for the reporting slice (BE-15).
// Every method is practitioner-only and scoped to the caller's own
// practice; there is no cross-practice view.
type ReportService interface {
	// Practice assembles the dashboard report for one window.
	Practice(ctx context.Context, practitionerID string, req ReportRequest) (PracticeReport, error)
	// UpcomingLoad is the next `days` of confirmed sessions per day.
	UpcomingLoad(ctx context.Context, practitionerID string, days int) ([]reporting.DayLoad, error)
}
