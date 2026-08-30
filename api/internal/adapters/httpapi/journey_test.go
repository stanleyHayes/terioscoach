package httpapi

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/adapters/cloudflare"
	"github.com/xcreativs/terios/api/internal/adapters/memory"
	"github.com/xcreativs/terios/api/internal/app/auth"
	bookingapp "github.com/xcreativs/terios/api/internal/app/booking"
	"github.com/xcreativs/terios/api/internal/app/catalog"
	clientsapp "github.com/xcreativs/terios/api/internal/app/clients"
	notesapp "github.com/xcreativs/terios/api/internal/app/notes"
	notificationsapp "github.com/xcreativs/terios/api/internal/app/notifications"
	paymentsapp "github.com/xcreativs/terios/api/internal/app/payments"
	reportsapp "github.com/xcreativs/terios/api/internal/app/reports"
	reviewsapp "github.com/xcreativs/terios/api/internal/app/reviews"
	schedulingapp "github.com/xcreativs/terios/api/internal/app/scheduling"
	signalingapp "github.com/xcreativs/terios/api/internal/app/signaling"
	domainbooking "github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/notification"
	domainsignaling "github.com/xcreativs/terios/api/internal/domain/signaling"
	"github.com/xcreativs/terios/api/internal/ports"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

/*
End-to-end journey (LCH-02).

Every other test in this package proves one slice in isolation. This one
proves they compose: a single client, a single booking, carried from the
first search of the catalogue through to a published review, over real HTTP
requests against the real router with every slice mounted at once.

What it is for is the seams. A slice can be perfectly correct on its own and
still be unreachable in sequence — a booking whose id the payment route
won't accept, a note written against a session the client can no longer see,
a review gate that opens on a status nothing ever sets. Those failures do
not show up in a per-slice test, because a per-slice test seeds its own
world.

It runs with no infrastructure: in-memory repositories, a fake gateway, a
fake mailer. What it cannot prove is what the fakes stand in for — Mongo's
unique indexes, Paystack's real webhook, a browser's WebRTC stack. The
browser half of LCH-02 lives in e2e/ and needs a deployed stack.
*/

// journeyRig is the whole application, mounted at once.
type journeyRig struct {
	srv           *Server
	users         *portstest.FakeUserRepository
	bookings      *portstest.FakeBookingRepository
	gateway       *portstest.FakePaymentGateway
	mailer        *portstest.FakeMailer
	jobs          *portstest.FakeNotificationJobRepository
	notifications *notificationsapp.Service
	// issue mints an access token for a seeded user, for the tests that
	// need more accounts than the registration rate limit allows.
	issue func(userID string, role identity.Role) string

	practitionerID    string
	practitionerToken string
	clientID          string
	clientToken       string
	clientEmail       string
}

