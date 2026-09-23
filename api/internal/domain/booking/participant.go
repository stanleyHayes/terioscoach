package booking

import (
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrParticipantRequired = errors.New("participant declaration and accuracy acknowledgement required")
var ErrGuardianRequired = errors.New("guardian name, relationship and contact email required")

// Participant is an explicit appointment-date declaration, never inferred from an account.
// A nil Participant on a legacy booking means not recorded, not adult.
type Participant struct {
	Name                 string    `json:"name" bson:"name"`
	Under18              bool      `json:"under18" bson:"under18"`
	Accurate             bool      `json:"accurate" bson:"accurate"`
	GuardianName         string    `json:"guardianName,omitempty" bson:"guardianName,omitempty"`
	GuardianRelationship string    `json:"guardianRelationship,omitempty" bson:"guardianRelationship,omitempty"`
	GuardianEmail        string    `json:"guardianEmail,omitempty" bson:"guardianEmail,omitempty"`
	Revision             int       `json:"revision" bson:"revision"`
	DeclaredAt           time.Time `json:"declaredAt" bson:"declaredAt"`
	DeclaredBy           string    `json:"declaredBy" bson:"declaredBy"`
	AppointmentAt        time.Time `json:"appointmentAt" bson:"appointmentAt"`
}

func (p Participant) Validate() error {
	if strings.TrimSpace(p.Name) == "" || utf8.RuneCountInString(p.Name) > 120 || !p.Accurate {
		return ErrParticipantRequired
	}
	if p.Under18 {
		addr, err := mail.ParseAddress(p.GuardianEmail)
		if strings.TrimSpace(p.GuardianName) == "" || len(p.GuardianName) > 120 || strings.TrimSpace(p.GuardianRelationship) == "" || len(p.GuardianRelationship) > 120 || err != nil || addr.Address != p.GuardianEmail {
			return ErrGuardianRequired
		}
	}
	return nil
}
