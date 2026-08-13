package clients

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/client"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/payment"
	"github.com/xcreativs/terios/api/internal/ports"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

var fixedNow = time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC)

type testRig struct {
	svc       *Service
	profiles  *portstest.FakeClientProfileRepository
	users     *portstest.FakeUserRepository
	bookings  *portstest.FakeBookingRepository
	payments  *portstest.FakePaymentRepository
	documents *portstest.FakeDocumentCounter
	forms     *portstest.FakeFormSubmissionCounter
}

func newTestRig() testRig {
	rig := testRig{
		profiles:  portstest.NewFakeClientProfileRepository(),
		users:     portstest.NewFakeUserRepository(),
		bookings:  portstest.NewFakeBookingRepository(),
		payments:  portstest.NewFakePaymentRepository(),
		documents: &portstest.FakeDocumentCounter{Counts: map[string]int{}},
		forms:     &portstest.FakeFormSubmissionCounter{Counts: map[string]int{}},
	}
	rig.svc = NewService(rig.profiles, rig.users, rig.bookings, rig.payments, rig.documents, rig.forms)
	rig.svc.now = func() time.Time { return fixedNow }
	return rig
}

// seedUser creates an account; the fake assigns ids sequentially
// (user-1, user-2, ...).
func seedUser(t *testing.T, rig testRig, email, name string, role identity.Role) identity.User {
	t.Helper()
	u, err := identity.NewUser(email, name, "fakehash:x", role, fixedNow)
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	u, err = rig.users.Create(context.Background(), u)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

// seedBooking inserts a booking directly (bypassing slot validation — the
// rollup logic under test does not depend on it).
func seedBooking(t *testing.T, rig testRig, clientID, practitionerID string, startAt time.Time, status booking.Status) booking.Booking {
	t.Helper()
	b, err := booking.New(clientID, practitionerID, "svc-1", startAt, 60, fixedNow)
	if err != nil {
		t.Fatalf("booking.New: %v", err)
	}
	b, err = rig.bookings.Create(context.Background(), b)
	if err != nil {
		t.Fatalf("seed booking: %v", err)
	}
	if status != booking.StatusConfirmed {
		switch status {
		case booking.StatusCancelled:
			if err := b.Cancel(fixedNow); err != nil {
				t.Fatalf("cancel: %v", err)
			}
		case booking.StatusCompleted:
			b.Status = booking.StatusCompleted // terminal stamp; lifecycle is booking's own concern
		}
		b, err = rig.bookings.Update(context.Background(), b)
		if err != nil {
			t.Fatalf("update booking: %v", err)
		}
	}
	return b
}

func seedPayment(t *testing.T, rig testRig, bookingID, clientID string, amountKobo int64, status payment.Status, createdAt time.Time) {
	t.Helper()
	p := payment.Payment{
		BookingID:  bookingID,
		ClientID:   clientID,
		AmountKobo: amountKobo,
		Currency:   "GHS",
		Status:     status,
		CreatedAt:  createdAt,
	}
	if _, err := rig.payments.Create(context.Background(), p); err != nil {
		t.Fatalf("seed payment: %v", err)
	}
}

// TestListClientsRollup: only clients of this practice appear; the rollup
// counts non-cancelled bookings and takes the latest non-cancelled startAt.
func TestListClientsRollup(t *testing.T) {
	rig := newTestRig()
	prac := seedUser(t, rig, "prac@spa.com", "Practitioner", identity.RolePractitioner)
	otherPrac := seedUser(t, rig, "other@spa.com", "Other", identity.RolePractitioner)
	ana := seedUser(t, rig, "ana@mail.com", "Ana", identity.RoleClient)
	ben := seedUser(t, rig, "ben@mail.com", "Ben", identity.RoleClient)
	cara := seedUser(t, rig, "cara@mail.com", "Cara", identity.RoleClient)

	seedBooking(t, rig, ana.ID, prac.ID, fixedNow.Add(-72*time.Hour), booking.StatusCompleted)
	seedBooking(t, rig, ana.ID, prac.ID, fixedNow.Add(-24*time.Hour), booking.StatusConfirmed)
	seedBooking(t, rig, ana.ID, prac.ID, fixedNow.Add(-1*time.Hour), booking.StatusCancelled) // not a session
	seedBooking(t, rig, ben.ID, prac.ID, fixedNow.Add(-48*time.Hour), booking.StatusCompleted)
	seedBooking(t, rig, cara.ID, otherPrac.ID, fixedNow.Add(-12*time.Hour), booking.StatusCompleted) // different practice

	summaries, err := rig.svc.ListClients(context.Background(), prac.ID)
	if err != nil {
		t.Fatalf("ListClients: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("got %d clients, want 2 (cara belongs to another practice): %+v", len(summaries), summaries)
	}
	// Ordered by lastSessionAt descending: ana (-24h) before ben (-48h).
	if summaries[0].Profile.UserID != ana.ID || summaries[1].Profile.UserID != ben.ID {
		t.Fatalf("order = [%s %s], want [ana ben]", summaries[0].Profile.UserID, summaries[1].Profile.UserID)
	}
	if summaries[0].TotalSessions != 2 {
		t.Errorf("ana totalSessions = %d, want 2 (cancelled excluded)", summaries[0].TotalSessions)
	}
	if summaries[0].LastSessionAt == nil || !summaries[0].LastSessionAt.Equal(fixedNow.Add(-24*time.Hour)) {
		t.Errorf("ana lastSessionAt = %v, want the latest non-cancelled start", summaries[0].LastSessionAt)
	}
	if summaries[0].Name != "Ana" || summaries[0].Email != "ana@mail.com" {
		t.Errorf("ana summary = %+v, want identity joined", summaries[0])
	}
}

// TestGetClientRecordAssembly: the full record combines profile, identity,
// capped recent bookings (startAt desc), the payment rollup, and the
// document/form counts.
func TestGetClientRecordAssembly(t *testing.T) {
	rig := newTestRig()
	prac := seedUser(t, rig, "prac@spa.com", "Practitioner", identity.RolePractitioner)
	ana := seedUser(t, rig, "ana@mail.com", "Ana", identity.RoleClient)

	// Twelve bookings — the record caps at ten, newest first.
	for i := 0; i < 12; i++ {
		b := seedBooking(t, rig, ana.ID, prac.ID, fixedNow.Add(time.Duration(i)*time.Hour), booking.StatusCompleted)
		switch i {
		case 11:
			seedPayment(t, rig, b.ID, ana.ID, 25000, payment.StatusSuccess, fixedNow.Add(2*time.Hour))
		case 10:
			seedPayment(t, rig, b.ID, ana.ID, 10000, payment.StatusRefunded, fixedNow.Add(time.Hour))
		case 9:
			seedPayment(t, rig, b.ID, ana.ID, 5000, payment.StatusPending, fixedNow)
		}
	}
	rig.documents.Counts[ana.ID] = 3
	rig.forms.Counts[ana.ID] = 2

	rec, err := rig.svc.GetClientRecord(context.Background(), prac.ID, ana.ID)
	if err != nil {
		t.Fatalf("GetClientRecord: %v", err)
	}
	if rec.Name != "Ana" || rec.Email != "ana@mail.com" || rec.Profile.UserID != ana.ID {
		t.Errorf("record = %+v, want identity and linked profile", rec)
	}
	if len(rec.RecentBookings) != 10 {
		t.Fatalf("recentBookings = %d, want capped at 10", len(rec.RecentBookings))
	}
	for i := 1; i < len(rec.RecentBookings); i++ {
		if rec.RecentBookings[i-1].StartAt.Before(rec.RecentBookings[i].StartAt) {
			t.Fatal("recentBookings must be ordered by startAt descending")
		}
	}
	if rec.Payments.TotalPaidKobo != 25000 || rec.Payments.TotalRefundedKobo != 10000 || rec.Payments.PaymentCount != 3 {
		t.Errorf("payments = %+v, want paid 25000 / refunded 10000 / count 3", rec.Payments)
	}
	if rec.Payments.Currency != "GHS" {
		t.Errorf("currency = %q, want GHS from the most recent payment", rec.Payments.Currency)
	}
	if rec.DocumentCount != 3 || rec.FormSubmissionCount != 2 {
		t.Errorf("counts = %d docs / %d forms, want 3 / 2", rec.DocumentCount, rec.FormSubmissionCount)
	}
	// No profile written yet: zero practice fields.
	if rec.Profile.ID != "" || rec.Profile.PracticeNotes != "" || len(rec.Profile.Tags) != 0 {
		t.Errorf("profile = %+v, want empty (no PATCH yet)", rec.Profile)
	}
}

// TestClientIsolation: a user with no bookings at this practice is
// indistinguishable from an unknown id — 404, no leak.
func TestClientIsolation(t *testing.T) {
	rig := newTestRig()
	prac := seedUser(t, rig, "prac@spa.com", "Practitioner", identity.RolePractitioner)
	otherPrac := seedUser(t, rig, "other@spa.com", "Other", identity.RolePractitioner)
	cara := seedUser(t, rig, "cara@mail.com", "Cara", identity.RoleClient)
	seedBooking(t, rig, cara.ID, otherPrac.ID, fixedNow, booking.StatusCompleted)

	if _, err := rig.svc.GetClientRecord(context.Background(), prac.ID, cara.ID); !errors.Is(err, client.ErrClientNotFound) {
		t.Errorf("cross-practice record err = %v, want ErrClientNotFound", err)
	}
	if _, err := rig.svc.GetClientRecord(context.Background(), prac.ID, "no-such-user"); !errors.Is(err, client.ErrClientNotFound) {
		t.Errorf("unknown record err = %v, want ErrClientNotFound", err)
	}
	if _, err := rig.svc.UpdatePracticeFields(context.Background(), prac.ID, cara.ID, client.PracticePatch{}); !errors.Is(err, client.ErrClientNotFound) {
		t.Errorf("cross-practice patch err = %v, want ErrClientNotFound", err)
	}
}

// TestUpdatePracticeFieldsUpsert: the first PATCH creates the profile
// (stamped), a later partial PATCH keeps untouched fields.
func TestUpdatePracticeFieldsUpsert(t *testing.T) {
	rig := newTestRig()
	prac := seedUser(t, rig, "prac@spa.com", "Practitioner", identity.RolePractitioner)
	ana := seedUser(t, rig, "ana@mail.com", "Ana", identity.RoleClient)
	seedBooking(t, rig, ana.ID, prac.ID, fixedNow, booking.StatusConfirmed)

	phone := "+233 24 000 0000"
	notes := "prefers quiet room"
	tags := []string{"vip"}
	profile, err := rig.svc.UpdatePracticeFields(context.Background(), prac.ID, ana.ID, client.PracticePatch{
		Phone: &phone, PracticeNotes: &notes, Tags: &tags,
	})
	if err != nil {
		t.Fatalf("first patch: %v", err)
	}
	if profile.ID == "" || !profile.CreatedAt.Equal(fixedNow) || profile.Phone != phone || profile.PracticeNotes != notes {
		t.Errorf("profile = %+v, want created and stamped", profile)
	}

	// Partial second patch: only tags change, phone/notes survive.
	newTags := []string{"regular"}
	profile, err = rig.svc.UpdatePracticeFields(context.Background(), prac.ID, ana.ID, client.PracticePatch{Tags: &newTags})
	if err != nil {
		t.Fatalf("second patch: %v", err)
	}
	if len(profile.Tags) != 1 || profile.Tags[0] != "regular" {
		t.Errorf("tags = %v, want [regular]", profile.Tags)
	}
	if profile.Phone != phone || profile.PracticeNotes != notes {
		t.Errorf("profile = %+v, want untouched fields kept", profile)
	}
	if !profile.CreatedAt.Equal(fixedNow) {
		t.Errorf("createdAt = %v, want the original stamp kept across upserts", profile.CreatedAt)
	}
}

func TestUpdatePracticeFieldsValidation(t *testing.T) {
	rig := newTestRig()
	prac := seedUser(t, rig, "prac@spa.com", "Practitioner", identity.RolePractitioner)
	ana := seedUser(t, rig, "ana@mail.com", "Ana", identity.RoleClient)
	seedBooking(t, rig, ana.ID, prac.ID, fixedNow, booking.StatusConfirmed)

	long := "12345678901234567890123456789012345678901" // 41 chars
	if _, err := rig.svc.UpdatePracticeFields(context.Background(), prac.ID, ana.ID, client.PracticePatch{Phone: &long}); !errors.Is(err, client.ErrPhoneTooLong) {
		t.Errorf("err = %v, want ErrPhoneTooLong", err)
	}
	if _, err := rig.profiles.FindByUserID(context.Background(), ana.ID); !errors.Is(err, client.ErrProfileNotFound) {
		t.Errorf("profile after failed patch = %v, want still absent", err)
	}
}

// TestGetMe: the client's own view carries identity + phone, never the
// practice-side fields (they are not even on the return type).
func TestGetMe(t *testing.T) {
	rig := newTestRig()
	prac := seedUser(t, rig, "prac@spa.com", "Practitioner", identity.RolePractitioner)
	ana := seedUser(t, rig, "ana@mail.com", "Ana", identity.RoleClient)

	me, err := rig.svc.GetMe(context.Background(), ana.ID)
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	if me.ID != ana.ID || me.Name != "Ana" || me.Email != "ana@mail.com" || me.Phone != "" {
		t.Errorf("me = %+v, want identity with empty phone (no profile yet)", me)
	}
	if !me.CreatedAt.Equal(fixedNow) {
		t.Errorf("createdAt = %v, want the account's", me.CreatedAt)
	}

	seedBooking(t, rig, ana.ID, prac.ID, fixedNow, booking.StatusConfirmed)
	phone := "+233 24 000 0000"
	if _, err := rig.svc.UpdatePracticeFields(context.Background(), prac.ID, ana.ID, client.PracticePatch{Phone: &phone}); err != nil {
		t.Fatalf("patch: %v", err)
	}
	me, err = rig.svc.GetMe(context.Background(), ana.ID)
	if err != nil {
		t.Fatalf("GetMe after patch: %v", err)
	}
	if me.Phone != phone {
		t.Errorf("phone = %q, want %q", me.Phone, phone)
	}
}

// Compile-time guard: the port surface stays implemented.
var _ ports.ClientService = (*Service)(nil)