func newJourneyRig(t *testing.T) journeyRig {
	t.Helper()

	issuer := portstest.NewFakeTokenIssuer(15 * time.Minute)
	users := portstest.NewFakeUserRepository()
	authSvc := auth.NewService(
		users,
		portstest.NewFakeRefreshTokenRepository(),
		portstest.FakeHasher{},
		issuer,
		30*24*time.Hour,
	)

	services := portstest.NewFakeServiceRepository()
	availability := portstest.NewFakeAvailabilityRepository()
	bookings := portstest.NewFakeBookingRepository()
	payments := portstest.NewFakePaymentRepository()
	gateway := portstest.NewFakePaymentGateway("sk_test")
	notes := portstest.NewFakeSessionNoteRepository()
	reviews := portstest.NewFakeReviewRepository()
	profiles := portstest.NewFakeClientProfileRepository()

	mailer := portstest.NewFakeMailer()
	jobs := portstest.NewFakeNotificationJobRepository()
	notifier := notificationsapp.NewService(
		jobs,
		&portstest.FakeEmailRenderer{},
		mailer,
		notificationsapp.Options{
			ReminderLead:  notification.DefaultReminderLead,
			Retry:         notification.DefaultRetryPolicy(),
			PracticeEmail: "practice@terioscoach.com",
			// A journey that silently swallowed a queueing failure would
			// pass while sending nothing.
			Report: func(err error) { t.Errorf("notification failure: %v", err) },
		},
	)

	bookingSvc := bookingapp.NewService(
		bookings, services, availability, bookings, domainbooking.DefaultPolicy(),
		bookingapp.WithNotifications(notifier, users),
	)
	notesSvc := notesapp.NewService(
		notes, bookings,
		notesapp.WithNotifications(notifier, users, services),
	)
	signalingSvc := signalingapp.NewService(bookings, memory.NewTicketStore(), signalingapp.Options{
		Policy: domainsignaling.DefaultPolicy(),
		ICE: cloudflare.StaticProvider{
			Servers: []ports.ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}},
		},
	})

	srv := NewServer(
		WithAuth(authSvc),
		WithCatalog(catalog.NewService(services), authSvc),
		WithScheduling(schedulingapp.NewService(services, availability, bookings), authSvc),
		WithBooking(bookingSvc, authSvc),
		WithPayments(paymentsapp.NewService(payments, bookings, services, users, gateway), authSvc),
		WithNotes(notesSvc, authSvc),
		WithReviews(reviewsapp.NewService(reviews, bookings, users, services), authSvc),
		WithSessions(signalingSvc, nil, authSvc),
		WithClients(clientsapp.NewService(
			profiles, users, bookings, payments,
			&portstest.FakeDocumentCounter{}, &portstest.FakeFormSubmissionCounter{},
		), authSvc),
		WithReports(reportsapp.NewService(bookings, payments, services, reviews), authSvc),
	)

	rig := journeyRig{
		srv:            srv,
		users:          users,
		bookings:       bookings,
		gateway:        gateway,
		mailer:         mailer,
		jobs:           jobs,
		notifications:  notifier,
		practitionerID: "prac-1",
		clientEmail:    "ama@example.com",
	}

	// The client arrives the way a real one does: through registration.
	rec := doJSON(t, srv, http.MethodPost, "/v1/auth/register", map[string]any{
		"email": rig.clientEmail, "password": "correct horse battery", "name": "Ama Serwaa",
	}, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body %s", rec.Code, rec.Body.String())
	}
	var registered struct {
		AccessToken string `json:"accessToken"`
		User        struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	decodeBody(t, rec, &registered)
	rig.clientID = registered.User.ID
	rig.clientToken = registered.AccessToken

	// The practitioner is seeded: there is no self-registration for the
	// practice side, by design.
	practitioner, err := identity.NewUser(
		"terios@terioscoach.com", "Terios", "hash", identity.RolePractitioner, time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("identity.NewUser: %v", err)
	}
	practitioner, err = users.Create(t.Context(), practitioner)
	if err != nil {
		t.Fatalf("seed practitioner: %v", err)
	}
	rig.practitionerID = practitioner.ID

	token, _, err := issuer.IssueAccessToken(identity.Identity{
		UserID: practitioner.ID, Role: identity.RolePractitioner,
	})
	if err != nil {
		t.Fatalf("issue practitioner token: %v", err)
	}
	rig.practitionerToken = token
	rig.issue = func(userID string, role identity.Role) string {
		issued, _, err := issuer.IssueAccessToken(identity.Identity{UserID: userID, Role: role})
		if err != nil {
			t.Fatalf("issue token: %v", err)
		}
		return issued
	}
	return rig
}

// mustJSON runs a request, fails on an unexpected status, and decodes.
func (r journeyRig) mustJSON(t *testing.T, method, path string, body any, token string, want int, out any) {
	t.Helper()
	var headers map[string]string
	if token != "" {
		headers = bearer(token)
	}
	rec := doJSON(t, r.srv, method, path, body, headers)
	if rec.Code != want {
		t.Fatalf("%s %s = %d, want %d (body %s)", method, path, rec.Code, want, rec.Body.String())
	}
	if out != nil {
		decodeBody(t, rec, out)
	}
}

