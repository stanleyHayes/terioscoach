package mongodb

import "testing"

func TestStoredAgreementPolicyIsCorrectedWithoutRewritingEvidence(t *testing.T) {
	for _, key := range []string{"holistic_coaching", "nurse_coaching", "holistic_sow", "nurse_sow", "holistic_confidentiality_hipaa", "nurse_confidentiality_hipaa", "holistic_liability_release", "nurse_liability_release"} {
		want := key == "holistic_coaching" || key == "nurse_coaching"
		a := (agreementDoc{Key: key, RequiresCountersignature: !want, Body: "Original terms", Version: 3}).toDomain()
		s := (agreementSignatureDoc{AgreementKey: key, RequiresCountersignature: !want, SignedName: "Client Name", AgreementBody: "Original terms", PractitionerSignedName: "Historical signature"}).toDomain()
		if a.RequiresCountersignature != want || s.RequiresCountersignature != want {
			t.Errorf("%s kept stale flags", key)
		}
		if a.Body != "Original terms" || a.Version != 3 || s.AgreementBody != "Original terms" || s.SignedName != "Client Name" || s.PractitionerSignedName != "Historical signature" {
			t.Errorf("%s changed historical evidence", key)
		}
	}
}
