package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/app/auth"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

// newTestServer wires a Server to a real auth service over in-memory fakes.
func newTestServer() (*Server, ports.AuthService) {
	svc := auth.NewService(
		portstest.NewFakeUserRepository(),
		portstest.NewFakeRefreshTokenRepository(),
		portstest.FakeHasher{},
		portstest.NewFakeTokenIssuer(15*time.Minute),
		30*24*time.Hour,
	)
	return NewServer(WithAuth(svc)), svc
}

func doJSON(t *testing.T, srv *Server, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	srv.Router.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(v); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
}

type userTestBody struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
	Name  string `json:"name"`
}

type authTestResponse struct {
	User         userTestBody `json:"user"`
	AccessToken  string       `json:"accessToken"`
	RefreshToken string       `json:"refreshToken"`
}

type errorTestResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func registerViaHTTP(t *testing.T, srv *Server, email string) authTestResponse {
	t.Helper()
	rec := doJSON(t, srv, http.MethodPost, "/v1/auth/register", map[string]string{
		"email":    email,
		"name":     "Test User",
		"password": "a long enough password",
	}, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body %s", rec.Code, rec.Body.String())
	}
	var res authTestResponse
	decodeBody(t, rec, &res)
	return res
}

func TestRegisterEndpoint(t *testing.T) {
	srv, _ := newTestServer()
	res := registerViaHTTP(t, srv, "happy@example.com")

	if res.User.ID == "" || res.User.Role != "client" {
		t.Errorf("user = %+v, want id set and role client", res.User)
	}
	if res.User.Email != "happy@example.com" || res.User.Name != "Test User" {
		t.Errorf("user = %+v, want contract shape {id, email, role, name}", res.User)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Error("expected both tokens")
	}
	if _, role, ok := portstest.SplitAccessToken(res.AccessToken); !ok || role != "client" {
		t.Errorf("access token does not carry client identity")
	}
}

func TestRegisterEndpointValidation(t *testing.T) {
	srv, _ := newTestServer()

	cases := []struct {
		name     string
		body     map[string]string
		wantCode string
	}{
		{"bad email", map[string]string{"email": "nope", "name": "X", "password": "a long enough password"}, "validation_error"},
		{"short password", map[string]string{"email": "ok@example.com", "name": "X", "password": "short"}, "validation_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doJSON(t, srv, http.MethodPost, "/v1/auth/register", tc.body, nil)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			var errRes errorTestResponse
			decodeBody(t, rec, &errRes)
			if errRes.Error.Code != tc.wantCode {
				t.Errorf("error code = %q, want %q", errRes.Error.Code, tc.wantCode)
			}
		})
	}

	// Duplicate email -> 409.
	registerViaHTTP(t, srv, "dupe@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/v1/auth/register", map[string]string{
		"email": "dupe@example.com", "name": "Y", "password": "a long enough password",
	}, nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d, want 409", rec.Code)
	}
}