// TestTheWholeJourney walks one client and one booking from the catalogue
// to a published review. Each step depends on the one before it: a failure
// anywhere stops the test, because in reality it stops the client.
func TestTheWholeJourney(t *testing.T) {
	rig := newJourneyRig(t)

	// ---- 1. The practitioner sets up shop -------------------------------

	var created struct {
		Service struct {
			ID        string `json:"id"`
			PriceKobo int64  `json:"priceKobo"`
		} `json:"service"`
	}
	rig.mustJSON(t, http.MethodPost, "/v1/services", map[string]any{
		"name": "Aromatherapy massage", "durationMinutes": 60, "priceKobo": 25000, "currency": "GHS",
	}, rig.practitionerToken, http.StatusCreated, &created)
	serviceID := created.Service.ID

	// A week out, so the booking is comfortably outside the change cutoff.
	day := time.Now().UTC().Add(7 * 24 * time.Hour).Truncate(24 * time.Hour)
	rig.mustJSON(t, http.MethodPut, "/v1/availability/rules", map[string]any{
		"rules": []map[string]any{{
			"weekday":       int(day.Weekday()),
			"windows":       []map[string]int{{"startMin": 540, "endMin": 720}},
			"bufferMinutes": 15,
		}},
	}, rig.practitionerToken, http.StatusOK, nil)

	// ---- 2. The client finds a slot and books ---------------------------

	// Anonymously, before signing in — this is the public booking page.
	var catalogue struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	rig.mustJSON(t, http.MethodGet, "/v1/services", nil, "", http.StatusOK, &catalogue)
	if len(catalogue.Items) != 1 || catalogue.Items[0].ID != serviceID {
		t.Fatalf("public catalogue = %+v, want the one service", catalogue.Items)
	}

	var slots struct {
		Slots []struct {
			StartAt time.Time `json:"startAt"`
		} `json:"slots"`
	}
	slotsPath := "/v1/availability/slots?serviceId=" + serviceID +
		"&from=" + day.Format("2006-01-02") + "&to=" + day.Format("2006-01-02") + "&tz=UTC"
	rig.mustJSON(t, http.MethodGet, slotsPath, nil, "", http.StatusOK, &slots)
	if len(slots.Slots) == 0 {
		t.Fatal("no slots offered on a day with availability")
	}
	start := slots.Slots[0].StartAt

	var booked struct {
		Booking struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"booking"`
	}
	rig.mustJSON(t, http.MethodPost, "/v1/bookings", map[string]any{
		"serviceId": serviceID, "startAt": start.Format(time.RFC3339), "tz": "UTC",
	}, rig.clientToken, http.StatusCreated, &booked)
	bookingID := booked.Booking.ID
	if booked.Booking.Status != "confirmed" {
		t.Fatalf("booking status = %q, want confirmed", booked.Booking.Status)
	}

	// The slot is gone from the public list, so nobody else is offered it.
	rig.mustJSON(t, http.MethodGet, slotsPath, nil, "", http.StatusOK, &slots)
	for _, slot := range slots.Slots {
		if slot.StartAt.Equal(start) {
			t.Fatal("the booked slot is still being offered")
		}
	}

	// ---- 3. Confirmation and reminder are queued ------------------------

	assertQueued(t, rig, notification.KindBookingConfirmation)
	assertQueued(t, rig, notification.KindSessionReminder)
	// The confirmation is due immediately; the reminder is not.
	assertDispatches(t, rig)

	// ---- 4. The client pays ---------------------------------------------

	var initialized struct {
		AuthorizationURL string `json:"authorizationUrl"`
		Reference        string `json:"reference"`
	}
	rig.mustJSON(t, http.MethodPost, "/v1/payments/initialize", map[string]any{
		"bookingId": bookingID,
	}, rig.clientToken, http.StatusOK, &initialized)
	if initialized.AuthorizationURL == "" || initialized.Reference == "" {
		t.Fatalf("initialize returned %+v, want a checkout URL and reference", initialized)
	}

	// Paystack calls the webhook. The body alone proves nothing — the
	// service re-verifies against the gateway before changing any state —
	// so a forged body with a valid signature still could not invent a
	// payment the gateway does not know about.
	payload := []byte(fmt.Sprintf(`{"event":"charge.success","data":{"reference":%q}}`, initialized.Reference))
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/paystack", bytes.NewReader(payload))
	req.Header.Set("x-paystack-signature", rig.gateway.SignWebhook(payload))
	whRec := httptest.NewRecorder()
	rig.srv.Router.ServeHTTP(whRec, req)
	if whRec.Code != http.StatusOK {
		t.Fatalf("webhook status = %d, body %s", whRec.Code, whRec.Body.String())
	}

	var paid struct {
		Items []struct {
			Status     string `json:"status"`
			AmountKobo int64  `json:"amountKobo"`
		} `json:"items"`
	}
	rig.mustJSON(t, http.MethodGet, "/v1/payments/mine", nil, rig.clientToken, http.StatusOK, &paid)
	if len(paid.Items) != 1 || paid.Items[0].Status != "success" {
		t.Fatalf("client's payments = %+v, want one successful payment", paid.Items)
	}
	if paid.Items[0].AmountKobo != created.Service.PriceKobo {
		t.Errorf("paid %d kobo, want the service price %d", paid.Items[0].AmountKobo, created.Service.PriceKobo)
	}

	// ---- 5. The session happens ------------------------------------------

	// The room is shut until shortly before the appointment, which is what
	// stops a client wandering in a week early.
	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/sessions/"+bookingID+"/join", nil,
		bearer(rig.clientToken))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("early join = %d, want 403 (body %s)", rec.Code, rec.Body.String())
	}

	// Move the appointment so it is under way, the way time would.
	moveBooking(t, rig, bookingID, 5*time.Minute)

	for _, who := range []struct {
		name  string
		token string
	}{{"client", rig.clientToken}, {"practitioner", rig.practitionerToken}} {
		var access struct {
			BookingID       string `json:"bookingId"`
			Ticket          string `json:"ticket"`
			TicketExpiresIn int    `json:"ticketExpiresIn"`
			Role            string `json:"role"`
			ICEServers      []struct {
				URLs []string `json:"urls"`
			} `json:"iceServers"`
		}
		rig.mustJSON(t, http.MethodPost, "/v1/sessions/"+bookingID+"/join", nil,
			who.token, http.StatusOK, &access)
		if access.Ticket == "" || access.BookingID != bookingID {
			t.Fatalf("%s got %+v, want a ticket for this booking", who.name, access)
		}
		if access.Role != who.name {
			t.Errorf("%s was given the role %q", who.name, access.Role)
		}
		// Single-use and short-lived: a ticket that leaked out of a URL
		// must not still open the room later.
		if access.TicketExpiresIn <= 0 || access.TicketExpiresIn > 120 {
			t.Errorf("%s ticket lives %ds, want a short TTL", who.name, access.TicketExpiresIn)
		}
		if len(access.ICEServers) == 0 {
			t.Errorf("%s got no ICE servers, so the browser cannot connect", who.name)
		}
	}

	// A stranger is told the session does not exist, not that it is not
	// theirs — the same rule as everywhere else.
	stranger := registerJourneyClient(t, rig, "koffi@example.com", "Koffi Mensah")
	rec = doJSON(t, rig.srv, http.MethodPost, "/v1/sessions/"+bookingID+"/join", nil,
		bearer(stranger))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("outsider join = %d, want 404", rec.Code)
	}

	// ---- 6. The practitioner closes the session and writes it up ---------

	// A session still under way cannot be completed — it has not finished.
	rec = doJSON(t, rig.srv, http.MethodPost, "/v1/bookings/"+bookingID+"/complete", nil,
		bearer(rig.practitionerToken))
	if rec.Code != http.StatusConflict {
		t.Fatalf("completing a session in progress = %d, want 409", rec.Code)
	}

	moveBooking(t, rig, bookingID, 2*time.Hour)
	rig.mustJSON(t, http.MethodPost, "/v1/bookings/"+bookingID+"/complete", nil,
		rig.practitionerToken, http.StatusOK, nil)

	rig.mustJSON(t, http.MethodPut, "/v1/bookings/"+bookingID+"/notes", map[string]any{
		"privateNotes":    "Mentioned a new job — watch the tension returning.",
		"sharedFeedback":  "Good session. Shoulders much freer by the end.",
		"sharedResources": []string{"Ten minutes of the stretch we did, daily."},
	}, rig.practitionerToken, http.StatusOK, nil)

	// Unshared, the client cannot see it — not even that it exists.
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/bookings/"+bookingID+"/notes", nil,
		bearer(rig.clientToken))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("client reading an unshared note = %d, want 404", rec.Code)
	}

	rig.mustJSON(t, http.MethodPost, "/v1/bookings/"+bookingID+"/notes/share", nil,
		rig.practitionerToken, http.StatusOK, nil)
	assertQueued(t, rig, notification.KindFeedbackShared)

	var clientNote struct {
		Note struct {
			SharedFeedback  string   `json:"sharedFeedback"`
			SharedResources []string `json:"sharedResources"`
			PrivateNotes    string   `json:"privateNotes"`
		} `json:"note"`
	}
	rig.mustJSON(t, http.MethodGet, "/v1/bookings/"+bookingID+"/notes", nil,
		rig.clientToken, http.StatusOK, &clientNote)
	if clientNote.Note.SharedFeedback == "" || len(clientNote.Note.SharedResources) != 1 {
		t.Errorf("client's copy = %+v, missing the parts meant for them", clientNote.Note)
	}
	// The private notes are the practitioner's own and stay that way.
	if clientNote.Note.PrivateNotes != "" {
		t.Errorf("client's copy carries private notes: %q", clientNote.Note.PrivateNotes)
	}

	// ---- 7. The client reviews it ----------------------------------------

	var review struct {
		Review struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"review"`
	}
	rig.mustJSON(t, http.MethodPost, "/v1/reviews", map[string]any{
		"bookingId": bookingID, "rating": 5, "comment": "I left feeling like a different person.",
	}, rig.clientToken, http.StatusCreated, &review)
	if review.Review.Status != "pending" {
		t.Fatalf("new review status = %q, want pending", review.Review.Status)
	}

	// Nothing appears publicly on the client's say-so.
	var public struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	rig.mustJSON(t, http.MethodGet, "/v1/content/reviews", nil, "", http.StatusOK, &public)
	if len(public.Items) != 0 {
		t.Fatalf("%d unapproved reviews are public, want none", len(public.Items))
	}

	rig.mustJSON(t, http.MethodPost, "/v1/admin/reviews/"+review.Review.ID+"/approve", nil,
		rig.practitionerToken, http.StatusOK, nil)

	rig.mustJSON(t, http.MethodGet, "/v1/content/reviews", nil, "", http.StatusOK, &public)
	if len(public.Items) != 1 {
		t.Fatalf("%d public reviews after approval, want 1", len(public.Items))
	}

	// ---- 8. It all shows up on the practice side -------------------------

	var record struct {
		Record struct {
			RecentBookings []struct {
				ID string `json:"id"`
			} `json:"recentBookings"`
			Payments struct {
				TotalPaidKobo int64 `json:"totalPaidKobo"`
				PaymentCount  int   `json:"paymentCount"`
			} `json:"payments"`
		} `json:"record"`
	}
	rig.mustJSON(t, http.MethodGet, "/v1/clients/"+rig.clientID, nil,
		rig.practitionerToken, http.StatusOK, &record)
	if len(record.Record.RecentBookings) != 1 || record.Record.RecentBookings[0].ID != bookingID {
		t.Errorf("client record shows %+v, want the one booking", record.Record.RecentBookings)
	}
	if record.Record.Payments.PaymentCount != 1 || record.Record.Payments.TotalPaidKobo != created.Service.PriceKobo {
		t.Errorf("client record payments = %+v, want one payment of %d kobo",
			record.Record.Payments, created.Service.PriceKobo)
	}

	var report struct {
		Summary struct {
			SessionsCompleted int   `json:"sessionsCompleted"`
			IncomeKobo        int64 `json:"incomeKobo"`
		} `json:"summary"`
		Reviews struct {
			Count int `json:"count"`
		} `json:"reviews"`
	}
	window := "from=" + time.Now().UTC().Add(-24*time.Hour).Format(time.RFC3339) +
		"&to=" + time.Now().UTC().Add(24*time.Hour).Format(time.RFC3339)
	rig.mustJSON(t, http.MethodGet, "/v1/admin/reports/practice?"+window, nil,
		rig.practitionerToken, http.StatusOK, &report)
	if report.Summary.SessionsCompleted != 1 {
		t.Errorf("report shows %d completed sessions, want 1", report.Summary.SessionsCompleted)
	}
	if report.Summary.IncomeKobo != created.Service.PriceKobo {
		t.Errorf("report income = %d, want %d", report.Summary.IncomeKobo, created.Service.PriceKobo)
	}
	// The approved review is the one that counts — moderation is upstream
	// of every number the practitioner is shown.
	if report.Reviews.Count != 1 {
		t.Errorf("report counts %d approved reviews, want 1", report.Reviews.Count)
	}
}

