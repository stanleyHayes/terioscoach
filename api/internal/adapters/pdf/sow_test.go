package pdf

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/agreement"
)

func TestCompletedSOWContainsSavedAnswersWithoutDuplicateEmptyPrompts(t *testing.T) {
	for _, key := range []string{"nurse_sow", "holistic_sow"} {
		t.Run(key, func(t *testing.T) {
			body := "## STATEMENT OF WORK\n\nCLIENT NAME:\n\nSERVICE START DATE:\n\nPACKAGE\n\nMONTHLY FEE:\n\nAdditional agreed terms remain unchanged."
			sig := agreement.Signature{AgreementBody: body, AgreementVersion: 3, SignedName: "Alex Example", SignedAt: time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC), StatementOfWork: &agreement.StatementOfWork{ClientName: "Alex Example", EffectiveDate: "2026-10-01", Package: "Six coaching sessions", InitialTermMonths: 3, MonthlyFee: "USD 150.00"}}
			out := SignedAgreement(agreement.Agreement{Key: key, Title: "Statement of Work", Body: body}, sig)
			for _, want := range []string{"Alex Example", "2026-10-01", "Six coaching sessions", "USD 150.00", "Additional agreed terms remain unchanged.", "accepted and executed electronically"} {
				if !bytes.Contains(out, []byte(want)) {
					t.Errorf("PDF missing %q", want)
				}
			}
			for _, blank := range []string{"(CLIENT NAME:)", "(SERVICE START DATE:)", "(MONTHLY FEE:)"} {
				if bytes.Contains(out, []byte(blank)) {
					t.Errorf("PDF repeats empty prompt %q", blank)
				}
			}
			if sig.AgreementBody != body {
				t.Fatal("changed evidence snapshot")
			}
		})
	}
}

func TestSOWTermsPreserveNonemptyFieldsAndOtherTerms(t *testing.T) {
	body := "CLIENT NAME: Example\n\nINITIAL TERM: [INSERT NUMBER OF MONTHS]\n\nMONTHLY FEE: Subject to agreed changes."
	got := sowTerms(body)
	if strings.Contains(got, "[INSERT") || !strings.Contains(got, "CLIENT NAME: Example") || !strings.Contains(got, "Subject to agreed changes.") {
		t.Fatalf("unexpected terms %q", got)
	}
}

func TestHistoricalSOWWithoutAnswersIsExplicitlyIncomplete(t *testing.T) {
	out := SignedAgreement(agreement.Agreement{Key: "nurse_sow", Title: "Statement of Work"}, agreement.Signature{})
	if !bytes.Contains(out, []byte("Completed answers unavailable")) {
		t.Fatal("missing historical-record explanation")
	}
}

func TestElectronicExecutionNoticeUsesApprovedWording(t *testing.T) {
	want := "This agreement was accepted and executed electronically. The signatures recorded above are electronic signatures displayed in italicized format and are associated with the version of this agreement presented in this document."
	if got := electronicExecutionNotice().Text; got != want {
		t.Fatalf("notice = %q", got)
	}
}
