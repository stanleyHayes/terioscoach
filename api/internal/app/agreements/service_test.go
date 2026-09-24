package agreements

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/agreement"
	"github.com/xcreativs/terios/api/internal/domain/catalog"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
)

var fixedNow = time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)

// --- doubles -----------------------------------------------------------

type fakeRepo struct {
	agreements map[string]agreement.Agreement
	signatures map[string]agreement.Signature // key: clientID + "|" + agreementID
	created    int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		agreements: map[string]agreement.Agreement{},
		signatures: map[string]agreement.Signature{},
	}
}

func (f *fakeRepo) List(context.Context, string, bool) ([]agreement.Agreement, error) {
	out := make([]agreement.Agreement, 0, len(f.agreements))
	for _, a := range f.agreements {
		out = append(out, a)
	}
	return out, nil
}

func (f *fakeRepo) GetByID(_ context.Context, id string) (agreement.Agreement, error) {
	if a, ok := f.agreements[id]; ok {
		return a, nil
	}
	return agreement.Agreement{}, agreement.ErrAgreementNotFound
}

func (f *fakeRepo) GetByKey(_ context.Context, practitionerID, key string) (agreement.Agreement, error) {
	for _, a := range f.agreements {
		if a.PractitionerID == practitionerID && a.Key == key {
			return a, nil
		}
	}
	return agreement.Agreement{}, agreement.ErrAgreementNotFound
}

func (f *fakeRepo) Create(_ context.Context, a agreement.Agreement) (agreement.Agreement, error) {
	a.ID = fmt.Sprintf("agr-%d", len(f.agreements)+1)
	f.agreements[a.ID] = a
	return a, nil
}

func (f *fakeRepo) Update(_ context.Context, a agreement.Agreement) (agreement.Agreement, error) {
	f.agreements[a.ID] = a
	return a, nil
}

func (f *fakeRepo) CreateSignature(_ context.Context, sig agreement.Signature) (agreement.Signature, error) {
	key := sig.ClientID + "|" + sig.AgreementID
	if sig.ContextID != "" {
		key += fmt.Sprintf("|%s|%d|%s", sig.ContextID, sig.AgreementVersion, sig.SignerRole)
	}
	if existing, ok := f.signatures[key]; ok {
		return existing, nil
	}
	sig.ID = fmt.Sprintf("sig-%d", len(f.signatures)+1)
	f.signatures[key] = sig
	f.created++
	return sig, nil
}

func (f *fakeRepo) UpdateSignature(_ context.Context, sig agreement.Signature) (agreement.Signature, error) {
	for k, s := range f.signatures {
		if s.ID == sig.ID {
			f.signatures[k] = sig
			return sig, nil
		}
	}
	return agreement.Signature{}, agreement.ErrSignatureNotFound
}

func (f *fakeRepo) SignatureFor(_ context.Context, clientID, agreementID string) (agreement.Signature, error) {
	if sig, ok := f.signatures[clientID+"|"+agreementID]; ok {
		return sig, nil
	}
	return agreement.Signature{}, agreement.ErrAgreementNotFound
}

func (f *fakeRepo) SignatureByID(_ context.Context, id string) (agreement.Signature, error) {
	for _, sig := range f.signatures {
		if sig.ID == id {
			return sig, nil
		}
	}
	return agreement.Signature{}, agreement.ErrAgreementNotFound
}

func (f *fakeRepo) SignaturesForClient(_ context.Context, clientID string) ([]agreement.Signature, error) {
	var out []agreement.Signature
	for _, sig := range f.signatures {
		if sig.ClientID == clientID {
			out = append(out, sig)
		}
	}
	return out, nil
}

type fakeServices struct{ items map[string]catalog.Service }

func (f *fakeServices) Create(context.Context, catalog.Service) (catalog.Service, error) {
	return catalog.Service{}, nil
}

func (f *fakeServices) FindByID(_ context.Context, id string) (catalog.Service, error) {
	if s, ok := f.items[id]; ok {
		return s, nil
	}
	return catalog.Service{}, catalog.ErrServiceNotFound
}

func (f *fakeServices) ListByPractitioner(context.Context, string, bool) ([]catalog.Service, error) {
	return nil, nil
}