// TestTheJourneyRefusesToSkipSteps checks the gates in the order a client
// would hit them. Each of these is a step someone will eventually try to
// take out of turn, whether by curiosity, a stale tab, or on purpose.
func TestTheJourneyRefusesToSkipSteps(t *testing.T) {
	rig := newJourneyRig(t)

	var created struct {
		Service struct {
			ID string `json:"id"`
		} `json:"service"`
	}
	rig.mustJSON(t, http.MethodPost, "/v1/services", map[string]any{
		"name": "Massage", "durationMinutes": 60, "priceKobo": 25000,
	}, rig.practitionerToken, http.StatusCreated, &created)

	day := time.Now().UTC().Add(7 * 24 * time.Hour).Truncate(24 * time.Hour)
	rig.mustJSON(t, http.MethodPut, "/v1/availability/rules", map[string]any{
		"rules": []map[string]any{{
			"weekday":       int(day.Weekday()),
			"windows":       []map[string]int{{"startMin": 540, "endMin": 720}},
			"bufferMinutes": 0,
		}},
	}, rig.practitionerToken, http.StatusOK, nil)

	var booked struct {
		Booking struct {
			ID string `json:"id"`
		} `json:"booking"`
	}
	rig.mustJSON(t, http.MethodPost, "/v1/bookings", map[string]any{
		"serviceId": created.Service.ID,
		"startAt":   day.Add(9 * time.Hour).Format(time.RFC3339),
		"tz":        "UTC",
	}, rig.clientToken, http.StatusCreated, &booked)
	bookingID := booked.Booking.ID

	t.Run("a session cannot be reviewed before it has happened", func(t *testing.T) {
		rec := doJSON(t, rig.srv, http.MethodPost, "/v1/reviews", map[string]any{
			"bookingId": bookingID, "rating": 5,
		}, bearer(rig.clientToken))
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("review of a future session = %d, want 422", rec.Code)
		}
	})

	t.Run("a session cannot be completed before it has happened", func(t *testing.T) {
		rec := doJSON(t, rig.srv, http.MethodPost, "/v1/bookings/"+bookingID+"/complete", nil,
			bearer(rig.practitionerToken))
		if rec.Code != http.StatusConflict {
			t.Errorf("completing a future session = %d, want 409", rec.Code)
		}
	})

	t.Run("a client cannot write the practice's session note", func(t *testing.T) {
		rec := doJSON(t, rig.srv, http.MethodPut, "/v1/bookings/"+bookingID+"/notes", map[string]any{
			"privateNotes": "I was great",
		}, bearer(rig.clientToken))
		if rec.Code != http.StatusForbidden {
			t.Errorf("client writing a note = %d, want 403", rec.Code)
		}
	})

	t.Run("someone else's booking cannot be paid for", func(t *testing.T) {
		stranger := registerJourneyClient(t, rig, "koffi@example.com", "Koffi Mensah")
		rec := doJSON(t, rig.srv, http.MethodPost, "/v1/payments/initialize", map[string]any{
			"bookingId": bookingID,
		}, bearer(stranger))
		// 404, not 403: a stranger must not learn the booking exists.
		if rec.Code != http.StatusNotFound {
			t.Errorf("paying for another client's booking = %d, want 404", rec.Code)
		}
	})

	t.Run("the same booking cannot be reviewed twice", func(t *testing.T) {
		moveBooking(t, rig, bookingID, 2*time.Hour)
		rig.mustJSON(t, http.MethodPost, "/v1/bookings/"+bookingID+"/complete", nil,
			rig.practitionerToken, http.StatusOK, nil)
		rig.mustJSON(t, http.MethodPost, "/v1/reviews", map[string]any{
			"bookingId": bookingID, "rating": 5,
		}, rig.clientToken, http.StatusCreated, nil)

		rec := doJSON(t, rig.srv, http.MethodPost, "/v1/reviews", map[string]any{
			"bookingId": bookingID, "rating": 1,
		}, bearer(rig.clientToken))
		if rec.Code != http.StatusConflict {
			t.Errorf("second review = %d, want 409", rec.Code)
		}
	})
}

