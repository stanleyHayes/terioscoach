package pdf

import (
	"github.com/xcreativs/terios/api/internal/domain/agreement"
	"strings"
	"testing"
	"time"
)

func TestGuardianExecutionRetainsPractitionerCountersignature(t *testing.T) {
	at := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	sig := agreement.Signature{SignerRole: "guardian", ParticipantName: "Child Participant", GuardianName: "Guardian Name", SignedName: "Guardian Signature", GuardianRelationship: "Parent", GuardianEmail: "parent@example.com", ConsentBody: "Exact accepted wording", ConsentVersion: "guardian_consent:v2", SignedAt: at, PractitionerSignedName: "Practitioner Name", PractitionerSignedAt: &at}
	blocks := executionBlocks("holistic_coaching", sig)
	var text strings.Builder
	for _, block := range blocks {
		text.WriteString(block.Text)
		text.WriteByte('\n')
	}
	for _, want := range []string{"Child Participant", "Guardian Name", "Guardian Signature", "Exact accepted wording", "guardian_consent:v2", "PRACTITIONER SIGNATURE", "Practitioner Name", "NOT IDENTITY VERIFIED"} {
		if !strings.Contains(text.String(), want) {
			t.Errorf("missing %q", want)
		}
	}
}
