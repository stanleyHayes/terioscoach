package agreements

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/agreement"
	"github.com/xcreativs/terios/api/internal/domain/document"
	"github.com/xcreativs/terios/api/internal/ports"
)

// fakeDocuments records what was filed. Only StoreDocument is exercised;
// the rest of the port is present so the double satisfies it.
type fakeDocuments struct {
	stored []ports.StoreDocumentInput
	by     []string
	err    error
}

func (f *fakeDocuments) StoreDocument(_ context.Context, uploadedBy string, in ports.StoreDocumentInput) (document.Document, error) {
	if f.err != nil {
		return document.Document{}, f.err
	}
	f.stored = append(f.stored, in)
	f.by = append(f.by, uploadedBy)
	return document.Document{ID: "doc-1", Kind: in.Kind, ClientID: in.ClientID}, nil
}

func (f *fakeDocuments) SignUpload(context.Context, ports.UploadRequest) (ports.SignedUpload, error) {
	return ports.SignedUpload{}, nil
}
func (f *fakeDocuments) RecordUpload(context.Context, string, ports.RecordUploadInput) (document.Document, error) {
	return document.Document{}, nil
}
func (f *fakeDocuments) ListForClient(context.Context, string) ([]document.Document, error) {
	return nil, nil
}
func (f *fakeDocuments) ListCMSImages(context.Context) ([]ports.CMSImage, error) { return nil, nil }
func (f *fakeDocuments) UpdateDocument(context.Context, string, document.Patch) (document.Document, error) {
	return document.Document{}, nil
}
func (f *fakeDocuments) DeleteDocument(context.Context, string) error { return nil }
func (f *fakeDocuments) ListMine(context.Context, string) ([]document.Document, error) {
	return nil, nil
}
func (f *fakeDocuments) DownloadURLForClient(context.Context, string, string) (string, error) {
	return "", nil
}
func (f *fakeDocuments) DownloadURLForPractitioner(context.Context, string) (string, error) {
	return "", nil
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func signedFixture() (agreement.Agreement, agreement.Signature) {
	a := agreement.Agreement{
		ID: "agr-1", Key: "holistic_coaching",
		Title: "Holistic Coaching Agreement", Body: "Terms apply.", Version: 2,
	}
	sig := agreement.Signature{
		ID: "sig-1", AgreementID: a.ID, AgreementKey: a.Key,
		AgreementTitle: a.Title, AgreementVersion: 2, AgreementBody: a.Body,
		ClientID: "client-1", ClientName: "Daniel Baah", ClientEmail: "daniel@example.com",
		SignedName: "Daniel K. Baah",
		SignedAt:   time.Date(2026, 9, 11, 15, 4, 0, 0, time.UTC),
	}
	return a, sig
}

func TestArchivistFilesAPrivatePDFOnTheClientsRecord(t *testing.T) {
	docs := &fakeDocuments{}
	a, sig := signedFixture()

	NewDocumentArchivist(docs, quietLogger()).ArchiveSignedAgreement(context.Background(), a, sig)

	if len(docs.stored) != 1 {
		t.Fatalf("documents stored = %d, want 1", len(docs.stored))
	}
	in := docs.stored[0]
	// signed_form is private and never shared back to the client — this is
	// the practice's record, not a handout.
	if in.Kind != document.KindSignedForm {
		t.Errorf("Kind = %q, want %q", in.Kind, document.KindSignedForm)
	}
	if !in.Kind.Private() {
		t.Error("the archived copy must be a private kind")
	}
	if in.ClientID != "client-1" {
		t.Errorf("ClientID = %q, want client-1", in.ClientID)
	}
	if in.ContentType != "application/pdf" || !strings.HasSuffix(in.Filename, ".pdf") {
		t.Errorf("filed as %q / %q, want a PDF", in.Filename, in.ContentType)
	}
	if in.Title != "Holistic Coaching Agreement" {
		t.Errorf("Title = %q, want the agreement's", in.Title)
	}
	if !bytes.HasPrefix(in.Data, []byte("%PDF-")) {
		t.Error("the filed bytes are not a PDF")
	}
}

func TestArchivedPDFCarriesTheSignature(t *testing.T) {
	docs := &fakeDocuments{}
	a, sig := signedFixture()
	NewDocumentArchivist(docs, quietLogger()).ArchiveSignedAgreement(context.Background(), a, sig)

	body := string(docs.stored[0].Data)
	for _, want := range []string{"Holistic Coaching Agreement", "Daniel K. Baah", "11 September 2026", "Version 2"} {
		if !strings.Contains(body, want) {
			t.Errorf("archived PDF is missing %q", want)
		}
	}
}

// The signature is already recorded by the time this runs, and the same
// document can be re-rendered on demand — so a storage outage must not
// propagate anywhere.
func TestArchiveFailureIsContainedAndLogged(t *testing.T) {
	var logged bytes.Buffer
	docs := &fakeDocuments{err: errors.New("cloudinary is unreachable")}
	a, sig := signedFixture()

	archivist := NewDocumentArchivist(docs, slog.New(slog.NewTextHandler(&logged, nil)))
	archivist.ArchiveSignedAgreement(context.Background(), a, sig)

	if !strings.Contains(logged.String(), "archive signed agreement") {
		t.Errorf("a failed archive should be logged loudly; got %q", logged.String())
	}
}

// Signing must still work on a deployment with no media store configured.
func TestArchivistWithoutDocumentsIsANoOp(t *testing.T) {
	a, sig := signedFixture()
	NewDocumentArchivist(nil, quietLogger()).
		ArchiveSignedAgreement(context.Background(), a, sig)
}

// The whole point of wiring it up: signing files the copy, once.
func TestSigningFilesExactlyOneCopy(t *testing.T) {
	repo := newFakeRepo()
	a, err := agreement.New("prac-1", "holistic", "Holistic Coaching Agreement", "Terms.", false, fixedNow)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	a.ID = "agr-holistic"
	repo.agreements[a.ID] = a

	docs := &fakeDocuments{}
	svc := NewService(repo, &fakeServices{}, Options{
		Archivist: NewDocumentArchivist(docs, quietLogger()),
		Now:       func() time.Time { return fixedNow },
	})

	req := ports.SignRequest{
		AgreementID: "agr-holistic", ClientID: "client-1",
		ClientName: "Daniel", SignedName: "Daniel Baah",
	}
	if _, err := svc.Sign(context.Background(), req); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if _, err := svc.Sign(context.Background(), req); err != nil {
		t.Fatalf("Sign again: %v", err)
	}
	if len(docs.stored) != 1 {
		t.Errorf("copies filed = %d, want 1 — a repeat signature must not file a second", len(docs.stored))
	}
}