func (f *fakeServices) Update(context.Context, catalog.Service) (catalog.Service, error) {
	return catalog.Service{}, nil
}

func (f *fakeServices) Delete(context.Context, string) error { return nil }

func (f *fakeServices) HasBookings(context.Context, string) (bool, error) { return false, nil }

type recordingNotifier struct{ notices []ports.AgreementSignedNotice }

func (r *recordingNotifier) AgreementSigned(_ context.Context, n ports.AgreementSignedNotice) {
	r.notices = append(r.notices, n)
}

type recordingArchivist struct{ calls int }

func (r *recordingArchivist) ArchiveSignedAgreement(context.Context, agreement.Agreement, agreement.Signature) {
	r.calls++
}

// --- rig ---------------------------------------------------------------

type rig struct {
	svc       *Service
	repo      *fakeRepo
	services  *fakeServices
	notifier  *recordingNotifier
	archivist *recordingArchivist
}

// newRig wires one practice: a Holistic agreement covering two services
// ("holistic" and "holistic-initial"), plus an open intro service.
func newRig(t *testing.T) *rig {
	t.Helper()
	repo := newFakeRepo()
	a, err := agreement.New("prac-1", "holistic_coaching", "Holistic Coaching Agreement", "Terms.", false, fixedNow)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	a.ID = "agr-holistic"
	repo.agreements[a.ID] = a

	a2, err := agreement.New("prac-1", "sow", "Statement of Work", "SOW Terms.", true, fixedNow)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	a2.ID = "agr-sow"
	repo.agreements[a2.ID] = a2

	services := &fakeServices{items: map[string]catalog.Service{
		"svc-holistic":         {ID: "svc-holistic", AgreementID: "agr-holistic"},
		"svc-holistic-initial": {ID: "svc-holistic-initial", AgreementID: "agr-holistic"},
		"svc-multi":            {ID: "svc-multi", AgreementIDs: []string{"agr-holistic", "agr-sow"}},
		"svc-intro":            {ID: "svc-intro"},
		"svc-dangling":         {ID: "svc-dangling", AgreementID: "agr-deleted"},
	}}
	notifier := &recordingNotifier{}
	archivist := &recordingArchivist{}
	return &rig{
		svc: NewService(repo, services, Options{
			Notifier:  notifier,
			Archivist: archivist,
			Now:       func() time.Time { return fixedNow },
		}),
		repo:      repo,
		services:  services,
		notifier:  notifier,
		archivist: archivist,
	}
}

// --- tests -------------------------------------------------------------

func TestServiceWithNoAgreementIsAlwaysBookable(t *testing.T) {
	r := newRig(t)
	status, err := r.svc.StatusForService(context.Background(), "client-1", "svc-intro")
	if err != nil {
		t.Fatalf("StatusForService: %v", err)
	}
	if status.Agreement != nil || !status.Signed() {
		t.Error("an intro conversation should need no agreement")
	}
	if err := r.svc.RequireSigned(context.Background(), "client-1", "svc-intro"); err != nil {
		t.Errorf("RequireSigned = %v, want nil", err)
	}
}

func TestUnsignedAgreementBlocksTheBooking(t *testing.T) {
	r := newRig(t)
	err := r.svc.RequireSigned(context.Background(), "client-1", "svc-holistic")
	if !errors.Is(err, agreement.ErrAgreementRequired) {
		t.Fatalf("err = %v, want ErrAgreementRequired", err)
	}
	// The title travels with the error so the client is told which contract
	// they are being asked for.
	if !contains(err.Error(), "Holistic Coaching Agreement") {
		t.Errorf("err = %q, want it to name the agreement", err)
	}
}

// The whole point of the feature: one signature, then every service under
// that agreement is open — including the same one again.
func TestOneSignatureCoversEverySeviceUnderTheAgreement(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()

	if _, err := r.svc.Sign(ctx, ports.SignRequest{
		AgreementID: "agr-holistic",
		ClientID:    "client-1",
		ClientName:  "Daniel Baah",
		ClientEmail: "daniel@example.com",
		SignedName:  "Daniel Baah",
		BookingID:   "booking-1",
	}); err != nil {
		t.Fatalf("Sign: %v", err)
	}

	for _, serviceID := range []string{"svc-holistic", "svc-holistic-initial"} {
		if err := r.svc.RequireSigned(ctx, "client-1", serviceID); err != nil {
			t.Errorf("RequireSigned(%s) = %v, want nil", serviceID, err)
		}
	}
}

