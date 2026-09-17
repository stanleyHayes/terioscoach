package httpapi

import (
	"github.com/xcreativs/terios/api/internal/domain/agreement"
	"net/http"
	"testing"
)

func TestAgreementResponsesCorrectLegacyCountersignatureFlags(t *testing.T) {
	for _, key := range []string{"holistic_coaching", "nurse_coaching", "holistic_sow", "nurse_sow", "holistic_confidentiality_hipaa", "nurse_confidentiality_hipaa", "holistic_liability_release", "nurse_liability_release"} {
		want := key == "holistic_coaching" || key == "nurse_coaching"
		a := newAgreementBody(agreement.Agreement{Key: key, RequiresCountersignature: !want})
		s := newAgreementSignatureBody(agreement.Signature{AgreementKey: key, RequiresCountersignature: !want})
		if a.RequiresCountersignature != want || s.RequiresCountersignature != want {
			t.Errorf("%s exposes stale flags", key)
		}
	}
}

func TestCountersignatureNotRequiredIsConflict(t *testing.T) {
	response, ok := mapAgreementError(agreement.ErrCountersignatureNotRequired)
	if !ok || response.status != http.StatusConflict {
		t.Fatalf("unexpected response: %+v", response)
	}
}