// registerJourneyClient signs a second client up and returns their access token.
func registerJourneyClient(t *testing.T, rig journeyRig, email, name string) string {
	t.Helper()
	var out struct {
		AccessToken string `json:"accessToken"`
	}
	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/auth/register", map[string]any{
		"email": email, "password": "correct horse battery", "name": name,
	}, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register %s = %d, body %s", email, rec.Code, rec.Body.String())
	}
	decodeBody(t, rec, &out)
	return out.AccessToken
}

// moveBooking shifts an appointment so that it started `ago` in the past,
// keeping its duration. Waiting for real time is not an option, and a clock
// injected into every service would be a much larger seam than the one
// thing this test needs.
//
//   - in progress (room open, not yet completable): ago < duration
//   - over (completable, reviewable):                ago > duration
func moveBooking(t *testing.T, rig journeyRig, bookingID string, ago time.Duration) {
	t.Helper()
	b, err := rig.bookings.FindByID(t.Context(), bookingID)
	if err != nil {
		t.Fatalf("find booking: %v", err)
	}
	duration := b.EndAt.Sub(b.StartAt)
	b.StartAt = time.Now().UTC().Add(-ago)
	b.EndAt = b.StartAt.Add(duration)
	if _, err = rig.bookings.Update(t.Context(), b); err != nil {
		t.Fatalf("move booking: %v", err)
	}
}

// assertQueued fails unless a job of that kind is in the outbox.
//
// Queued, not sent: notifications are durable jobs, and a reminder for a
// session a week out is queued now and dispatched then. Asserting on
// delivery would mean asserting that the reminder had already gone, which
// is exactly what must not happen.
func assertQueued(t *testing.T, rig journeyRig, kind notification.Kind) {
	t.Helper()
	if len(rig.jobs.OfKind(kind)) == 0 {
		t.Errorf("no %s job was queued; the outbox holds %d jobs", kind, len(rig.jobs.All()))
	}
}

// assertDispatches drains what is due and fails unless something was sent.
func assertDispatches(t *testing.T, rig journeyRig) {
	t.Helper()
	before := len(rig.mailer.Sent())
	if _, err := rig.notifications.DispatchDue(t.Context(), 20); err != nil {
		t.Fatalf("dispatch due: %v", err)
	}
	if len(rig.mailer.Sent()) == before {
		t.Error("nothing was dispatched, so the outbox never reaches the mailer")
	}
}
