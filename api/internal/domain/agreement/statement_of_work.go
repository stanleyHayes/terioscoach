package agreement

import (
	"strings"
	"time"
	"unicode/utf8"
)

// StatementOfWork contains form answers, not an electronic signature.
type StatementOfWork struct {
	Package           string `json:"package,omitempty" bson:"package,omitempty"`
	ClientName        string `json:"clientName" bson:"clientName"`
	EffectiveDate     string `json:"effectiveDate" bson:"effectiveDate"`
	InitialTermMonths int    `json:"initialTermMonths" bson:"initialTermMonths"`
	MonthlyFee        string `json:"monthlyFee" bson:"monthlyFee"`
}

func IsStatementOfWork(key string) bool {
	return key == "holistic_sow" || key == "nurse_sow"
}

func (a Agreement) SubmitStatementOfWork(clientID, clientName, clientEmail, bookingID string, answers StatementOfWork, now time.Time) (Signature, error) {
	if !IsStatementOfWork(a.Key) {
		return Signature{}, ErrInvalidStatementOfWork
	}
	if !a.Active {
		return Signature{}, ErrAgreementInactive
	}
	if clientID == "" {
		return Signature{}, ErrInvalidClient
	}
	answers.ClientName = strings.TrimSpace(answers.ClientName)
	answers.EffectiveDate = strings.TrimSpace(answers.EffectiveDate)
	answers.MonthlyFee = strings.TrimSpace(answers.MonthlyFee)
	answers.Package = strings.TrimSpace(answers.Package)
	if answers.ClientName == "" || utf8.RuneCountInString(answers.ClientName) > MaxSignedNameLen || answers.MonthlyFee == "" || utf8.RuneCountInString(answers.MonthlyFee) > 120 {
		return Signature{}, ErrInvalidStatementOfWork
	}
	if a.Key == "nurse_sow" && answers.Package != "" {
		if len(strings.TrimSpace(answers.Package)) > 200 {
			return Signature{}, ErrInvalidStatementOfWork
		}
	} else if answers.InitialTermMonths < 1 || answers.InitialTermMonths > 1200 {
		return Signature{}, ErrInvalidStatementOfWork
	}
	if _, err := time.Parse("2006-01-02", answers.EffectiveDate); err != nil {
		return Signature{}, ErrInvalidStatementOfWork
	}
	submittedAt := now.UTC()
	return Signature{
		AgreementID: a.ID, AgreementKey: a.Key, AgreementTitle: a.Title,
		AgreementVersion: a.Version, AgreementBody: a.Body,
		ClientID: clientID, ClientName: clientName, ClientEmail: clientEmail,
		BookingID: strings.TrimSpace(bookingID), StatementOfWork: &answers, SubmittedAt: &submittedAt,
	}, nil
}