func TestAnotherClientIsStillAskedToSign(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	if _, err := r.svc.Sign(ctx, ports.SignRequest{
		AgreementID: "agr-holistic", ClientID: "client-1",
		ClientName: "Daniel", SignedName: "Daniel Baah",
	}); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if err := r.svc.RequireSigned(ctx, "client-2", "svc-holistic"); !errors.Is(err, agreement.ErrAgreementRequired) {
		t.Errorf("err = %v, want client-2 to be asked to sign", err)
	}
}

// A resubmitted booking form must not produce a second acceptance, nor a
// second email to the practice.
func TestSigningTwiceIsIdempotent(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	req := ports.SignRequest{
		AgreementID: "agr-holistic", ClientID: "client-1",
		ClientName: "Daniel", SignedName: "Daniel Baah",
	}
	first, err := r.svc.Sign(ctx, req)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	// An exact replay returns the original execution.
	second, err := r.svc.Sign(ctx, req)
	if err != nil {
		t.Fatalf("Sign again: %v", err)
	}
	if second.ID != first.ID || second.SignedName != first.SignedName {
		t.Errorf("second = %+v, want the original signature back", second)
	}
	if r.repo.created != 1 {
		t.Errorf("signatures created = %d, want 1", r.repo.created)
	}
	if len(r.notifier.notices) != 1 {
		t.Errorf("practice notified %d times, want 1", len(r.notifier.notices))
	}
	if r.archivist.calls != 1 {
		t.Errorf("archived %d times, want 1", r.archivist.calls)
	}
}

func TestSigningNotifiesAndArchives(t *testing.T) {
	r := newRig(t)
	if _, err := r.svc.Sign(context.Background(), ports.SignRequest{
		AgreementID: "agr-holistic", ClientID: "client-1",
		ClientName: "Daniel Baah", ClientEmail: "daniel@example.com",
		SignedName: "Daniel K. Baah",
	}); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if len(r.notifier.notices) != 1 {
		t.Fatalf("notices = %d, want 1", len(r.notifier.notices))
	}
	notice := r.notifier.notices[0]
	if notice.SignedName != "Daniel K. Baah" || notice.AgreementTitle != "Holistic Coaching Agreement" {
		t.Errorf("notice = %+v, want what was actually signed", notice)
	}
	if r.archivist.calls != 1 {
		t.Errorf("archived %d times, want 1", r.archivist.calls)
	}
}

