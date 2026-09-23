package agreement

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

// The digest excludes mutable delivery and countersignature metadata.
func (s Signature) Digest() string {
	evidence := struct {
		ContextID, AgreementID, Body, ClientID, ActorID, ParticipantName, GuardianName, GuardianRelationship, GuardianEmail, Role, SignedName, SignedAt, ConsentBody, ConsentVersion string
		Version                                                                                                                                                                      int
		Acknowledged                                                                                                                                                                 bool
		Answers                                                                                                                                                                      *StatementOfWork
	}{s.ContextID, s.AgreementID, s.AgreementBody, s.ClientID, s.ActorID, s.ParticipantName, s.GuardianName, s.GuardianRelationship, s.GuardianEmail, s.SignerRole, s.SignedName, s.SignedAt.UTC().Truncate(time.Millisecond).Format(time.RFC3339Nano), s.ConsentBody, s.ConsentVersion, s.AgreementVersion, s.Acknowledged, s.StatementOfWork}
	raw, _ := json.Marshal(evidence)
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}
func (s Signature) VerifyIntegrity() bool {
	return s.EvidenceHash == "" || s.EvidenceHash == s.Digest()
}