func TestLoginEndpoint(t *testing.T) {
	srv, _ := newTestServer()
	registerViaHTTP(t, srv, "login@example.com")

	rec := doJSON(t, srv, http.MethodPost, "/v1/auth/login", map[string]string{
		"email": "login@example.com", "password": "a long enough password",
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, body %s", rec.Code, rec.Body.String())
	}
	var loginRes authTestResponse
	decodeBody(t, rec, &loginRes)
	if loginRes.User.Email != "login@example.com" || loginRes.User.Name != "Test User" || loginRes.User.Role != "client" {
		t.Errorf("login user = %+v, want contract shape {id, email, role, name}", loginRes.User)
	}

	// Unknown email and wrong password: identical 401 + code (no enumeration).
	for _, body := range []map[string]string{
		{"email": "ghost@example.com", "password": "a long enough password"},
		{"email": "login@example.com", "password": "the wrong password!"},
	} {
		rec := doJSON(t, srv, http.MethodPost, "/v1/auth/login", body, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d for %v, want 401", rec.Code, body)
		}
		var errRes errorTestResponse
		decodeBody(t, rec, &errRes)
		if errRes.Error.Code != "invalid_credentials" {
			t.Errorf("error code = %q, want invalid_credentials", errRes.Error.Code)
		}
	}
}

func TestRefreshEndpointRotates(t *testing.T) {
	srv, _ := newTestServer()
	first := registerViaHTTP(t, srv, "rotate@example.com")

	rec := doJSON(t, srv, http.MethodPost, "/v1/auth/refresh", map[string]string{
		"refreshToken": first.RefreshToken,
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, body %s", rec.Code, rec.Body.String())
	}
	var second authTestResponse
	decodeBody(t, rec, &second)
	if second.RefreshToken == first.RefreshToken {
		t.Error("rotation must return a new refresh token")
	}
	if second.User.Email != "rotate@example.com" || second.User.Name != "Test User" || second.User.ID != first.User.ID {
		t.Errorf("refresh user = %+v, want full account shape of the same user", second.User)
	}

	// Reuse of the rotated token is rejected.
	rec = doJSON(t, srv, http.MethodPost, "/v1/auth/refresh", map[string]string{
		"refreshToken": first.RefreshToken,
	}, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("reuse status = %d, want 401", rec.Code)
	}
	var errRes errorTestResponse
	decodeBody(t, rec, &errRes)
	if errRes.Error.Code != "token_invalid" {
		t.Errorf("error code = %q, want token_invalid", errRes.Error.Code)
	}
}

func TestLogoutEndpoint(t *testing.T) {
	srv, _ := newTestServer()
	res := registerViaHTTP(t, srv, "bye@example.com")

	rec := doJSON(t, srv, http.MethodPost, "/v1/auth/logout", map[string]string{
		"refreshToken": res.RefreshToken,
	}, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want 204", rec.Code)
	}

	// The logged-out token can no longer refresh; logout stays idempotent.
	rec = doJSON(t, srv, http.MethodPost, "/v1/auth/refresh", map[string]string{
		"refreshToken": res.RefreshToken,
	}, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("refresh after logout status = %d, want 401", rec.Code)
	}
	rec = doJSON(t, srv, http.MethodPost, "/v1/auth/logout", map[string]string{
		"refreshToken": "never-issued",
	}, nil)
	if rec.Code != http.StatusNoContent {
		t.Errorf("unknown logout status = %d, want 204", rec.Code)
	}
}

func TestMeEndpoint(t *testing.T) {
	srv, _ := newTestServer()
	res := registerViaHTTP(t, srv, "me@example.com")

	rec := doJSON(t, srv, http.MethodGet, "/v1/auth/me", nil, map[string]string{
		"Authorization": "Bearer " + res.AccessToken,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("me status = %d, body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		User userTestBody `json:"user"`
	}
	decodeBody(t, rec, &body)
	if body.User != res.User {
		t.Errorf("me = %+v, want full account shape %+v", body.User, res.User)
	}

	// No header, malformed header, forged token: all 401.
	for _, header := range []string{"", "Bearer", "Token abc", "Bearer forged-token"} {
		rec := doJSON(t, srv, http.MethodGet, "/v1/auth/me", nil, map[string]string{
			"Authorization": header,
		})
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("me with %q status = %d, want 401", header, rec.Code)
		}
	}
}

func TestRequireRole(t *testing.T) {
	srv, svc := newTestServer()
	srv.Router.With(RequireAuth(svc), RequireRole(identity.RolePractitioner)).
		Get("/v1/test-practitioner-only", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"ok": "yes"})
		})

	res := registerViaHTTP(t, srv, "client@example.com")
	rec := doJSON(t, srv, http.MethodGet, "/v1/test-practitioner-only", nil, map[string]string{
		"Authorization": "Bearer " + res.AccessToken,
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("client on practitioner route status = %d, want 403", rec.Code)
	}
	var errRes errorTestResponse
	decodeBody(t, rec, &errRes)
	if errRes.Error.Code != "forbidden" {
		t.Errorf("error code = %q, want forbidden", errRes.Error.Code)
	}
}

func TestAuthUnavailableWithoutService(t *testing.T) {
	srv := NewServer(WithAuth(nil))

	for _, path := range []string{"/v1/auth/register", "/v1/auth/login", "/v1/auth/me"} {
		rec := doJSON(t, srv, http.MethodPost, path, map[string]string{}, nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s status = %d, want 503", path, rec.Code)
		}
		var errRes errorTestResponse
		decodeBody(t, rec, &errRes)
		if errRes.Error.Code != "service_unavailable" {
			t.Errorf("%s error code = %q, want service_unavailable", path, errRes.Error.Code)
		}
	}
}

func TestMalformedJSON(t *testing.T) {
	srv, _ := newTestServer()
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader("{not json"))
	rec := httptest.NewRecorder()
	srv.Router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var errRes errorTestResponse
	decodeBody(t, rec, &errRes)
	if errRes.Error.Code != "bad_request" {
		t.Errorf("error code = %q, want bad_request", errRes.Error.Code)
	}
}