// A retired agreement is not a hostage situation: the practice withdrew the
// wording, so it cannot be the reason a client is refused a booking.
func TestRetiredAgreementStopsBlockingBookings(t *testing.T) {
	r := newRig(t)
	no := false
	if _, err := r.svc.Update(context.Background(), "agr-holistic", ports.AgreementPatch{Active: &no}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if err := r.svc.RequireSigned(context.Background(), "client-1", "svc-holistic"); err != nil {
		t.Errorf("RequireSigned = %v, want nil once the agreement is retired", err)
	}
}

// A service pointing at an agreement that is gone is a misconfiguration on
// the practice's side; the client should not be the one who cannot book.
func TestDanglingAgreementReferenceDoesNotBlock(t *testing.T) {
	r := newRig(t)
	if err := r.svc.RequireSigned(context.Background(), "client-1", "svc-dangling"); err != nil {
		t.Errorf("RequireSigned = %v, want nil", err)
	}
}

func TestEditingTheTextRequiresCurrentVersionWithoutRewritingEvidence(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	if _, err := r.svc.Sign(ctx, ports.SignRequest{
		AgreementID: "agr-holistic", ClientID: "client-1",
		ClientName: "Daniel", SignedName: "Daniel Baah",
	}); err != nil {
		t.Fatalf("Sign: %v", err)
	}

	body := "Terms, revised."
	if _, err := r.svc.Update(ctx, "agr-holistic", ports.AgreementPatch{Body: &body}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if err := r.svc.RequireSigned(ctx, "client-1", "svc-holistic"); !errors.Is(err, agreement.ErrAgreementRequired) {
		t.Errorf("RequireSigned=%v, want current version required", err)
	}
	sigs, _ := r.svc.SignaturesForClient(ctx, "client-1")
	if len(sigs) != 1 || sigs[0].AgreementVersion != 1 {
		t.Fatalf("signature = %+v, want it pinned to version 1", sigs)
	}
	if sigs[0].AgreementBody != "Terms." {
		t.Errorf("AgreementBody = %q, want the wording as it stood when signed", sigs[0].AgreementBody)
	}
}

// The signed document has to show what was accepted, not what the practice
// has written since.
func TestTheSignedPDFShowsTheWordingThatWasSigned(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	sig, err := r.svc.Sign(ctx, ports.SignRequest{
		AgreementID: "agr-holistic", ClientID: "client-1",
		ClientName: "Daniel Baah", SignedName: "Daniel K. Baah",
	})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	revised := "Terms, entirely rewritten."
	if _, err := r.svc.Update(ctx, "agr-holistic", ports.AgreementPatch{Body: &revised}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	doc, err := r.svc.SignedDocument(ctx, identity.Identity{UserID: "client-1", Role: identity.RoleClient}, sig.ID)
	if err != nil {
		t.Fatalf("SignedDocument: %v", err)
	}
	body := string(doc.Data)
	if !strings.Contains(body, "Terms.") {
		t.Error("the PDF should carry the wording that was signed")
	}
	if strings.Contains(body, "entirely rewritten") {
		t.Error("the PDF must not carry wording written after the signature")
	}
	if !strings.Contains(doc.Filename, "daniel-baah") {
		t.Errorf("Filename = %q, want it to name the signatory", doc.Filename)
	}
}

// A client must not be able to pull down someone else's signed contract.
func TestAnotherClientCannotFetchTheDocument(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	sig, err := r.svc.Sign(ctx, ports.SignRequest{
		AgreementID: "agr-holistic", ClientID: "client-1",
		ClientName: "Daniel", SignedName: "Daniel Baah",
	})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	_, err = r.svc.SignedDocument(ctx, identity.Identity{UserID: "client-2", Role: identity.RoleClient}, sig.ID)
	if !errors.Is(err, agreement.ErrAgreementNotFound) {
		t.Errorf("err = %v, want it reported as missing, not forbidden", err)
	}
}

func TestThePractitionerCanFetchAnySignedDocument(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	sig, err := r.svc.Sign(ctx, ports.SignRequest{
		AgreementID: "agr-holistic", ClientID: "client-1",
		ClientName: "Daniel", SignedName: "Daniel Baah",
	})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if _, err := r.svc.SignedDocument(ctx, identity.Identity{UserID: "prac-1", Role: identity.RolePractitioner}, sig.ID); err != nil {
		t.Errorf("SignedDocument = %v, want nil", err)
	}
}

func TestSignRejectsAnEmptyName(t *testing.T) {
	r := newRig(t)
	_, err := r.svc.Sign(context.Background(), ports.SignRequest{
		AgreementID: "agr-holistic", ClientID: "client-1", SignedName: "  ",
	})
	if !errors.Is(err, agreement.ErrInvalidSignedName) {
		t.Errorf("err = %v, want ErrInvalidSignedName", err)
	}
}

func TestMultiAgreementServiceRequiresAllSigned(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()

	// Initial check - requires first agreement
	err := r.svc.RequireSigned(ctx, "client-1", "svc-multi")
	if !errors.Is(err, agreement.ErrAgreementRequired) {
		t.Fatalf("err = %v, want ErrAgreementRequired", err)
	}

	// Sign first agreement
	if _, err := r.svc.Sign(ctx, ports.SignRequest{
		AgreementID: "agr-holistic", ClientID: "client-1",
		ClientName: "Daniel", SignedName: "Daniel Baah",
	}); err != nil {
		t.Fatalf("Sign 1: %v", err)
	}

	// Still requires second agreement
	err = r.svc.RequireSigned(ctx, "client-1", "svc-multi")
	if !errors.Is(err, agreement.ErrAgreementRequired) {
		t.Fatalf("err = %v, want ErrAgreementRequired for 2nd agreement", err)
	}

	// Sign second agreement
	if _, err := r.svc.Sign(ctx, ports.SignRequest{
		AgreementID: "agr-sow", ClientID: "client-1",
		ClientName: "Daniel", SignedName: "Daniel Baah",
	}); err != nil {
		t.Fatalf("Sign 2: %v", err)
	}

	// Now both are signed
	if err := r.svc.RequireSigned(ctx, "client-1", "svc-multi"); err != nil {
		t.Errorf("RequireSigned = %v, want nil", err)
	}
}

func TestCountersignAgreement(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()

	sig, err := r.svc.Sign(ctx, ports.SignRequest{
		AgreementID: "agr-holistic", ClientID: "client-1",
		ClientName: "Daniel", SignedName: "Daniel Baah",
	})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	// Client cannot countersign
	_, err = r.svc.Countersign(ctx, identity.Identity{UserID: "client-1", Role: identity.RoleClient}, sig.ID, "Dr. Stanley Hayes")
	if !errors.Is(err, agreement.ErrAgreementNotFound) {
		t.Errorf("expected error for client countersign, got %v", err)
	}

	// Practitioner countersigns
	countersigned, err := r.svc.Countersign(ctx, identity.Identity{UserID: "prac-1", Role: identity.RolePractitioner}, sig.ID, "Dr. Stanley Hayes")
	if err != nil {
		t.Fatalf("Countersign: %v", err)
	}
	if countersigned.PractitionerSignedName != "Dr. Stanley Hayes" {
		t.Errorf("PractitionerSignedName = %q, want Dr. Stanley Hayes", countersigned.PractitionerSignedName)
	}
	if countersigned.PractitionerSignedAt == nil {
		t.Error("PractitionerSignedAt should be set")
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle ||
		len(needle) == 0 || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func TestCountersignRejectsExistingClientOnlySignature(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	sig, err := r.svc.Sign(ctx, ports.SignRequest{AgreementID: "agr-sow", ClientID: "client-1", SignedName: "Daniel Baah"})
	if err != nil {
		t.Fatal(err)
	}
	sig.RequiresCountersignature = true
	if _, err := r.repo.UpdateSignature(ctx, sig); err != nil {
		t.Fatal(err)
	}
	_, err = r.svc.Countersign(ctx, identity.Identity{UserID: "prac-1", Role: identity.RolePractitioner}, sig.ID, "Practitioner Name")
	if !errors.Is(err, agreement.ErrCountersignatureNotRequired) {
		t.Fatalf("got %v", err)
	}
	stored, err := r.repo.SignatureByID(ctx, sig.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.PractitionerSignedAt != nil {
		t.Fatal("rejected countersignature was persisted")
	}
}

// A newer completed SOW must download its own saved answers, while a legacy
// signature remains a separate historical record and is never silently rewritten.
func TestSOWDownloadKeepsHistoricalAndCompletedRecordsSeparate(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	old := agreement.Signature{ID: "legacy-sow", AgreementID: "agr-sow", AgreementKey: "nurse_sow", AgreementTitle: "Nurse Coaching Statement of Work", AgreementVersion: 1, AgreementBody: "CLIENT NAME:\n\nMONTHLY FEE:", ClientID: "client-1", SignedName: "Example Signer", SignedAt: fixedNow}
	completed := old
	completed.ID = "completed-sow"
	completed.ContextID = "booking:example:participant:1"
	completed.StatementOfWork = &agreement.StatementOfWork{ClientName: "Example Participant", EffectiveDate: "2026-10-01", Package: "Six sessions", MonthlyFee: "USD 150.00"}
	r.repo.signatures["old"] = old
	r.repo.signatures["new"] = completed
	caller := identity.Identity{UserID: "client-1", Role: identity.RoleClient}
	for _, tc := range []struct{ id, want string }{{old.ID, "Completed answers unavailable"}, {completed.ID, "Example Participant"}} {
		doc, err := r.svc.SignedDocument(ctx, caller, tc.id)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(doc.Data), tc.want) {
			t.Errorf("%s missing %q", tc.id, tc.want)
		}
	}
	if r.repo.signatures["old"].StatementOfWork != nil {
		t.Fatal("historical record was rewritten")
	}
}
