package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/reporting"
	"github.com/xcreativs/terios/api/internal/ports"
)

// WithReports mounts the reporting routes backed by the report port
// (BE-15). Every route is practitioner-only and scoped to the caller's own
// practice. A nil service keeps the routes mounted but answering 503.
func WithReports(svc ports.ReportService, auth ports.AuthService) Option {
	return func(s *Server) {
		s.Router.Route("/v1/admin/reports", func(r chi.Router) {
			if svc == nil {
				r.HandleFunc("/*", handleReportsUnavailable)
				r.HandleFunc("/", handleReportsUnavailable)
				return
			}
			h := &reportHandler{svc: svc}
			r.Use(RequireAuth(auth), RequireRole(identity.RolePractitioner))
			r.Get("/practice", h.practice)
			r.Get("/upcoming-load", h.upcomingLoad)
		})
	}
}

// handleReportsUnavailable answers every reporting route when the database
// is not configured.
func handleReportsUnavailable(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusServiceUnavailable, "service_unavailable", "reports are unavailable: database not connected")
}

type reportHandler struct {
	svc ports.ReportService
}

// practice handles GET /v1/admin/reports/practice.
func (h *reportHandler) practice(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	query := r.URL.Query()

	from, ok := parseReportTime(w, query.Get("from"), "from")
	if !ok {
		return
	}
	to, ok := parseReportTime(w, query.Get("to"), "to")
	if !ok {
		return
	}

	report, err := h.svc.Practice(r.Context(), id.UserID, ports.ReportRequest{
		From:        from,
		To:          to,
		Granularity: reporting.Granularity(query.Get("granularity")),
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}

	byService := make([]map[string]any, 0, len(report.ByService))
	for _, row := range report.ByService {
		byService = append(byService, map[string]any{
			"serviceId":  row.ServiceID,
			"name":       row.Name,
			"sessions":   row.Sessions,
			"incomeKobo": row.IncomeKobo,
		})
	}
	series := make([]map[string]any, 0, len(report.Series))
	for _, bucket := range report.Series {
		series = append(series, map[string]any{
			"start":      bucket.Start.UTC(),
			"sessions":   bucket.Sessions,
			"incomeKobo": bucket.IncomeKobo,
		})
	}
	distribution := make(map[string]int, 5)
	for star := 1; star <= 5; star++ {
		distribution[strconv.Itoa(star)] = report.Reviews.Distribution[star]
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"from":        report.From.UTC(),
		"to":          report.To.UTC(),
		"granularity": string(report.Granularity),
		"summary": map[string]any{
			"sessionsCompleted": report.Summary.SessionsCompleted,
			"sessionsUpcoming":  report.Summary.SessionsUpcoming,
			"cancellations":     report.Summary.Cancellations,
			"noShows":           report.Summary.NoShows,
			"newClients":        report.Summary.NewClients,
			"incomeKobo":        report.Summary.IncomeKobo,
			"refundedKobo":      report.Summary.RefundedKobo,
			"netKobo":           report.Summary.NetKobo(),
			"currency":          report.Summary.Currency,
		},
		"byService": byService,
		"series":    series,
		"reviews": map[string]any{
			"count":        report.Reviews.Count,
			"average":      report.Reviews.Average,
			"distribution": distribution,
		},
	})
}

// upcomingLoad handles GET /v1/admin/reports/upcoming-load.
func (h *reportHandler) upcomingLoad(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}

	days := 7
	if raw := r.URL.Query().Get("days"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 90 {
			writeError(w, http.StatusBadRequest, "validation_error", "days must be between 1 and 90")
			return
		}
		days = parsed
	}

	load, err := h.svc.UpcomingLoad(r.Context(), id.UserID, days)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(load))
	for _, entry := range load {
		items = append(items, map[string]any{
			"date":     entry.Date.UTC().Format("2006-01-02"),
			"sessions": entry.Sessions,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// parseReportTime reads an RFC 3339 timestamp, writing a 400 on a bad one.
// An empty value is not an error: it means "use the default window".
func parseReportTime(w http.ResponseWriter, raw, field string) (time.Time, bool) {
	if raw == "" {
		return time.Time{}, true
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", field+" must be an RFC 3339 timestamp")
		return time.Time{}, false
	}
	return parsed.UTC(), true
}
