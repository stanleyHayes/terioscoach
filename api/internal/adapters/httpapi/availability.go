package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/scheduling"
	"github.com/xcreativs/terios/api/internal/ports"
)

// defaultSlotsTimezone matches the contract default for the slots route.
const defaultSlotsTimezone = "Africa/Accra"

// WithScheduling mounts the /v1/availability routes backed by the
// scheduling port. Rules and time-off are practitioner-only; slot
// computation is public. A nil service keeps the routes mounted but
// answering 503.
func WithScheduling(svc ports.SchedulingService, auth ports.AuthService) Option {
	return func(s *Server) {
		s.Router.Route("/v1/availability", func(r chi.Router) {
			if svc == nil {
				r.HandleFunc("/*", handleSchedulingUnavailable)
				return
			}
			h := &schedulingHandler{svc: svc}
			r.Get("/slots", h.getSlots)
			r.With(RequireAuth(auth), RequireRole(identity.RolePractitioner)).Get("/rules", h.getRules)
			r.With(RequireAuth(auth), RequireRole(identity.RolePractitioner)).Put("/rules", h.putRules)
			r.With(RequireAuth(auth), RequireRole(identity.RolePractitioner)).Post("/time-off", h.addTimeOff)
		})
	}
}

// handleSchedulingUnavailable answers every availability route when the
// database — and therefore the scheduling service — is not configured.
func handleSchedulingUnavailable(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusServiceUnavailable, "service_unavailable", "availability is unavailable: database not connected")
}

type schedulingHandler struct {
	svc ports.SchedulingService
}

// windowBody is the contract window shape: minutes since local midnight.
type windowBody struct {
	StartMin int `json:"startMin"`
	EndMin   int `json:"endMin"`
}

// ruleBody is the contract weekly-rule shape.
type ruleBody struct {
	Weekday       int          `json:"weekday"`
	Windows       []windowBody `json:"windows"`
	BufferMinutes int          `json:"bufferMinutes"`
}

func newRuleBodies(rules []scheduling.WeeklyRule) []ruleBody {
	out := make([]ruleBody, 0, len(rules))
	for _, r := range rules {
		body := ruleBody{Weekday: int(r.Weekday), BufferMinutes: r.BufferMinutes, Windows: []windowBody{}}
		for _, w := range r.Windows {
			body.Windows = append(body.Windows, windowBody{StartMin: w.StartMin, EndMin: w.EndMin})
		}
		out = append(out, body)
	}
	return out
}

func rulesFromBodies(bodies []ruleBody) []scheduling.WeeklyRule {
	rules := make([]scheduling.WeeklyRule, 0, len(bodies))
	for _, b := range bodies {
		rule := scheduling.WeeklyRule{
			Weekday:       time.Weekday(b.Weekday),
			BufferMinutes: b.BufferMinutes,
		}
		for _, w := range b.Windows {
			rule.Windows = append(rule.Windows, scheduling.Window{StartMin: w.StartMin, EndMin: w.EndMin})
		}
		rules = append(rules, rule)
	}
	return rules
}

// getRules handles GET /v1/availability/rules.
func (h *schedulingHandler) getRules(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	rules, err := h.svc.GetRules(r.Context(), id.UserID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]ruleBody{"rules": newRuleBodies(rules)})
}

// putRules handles PUT /v1/availability/rules — full replacement of the
// weekly schedule.
func (h *schedulingHandler) putRules(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	var req struct {
		Rules []ruleBody `json:"rules"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	rules, err := h.svc.ReplaceRules(r.Context(), id.UserID, rulesFromBodies(req.Rules))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]ruleBody{"rules": newRuleBodies(rules)})
}

// timeOffBody is the contract time-off shape.
type timeOffBody struct {
	ID             string    `json:"id"`
	PractitionerID string    `json:"practitionerId"`
	StartAt        time.Time `json:"startAt"`
	EndAt          time.Time `json:"endAt"`
	Reason         string    `json:"reason"`
	CreatedAt      time.Time `json:"createdAt"`
}

// addTimeOff handles POST /v1/availability/time-off.
func (h *schedulingHandler) addTimeOff(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	var req struct {
		StartAt time.Time `json:"startAt"`
		EndAt   time.Time `json:"endAt"`
		Reason  string    `json:"reason"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	off, err := h.svc.AddTimeOff(r.Context(), id.UserID, req.StartAt, req.EndAt, req.Reason)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]timeOffBody{"timeOff": {
		ID:             off.ID,
		PractitionerID: off.PractitionerID,
		StartAt:        off.StartAt,
		EndAt:          off.EndAt,
		Reason:         off.Reason,
		CreatedAt:      off.CreatedAt,
	}})
}

// slotBody is one bookable slot; timestamps are RFC 3339 UTC.
type slotBody struct {
	StartAt time.Time `json:"startAt"`
	EndAt   time.Time `json:"endAt"`
}

// slotsResponse is the contract slots envelope.
type slotsResponse struct {
	ServiceID       string     `json:"serviceId"`
	DurationMinutes int        `json:"durationMinutes"`
	Timezone        string     `json:"timezone"`
	Slots           []slotBody `json:"slots"`
}

// getSlots handles GET /v1/availability/slots — public slot computation.
// from/to are inclusive YYYY-MM-DD dates interpreted in tz.
func (h *schedulingHandler) getSlots(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	serviceID := q.Get("serviceId")
	if serviceID == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "serviceId is required")
		return
	}
	from, err := parseQueryDate(q.Get("from"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "from must be a YYYY-MM-DD date")
		return
	}
	to, err := parseQueryDate(q.Get("to"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "to must be a YYYY-MM-DD date")
		return
	}
	tz := q.Get("tz")
	if tz == "" {
		tz = defaultSlotsTimezone
	}

	result, err := h.svc.GetSlots(r.Context(), serviceID, from, to, tz)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	res := slotsResponse{
		ServiceID:       serviceID,
		DurationMinutes: result.DurationMinutes,
		Timezone:        tz,
		Slots:           []slotBody{},
	}
	for _, iv := range result.Slots {
		res.Slots = append(res.Slots, slotBody{StartAt: iv.Start.UTC(), EndAt: iv.End.UTC()})
	}
	writeJSON(w, http.StatusOK, res)
}

// parseQueryDate parses a YYYY-MM-DD query date (UTC-anchored; the slot
// engine re-interprets it in the requested timezone).
func parseQueryDate(raw string) (time.Time, error) {
	return time.Parse("2006-01-02", raw)
}
