package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/ops"
	"github.com/xcreativs/terios/api/internal/ports"
)

// OpsSource supplies the counts the health rules judge. It is an interface
// so the composition root can assemble it from whatever is configured —
// with no database there are no counts, and the endpoint says so rather
// than reporting a suspiciously healthy zero.
type OpsSource interface {
	Snapshot(ctx context.Context) (ops.Snapshot, error)
}

// OpsSourceFunc adapts a function to OpsSource.
type OpsSourceFunc func(ctx context.Context) (ops.Snapshot, error)

func (f OpsSourceFunc) Snapshot(ctx context.Context) (ops.Snapshot, error) { return f(ctx) }

// WithOps mounts the operational health endpoint (LCH-09).
//
// It sits under /v1/admin because it describes the practice's own system
// and says how many accounts are locked — which is exactly the signal an
// attacker probing the login would like confirmed. It is not a public
// status page.
//
// A nil source keeps the route mounted and answering 503, matching every
// other slice: a monitor polling this must be able to tell "nothing is
// wrong" from "nothing is being measured".
func WithOps(source OpsSource, auth ports.AuthService, thresholds ops.Thresholds) Option {
	return func(s *Server) {
		s.Router.Route("/v1/admin/ops", func(r chi.Router) {
			if source == nil {
				r.HandleFunc("/*", handleOpsUnavailable)
				r.HandleFunc("/", handleOpsUnavailable)
				return
			}
			h := &opsHandler{
				source:     source,
				thresholds: thresholds,
				tracker:    ops.NewTracker(thresholds.Window),
				log:        slog.Default(),
			}
			r.Use(RequireAuth(auth), RequireRole(identity.RolePractitioner))
			r.Get("/health", h.health)
		})
	}
}

func handleOpsUnavailable(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusServiceUnavailable, "service_unavailable",
		"operational health is unavailable: database not connected")
}

type opsHandler struct {
	source     OpsSource
	thresholds ops.Thresholds
	tracker    *ops.Tracker
	log        *slog.Logger
}

type alertBody struct {
	Kind      string    `json:"kind"`
	Severity  string    `json:"severity"`
	Observed  int       `json:"observed"`
	Threshold int       `json:"threshold"`
	Since     time.Time `json:"since"`
	Summary   string    `json:"summary"`
}

// health reports what is currently wrong, if anything.
//
// The response is shaped for two readers at once: an uptime monitor, which
// only looks at `status`, and a person, who needs the summaries. The HTTP
// status stays 200 for both — a monitor that treats a 500 as "the API is
// down" would page for a mail backlog, and the API is not down.
func (h *opsHandler) health(w http.ResponseWriter, r *http.Request) {
	snapshot, err := h.source.Snapshot(r.Context())
	if err != nil {
		// Losing the ability to measure is itself worth reporting, and it
		// must not read as healthy.
		h.log.Error("ops snapshot failed", "error", err)
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "unknown",
			"reason":  "the health counters could not be read",
			"alerts":  []alertBody{},
			"checked": time.Now().UTC(),
		})
		return
	}

	now := time.Now().UTC()
	active := ops.Evaluate(snapshot, h.thresholds, h.tracker.Since(), now)
	h.tracker.NoteActive(active, now)

	body := make([]alertBody, 0, len(active))
	status := "healthy"
	for _, alert := range active {
		if alert.Severity == ops.SeverityCritical {
			status = "critical"
		} else if status == "healthy" {
			status = "degraded"
		}
		body = append(body, alertBody{
			Kind:      string(alert.Kind),
			Severity:  string(alert.Severity),
			Observed:  alert.Observed,
			Threshold: alert.Threshold,
			Since:     alert.Since,
			Summary:   alert.Summary,
		})
		// Logged as well as returned, so alerting can be driven from the
		// log stream by anyone who never polls this endpoint. The message
		// is fixed and the detail is structured, which is what makes it
		// matchable.
		h.log.Error("ops alert",
			"kind", alert.Kind,
			"severity", alert.Severity,
			"observed", alert.Observed,
			"threshold", alert.Threshold,
			"since", alert.Since,
		)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  status,
		"alerts":  body,
		"checked": time.Now().UTC(),
		"counters": map[string]int{
			"notificationBacklog":         snapshot.NotificationBacklog,
			"notificationFailures":        snapshot.NotificationFailures,
			"lockedAccounts":              snapshot.LockedAccounts,
			"paymentVerificationFailures": snapshot.PaymentVerificationFailures,
		},
	})
}
