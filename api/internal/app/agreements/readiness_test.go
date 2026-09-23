package agreements

import (
	"context"
	"errors"
	"github.com/xcreativs/terios/api/internal/domain/agreement"
	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/catalog"
	"github.com/xcreativs/terios/api/internal/domain/form"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
	"testing"
	"time"
)

func executionRig(t *testing.T, minor bool, key string) (*Service, *fakeRepo, *portstest.FakeBookingRepository, booking.Booking, agreement.Agreement) {
	t.Helper()
	ctx := context.Background()
	repo := newFakeRepo()
	a, err := agreement.New("practitioner", key, "Required document", "Original wording", false, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	a, err = repo.Create(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	services := &fakeServices{items: map[string]catalog.Service{"service": {ID: "service", PractitionerID: "practitioner", AgreementID: a.ID, Currency: "USD", PriceKobo: 25000}}}
	bookings := portstest.NewFakeBookingRepository()
	start := fixedNow.Add(time.Hour)
	b, err := bookings.Create(ctx, booking.Booking{ClientID: "client", PractitionerID: "practitioner", ServiceID: "service", Status: booking.StatusConfirmed, StartAt: start, EndAt: start.Add(time.Hour), Participant: &booking.Participant{Name: "Participant", Under18: minor, Accurate: true, GuardianName: "Parent", GuardianEmail: "parent@example.com", GuardianRelationship: "Parent", Revision: 1, AppointmentAt: start}})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(repo, services, Options{Bookings: bookings})
	svc.now = func() time.Time { return fixedNow }
	return svc, repo, bookings, b, a
}
func signedRequest(b booking.Booking, a agreement.Agreement) ports.SignRequest {
	req := ports.SignRequest{ClientID: b.ClientID, AgreementID: a.ID, BookingID: b.ID, AgreementVersion: a.Version, SignedName: "Participant", SignerRole: "client", Acknowledged: true}
	if b.Participant.Under18 {
		req.SignedName = "Parent"
		req.SignerRole = "guardian"
		req.ConsentVersion = "guardian_consent:v1"
	}
	return req
}
func TestMinorReadinessBindsParticipantAndConsentVersion(t *testing.T) {
	ctx := context.Background()
	svc, repo, bookings, b, a := executionRig(t, true, "liability")
	if err := svc.RequireBookingReady(ctx, b.ID); !errors.Is(err, agreement.ErrAgreementRequired) {
		t.Fatalf("unsigned ready: %v", err)
	}
	req := signedRequest(b, a)
	sig, err := svc.Sign(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := svc.Sign(ctx, req)
	if err != nil || duplicate.ID != sig.ID {
		t.Fatalf("retry failed: %v", err)
	}
	bad := req
	bad.SignedName = ""
	if _, err = svc.Sign(ctx, bad); err == nil {
		t.Fatal("idempotency bypassed validation")
	}
	guardian, err := svc.GuardianConsent(ctx, b.PractitionerID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Sign(ctx, signedRequest(b, guardian)); err != nil {
		t.Fatal(err)
	}
	if err = svc.RequireBookingReady(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.StatusesForBooking(ctx, identity.Identity{Role: identity.RoleClient, UserID: "other"}, b.ID); err == nil {
		t.Fatal("cross-client read")
	}
	if _, err = svc.UpdateGuardianConsent(ctx, b.PractitionerID, "Revised consent wording."); err != nil {
		t.Fatal(err)
	}
	if err = svc.RequireBookingReady(ctx, b.ID); !errors.Is(err, agreement.ErrAgreementRequired) {
		t.Fatal("old consent covered changed wording")
	}
	req.ConsentVersion = "guardian_consent:v2"
	newSig, err := svc.Sign(ctx, req)
	if err != nil || newSig.ID == sig.ID {
		t.Fatalf("new consent execution: %v", err)
	}
	original, _ := repo.SignatureByID(ctx, sig.ID)
	if original.ConsentVersion != "guardian_consent:v1" || original.ConsentBody != DefaultGuardianConsent {
		t.Fatal("old evidence changed")
	}
	participant := *b.Participant
	participant.Revision++
	participant.Name = "Another child"
	b.Participant = &participant
	if _, err = bookings.Update(ctx, b); err != nil {
		t.Fatal(err)
	}
	statuses, err := svc.StatusesForBooking(ctx, identity.Identity{Role: identity.RoleClient, UserID: b.ClientID}, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range statuses {
		if st.Signed() {
			t.Fatal("another child inherited signature")
		}
	}
}
func TestSignedSOWRequiresAnswersSignatureAndCatalogFee(t *testing.T) {
	for _, key := range []string{"holistic_sow", "nurse_sow"} {
		t.Run(key, func(t *testing.T) {
			ctx := context.Background()
			svc, repo, _, b, a := executionRig(t, false, key)
			req := signedRequest(b, a)
			req.StatementOfWork = &agreement.StatementOfWork{ClientName: b.Participant.Name, EffectiveDate: "2026-10-01", InitialTermMonths: 3, MonthlyFee: "USD 250.00"}
			if key == "nurse_sow" {
				req.StatementOfWork.Package = "Three-session package"
			}
			bad := req
			bad.Acknowledged = false
			if _, err := svc.Sign(ctx, bad); err == nil {
				t.Fatal("unsigned accepted")
			}
			answers := *req.StatementOfWork
			answers.MonthlyFee = "USD 1.00"
			bad = req
			bad.StatementOfWork = &answers
			if _, err := svc.Sign(ctx, bad); err == nil {
				t.Fatal("fee override accepted")
			}
			sig, err := svc.Sign(ctx, req)
			if err != nil {
				t.Fatal(err)
			}
			if sig.SignedAt.IsZero() || sig.EvidenceHash == "" || !sig.VerifyIntegrity() {
				t.Fatal("missing evidence")
			}
			req.StatementOfWork.MonthlyFee = "USD 9.00"
			stored, _ := repo.SignatureByID(ctx, sig.ID)
			if stored.StatementOfWork.MonthlyFee != "USD 250.00" {
				t.Fatal("answer snapshot mutated")
			}
			stored.SignedName = "Tampered"
			if stored.VerifyIntegrity() {
				t.Fatal("tampering undetected")
			}
		})
	}
}
func TestLegacyUnknownAndRescheduledDeclarationsDoNotGrantReadiness(t *testing.T) {
	ctx := context.Background()
	svc, _, bookings, b, _ := executionRig(t, false, "liability")
	b.Participant = nil
	_, _ = bookings.Update(ctx, b)
	if err := svc.RequireBookingReady(ctx, b.ID); !errors.Is(err, agreement.ErrAgreementRequired) {
		t.Fatal("unknown age passed")
	}
	b.Participant = &booking.Participant{Name: "Adult", Accurate: true, Revision: 1, AppointmentAt: b.StartAt.Add(-time.Hour)}
	_, _ = bookings.Update(ctx, b)
	if err := svc.RequireBookingReady(ctx, b.ID); !errors.Is(err, agreement.ErrAgreementRequired) {
		t.Fatal("old appointment declaration passed")
	}
}

func TestCountersignatureAndAssignedFormsBlockSessionAccess(t *testing.T) {
	ctx := context.Background()
	svc, _, _, b, a := executionRig(t, false, "holistic_coaching")
	sig, err := svc.Sign(ctx, signedRequest(b, a))
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.RequireBookingReady(ctx, b.ID); !errors.Is(err, agreement.ErrAgreementRequired) {
		t.Fatal("uncountersigned coaching agreement passed")
	}
	if _, err = svc.Countersign(ctx, identity.Identity{UserID: b.PractitionerID, Role: identity.RolePractitioner}, sig.ID, "Practitioner"); err != nil {
		t.Fatal(err)
	}
	if err = svc.RequireBookingReady(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	forms := portstest.NewFakeFormSubmissionRepository()
	svc.forms = forms
	if _, err = forms.Create(ctx, form.Submission{ClientID: b.ClientID, BookingID: b.ID, Status: form.StatusAssigned}); err != nil {
		t.Fatal(err)
	}
	if err = svc.RequireBookingReady(ctx, b.ID); !errors.Is(err, agreement.ErrAgreementRequired) {
		t.Fatal("assigned form did not block entry")
	}
}
