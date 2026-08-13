package httpapi

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/app/auth"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/ops"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

// newOpsRig mounts the ops endpoint over a scripted snapshot source.
func newOpsRig(t *testing.T, snapshot ops.Snapshot, err error) (*Server, string, string) {
	t.Helper()
	issuer := portstest.NewFakeTokenIssuer(15 * time.Minute)
	authSvc := auth.NewService(
		portstest.NewFakeUserRepository(),
		portstest.NewFakeRefreshTokenRepository(),
		portstest.FakeHasher{},
		issuer,
		30*24*time.Hour,
	)

	source := OpsSourceFunc(func(context.Context) (ops.Snapshot, error) {
		return snapshot, err
	})
	srv := NewServer(WithAuth(authSvc), WithOps(source, authSvc, ops.DefaultThresholds()))

	issue := func(id string, role identity.Role) string {
		token, _, issueErr := issuer.IssueAccessToken(identity.Identity{UserID: id, Role: role})
		if issueErr != nil {
			t.Fatalf("issue token: %v", issueErr)
		}
		return token
	}
	return srv, issue("prac-1", identity.RolePractitioner), issue("client-1", identity.RoleClient)
}

type opsResponse struct {
	Status   string `json:"status"`
	Reason   string `json:"reason"`
	Counters struct {
		NotificationBacklog int `json:"notificationBacklog"`
		LockedAccounts      int `json:"lockedAccounts"`
	} `json:"counters"`
	Alerts []struct {
		Kind      string    `json:"kind"`
		Severity  string    `json:"severity"`
		Observed  int       `json:"observed"`
		Threshold int       `json:"threshold"`
		Since     time.Time `json:"since"`
		Summary   string    `json:"summary"`
	} `json:"alerts"`
}

func TestOpsHealthyReportsNoAlerts(t *testing.T) {
	srv, practitioner, _ := newOpsRig(t, ops.Snapshot{}, nil)

	rec := doJSON(t, srv, http.MethodGet, "/v1/admin/ops/health", nil, bearer(practitioner))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body opsResponse
	decodeBody(t, rec, &body)

	if body.Status != "healthy" {
		t.Errorf("status = %q, want healthy", body.Status)
	}
	// An absent array and an empty one read very differently to a monitor.
	if body.Alerts == nil {
		t.Error("alerts is null, want an empty array")
	}
}

func TestOpsReportsABacklogWithSomethingActionable(t *testing.T) {
	thresholds := ops.DefaultThresholds()
	srv, practitioner, _ := newOpsRig(t, ops.Snapshot{
		NotificationBacklog: thresholds.NotificationBacklogCritical,
	}, nil)

	rec := doJSON(t, srv, http.MethodGet, "/v1/admin/ops/health", nil, bearer(practitioner))
	var body opsResponse
	decodeBody(t, rec, &body)

	if body.Status != "critical" {
		t.Fatalf("status = %q, want critical", body.Status)
	}
	if len(body.Alerts) != 1 {
		t.Fatalf("got %d alerts, want 1", len(body.Alerts))
	}
	alert := body.Alerts[0]
	if alert.Kind != "notification_backlog" || alert.Severity != "critical" {
		t.Errorf("alert = %+v, want a critical backlog", alert)
	}
	if alert.Summary == "" {
		t.Error("no summary — a bare count tells whoever is woken nothing")
	}
	if alert.Since.IsZero() {
		t.Error("no since — a responder cannot tell a spike from a week-old fault")
	}
}

// TestOpsStaysHTTP200WhenUnhealthy. A monitor treating 5xx as "the API is
// down" would page for a mail backlog, and the API is not down.
func TestOpsStaysHTTP200WhenUnhealthy(t *testing.T) {
	thresholds := ops.DefaultThresholds()
	srv, practitioner, _ := newOpsRig(t, ops.Snapshot{
		LockedAccounts: thresholds.LockoutCritical,
	}, nil)

	rec := doJSON(t, srv, http.MethodGet, "/v1/admin/ops/health", nil, bearer(practitioner))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 with a critical body", rec.Code)
	}
}

// TestOpsSaysUnknownRatherThanHealthyWhenItCannotMeasure. Reporting zero
// counters because the query failed is the worst possible answer: it looks
// exactly like a healthy system.
func TestOpsSaysUnknownRatherThanHealthyWhenItCannotMeasure(t *testing.T) {
	srv, practitioner, _ := newOpsRig(t, ops.Snapshot{}, errors.New("mongo is unreachable"))

	rec := doJSON(t, srv, http.MethodGet, "/v1/admin/ops/health", nil, bearer(practitioner))
	var body opsResponse
	decodeBody(t, rec, &body)

	if body.Status != "unknown" {
		t.Errorf("status = %q, want unknown", body.Status)
	}
	if body.Reason == "" {
		t.Error("no reason given for an unknown status")
	}
}

// TestOpsIsPractitionerOnly: the locked-account count is exactly what an
// attacker probing the login would like confirmed.
func TestOpsIsPractitionerOnly(t *testing.T) {
	srv, _, client := newOpsRig(t, ops.Snapshot{}, nil)

	anonymous := doJSON(t, srv, http.MethodGet, "/v1/admin/ops/health", nil, nil)
	if anonymous.Code != http.StatusUnauthorized {
		t.Errorf("anonymous = %d, want 401", anonymous.Code)
	}

	asClient := doJSON(t, srv, http.MethodGet, "/v1/admin/ops/health", nil, bearer(client))
	if asClient.Code != http.StatusForbidden {
		t.Errorf("client = %d, want 403", asClient.Code)
	}
}

// TestOpsUnavailableWithoutASource matches every other slice: a monitor
// must be able to tell "nothing is wrong" from "nothing is measured".
func TestOpsUnavailableWithoutASource(t *testing.T) {
	srv := NewServer(WithOps(nil, nil, ops.DefaultThresholds()))

	rec := doJSON(t, srv, http.MethodGet, "/v1/admin/ops/health", nil, nil)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

// TestOpsKeepsSinceStableAcrossPolls: a monitor polls this every minute,
// and every poll must not reset how long the problem has been going on.
func TestOpsKeepsSinceStableAcrossPolls(t *testing.T) {
	thresholds := ops.DefaultThresholds()
	srv, practitioner, _ := newOpsRig(t, ops.Snapshot{
		LockedAccounts: thresholds.LockoutWarning,
	}, nil)

	var first, second opsResponse
	decodeBody(t, doJSON(t, srv, http.MethodGet, "/v1/admin/ops/health", nil, bearer(practitioner)), &first)
	decodeBody(t, doJSON(t, srv, http.MethodGet, "/v1/admin/ops/health", nil, bearer(practitioner)), &second)

	if len(first.Alerts) != 1 || len(second.Alerts) != 1 {
		t.Fatalf("alerts: first %d, second %d — want 1 each", len(first.Alerts), len(second.Alerts))
	}
	if !first.Alerts[0].Since.Equal(second.Alerts[0].Since) {
		t.Errorf("since moved between polls: %v then %v", first.Alerts[0].Since, second.Alerts[0].Since)
	}
}
