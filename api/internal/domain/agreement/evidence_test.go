package agreement

import (
	"testing"
	"time"
)

func TestEvidenceDigestSurvivesMongoMillisecondPrecision(t *testing.T) {
	sig := Signature{ContextID: "booking:one:participant:1", SignedName: "Guardian", SignedAt: time.Date(2026, 9, 23, 12, 0, 0, 123456789, time.UTC), AgreementBody: "Exact wording", AgreementVersion: 2, Acknowledged: true}
	sig.EvidenceHash = sig.Digest()
	sig.SignedAt = sig.SignedAt.Truncate(time.Millisecond)
	if !sig.VerifyIntegrity() {
		t.Fatal("Mongo timestamp precision changed digest")
	}
	sig.AgreementBody = "Different wording"
	if sig.VerifyIntegrity() {
		t.Fatal("wording tampering passed")
	}
}
