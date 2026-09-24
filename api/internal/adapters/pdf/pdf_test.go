package pdf

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/agreement"
)

func TestBuildProducesAParseableFile(t *testing.T) {
	out := Build([]Block{Title("Hello"), Paragraph("World.")})

	if !bytes.HasPrefix(out, []byte("%PDF-1.4")) {
		t.Errorf("prefix = %q, want a PDF header", out[:8])
	}
	if !bytes.HasSuffix(bytes.TrimRight(out, "\n"), []byte("%%EOF")) {
		t.Error("file should end with an EOF marker")
	}
	for _, want := range []string{"/Type /Catalog", "/Type /Pages", "/Type /Page", "trailer", "startxref"} {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestBuildIncludesTeriosLetterhead(t *testing.T) {
	out := Build([]Block{Title("Agreement")})
	for _, want := range []string{"/Subtype /Image", "/ColorSpace /DeviceRGB", "/Logo 6 0 R", "/Logo Do"} {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("letterhead is missing %q", want)
		}
	}
}

// A cross-reference table whose offsets do not land on their objects is the
// one defect that makes a reader refuse the file outright, and it is
// invisible to eyeballing the output.
func TestCrossReferenceOffsetsPointAtTheirObjects(t *testing.T) {
	out := Build([]Block{Title("Offsets"), Paragraph(strings.Repeat("Filler. ", 400))})

	start := bytes.LastIndex(out, []byte("startxref"))
	if start < 0 {
		t.Fatal("no startxref")
	}
	var xrefAt int
	if _, err := fmt.Sscanf(string(out[start:]), "startxref\n%d", &xrefAt); err != nil {
		t.Fatalf("parse startxref: %v", err)
	}
	if xrefAt <= 0 || xrefAt >= len(out) || !bytes.HasPrefix(out[xrefAt:], []byte("xref")) {
		t.Fatalf("startxref = %d, does not point at the xref table", xrefAt)
	}

	entries := regexp.MustCompile(`(?m)^(\d{10}) 00000 n $`).FindAllStringSubmatch(string(out[xrefAt:]), -1)
	if len(entries) < 6 {
		t.Fatalf("xref entries = %d, want at least 6", len(entries))
	}
	for i, entry := range entries {
		offset, err := strconv.Atoi(entry[1])
		if err != nil {
			t.Fatalf("entry %d: %v", i, err)
		}
		want := fmt.Sprintf("%d 0 obj", i+1)
		if !bytes.HasPrefix(out[offset:], []byte(want)) {
			got := out[offset:min(offset+20, len(out))]
			t.Errorf("object %d: offset %d points at %q, want %q", i+1, offset, got, want)
		}
	}
}

func TestLongDocumentPaginates(t *testing.T) {
	var blocks []Block
	for i := range 200 {
		blocks = append(blocks, Paragraph(fmt.Sprintf("Clause %d. %s", i, strings.Repeat("Words words. ", 12))))
	}
	out := Build(blocks)
	if pages := bytes.Count(out, []byte("/Type /Page\n")) + bytes.Count(out, []byte("/Type /Page ")); pages < 5 {
		t.Errorf("pages = %d, want the document to flow onto several", pages)
	}
}

// Parentheses and backslashes end a PDF string literal early if they are
// not escaped, which corrupts every object after them.
func TestLiteralsAreEscaped(t *testing.T) {
	out := Build([]Block{Paragraph(`A (parenthesis) and a \ backslash`)})
	if !bytes.Contains(out, []byte(`A \(parenthesis\) and a \\ backslash`)) {
		t.Error("parentheses and backslashes should be escaped in the content stream")
	}
}

func TestSmartPunctuationSurvivesAsWinAnsi(t *testing.T) {
	// The agreements are full of curly quotes and em dashes; dropping them
	// silently would quietly alter the wording of a contract.
	got := encode("It’s here — really…")
	want := "It\x92s here \x97 really\x85"
	if got != want {
		t.Errorf("encode = %q, want %q", got, want)
	}
}

func TestUnmappableCharacterBecomesAVisibleStandIn(t *testing.T) {
	if got := encode("emoji \U0001F600"); !strings.HasSuffix(got, "?") {
		t.Errorf("encode = %q, want a trailing ?", got)
	}
}

func TestWrapBreaksAWordTooLongForTheColumn(t *testing.T) {
	long := strings.Repeat("A", 400)
	lines := wrap(long, Regular, 10.5, textWidth)
	if len(lines) < 2 {
		t.Fatalf("lines = %d, want the word broken across several", len(lines))
	}
	for i, line := range lines {
		if w := textLen(line, Regular, 10.5); w > textWidth+0.01 {
			t.Errorf("line %d width = %.2f, want <= %.2f", i, w, textWidth)
		}
	}
}

func TestSignedAgreementCarriesWhatWasSigned(t *testing.T) {
	a := agreement.Agreement{
		Title:   "Holistic Coaching Agreement",
		Body:    "## Terms\n\n1. The Client must be punctual.\n\nThe Coach is not a licensed nurse.",
		Version: 3,
	}
	sig := agreement.Signature{
		AgreementVersion: 3,
		ClientName:       "Daniel Baah",
		ClientEmail:      "daniel@example.com",
		SignedName:       "Daniel K. Baah",
		SignedAt:         time.Date(2026, 9, 11, 15, 4, 0, 0, time.UTC),
	}
	out := string(SignedAgreement(a, sig))

	for _, want := range []string{
		"Holistic Coaching Agreement",
		"The Client must be punctual",
		"not a licensed nurse",
		"Daniel K. Baah",
		"daniel@example.com",
		"11 September 2026",
		"Version 3",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("PDF is missing %q", want)
		}
	}
}

// The document is reproduced on demand rather than stored, so the same
// signature must always render to the same bytes.
func TestSignedAgreementIsDeterministic(t *testing.T) {
	a := agreement.Agreement{Title: "T", Body: "Terms.", Version: 1}
	sig := agreement.Signature{
		AgreementVersion: 1, ClientName: "D", SignedName: "D B",
		SignedAt: time.Date(2026, 9, 11, 15, 4, 0, 0, time.UTC),
	}
	if !bytes.Equal(SignedAgreement(a, sig), SignedAgreement(a, sig)) {
		t.Error("two renders of the same signature differ")
	}
}

func TestPendingCountersignatureOnlyAppearsOnCoachingAgreements(t *testing.T) {
	for _, key := range []string{"holistic_coaching", "nurse_coaching", "holistic_sow", "nurse_sow", "holistic_confidentiality_hipaa", "nurse_confidentiality_hipaa", "holistic_liability_release", "nurse_liability_release"} {
		want := key == "holistic_coaching" || key == "nurse_coaching"
		out := SignedAgreement(agreement.Agreement{Key: key, Title: "Document", Body: "Terms.", RequiresCountersignature: !want}, agreement.Signature{AgreementKey: key, SignedName: "Daniel Baah", RequiresCountersignature: !want})
		if bytes.Contains(out, []byte("Pending Practitioner Countersignature")) != want {
			t.Errorf("%s has incorrect pending signature block", key)
		}
		if !bytes.Contains(out, []byte("Daniel Baah")) {
			t.Errorf("%s lost client signature", key)
		}
	}
}

func TestDocumentSpecificExecutionBlocks(t *testing.T) {
	now := time.Date(2026, 9, 11, 15, 4, 0, 0, time.UTC)
	practitionerAt := now.Add(time.Hour)
	sig := agreement.Signature{
		AgreementKey:           "holistic_coaching",
		ClientName:             "Ama Serwaa",
		SignedName:             "Ama Serwaa",
		SignedAt:               now,
		PractitionerSignedName: "Dr. Stanley Hayes",
		PractitionerSignedAt:   &practitionerAt,
	}
	coaching := string(SignedAgreement(agreement.Agreement{Key: "holistic_coaching", Title: "Holistic Coaching Agreement", Body: "Terms."}, sig))
	for _, want := range []string{"IN WITNESS WHEREOF", "TERIOS WELLNESS SPA", "PRACTITIONER SIGNATURE", "Dr. Stanley Hayes", "CLIENT SIGNATURE", "OR I am the parent or legal guardian"} {
		if !strings.Contains(coaching, want) {
			t.Errorf("coaching PDF is missing %q", want)
		}
	}

	for _, key := range []string{"holistic_confidentiality_hipaa", "nurse_confidentiality_hipaa", "holistic_liability_release", "nurse_liability_release"} {
		out := string(SignedAgreement(agreement.Agreement{Key: key, Title: "Document", Body: "Terms."}, sig))
		if strings.Contains(out, "PRACTITIONER SIGNATURE") || !strings.Contains(out, "PRINTED NAME") || !strings.Contains(out, "CLIENT SIGNATURE") {
			t.Errorf("%s has the wrong client-only execution block", key)
		}
	}
}
