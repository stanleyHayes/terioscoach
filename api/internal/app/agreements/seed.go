package agreements

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"strings"

	"github.com/xcreativs/terios/api/internal/domain/agreement"
)

// The practice's two agreements, embedded as the text they were extracted
// from the signed Word originals. They are the starting wording only: the
// practitioner edits them in the dashboard from then on, and every edit
// bumps the version, so what this file says is never retroactively imposed
// on a client who has already signed.
//
//go:embed seed/*.txt
var seedFS embed.FS

// SeedAgreement is one starting agreement.
type SeedAgreement struct {
	Key   string
	Title string
	File  string
}

// SeedAgreements is what a fresh practice starts with.
var SeedAgreements = []SeedAgreement{
	{Key: "holistic_coaching", Title: "Holistic Coaching Agreement", File: "seed/holistic-coaching.txt"},
	{Key: "nurse_coaching", Title: "Nurse Coaching Agreement", File: "seed/nurse-coaching.txt"},
}

// Seed creates any of the starting agreements the practice does not have
// yet, and leaves every existing one exactly as it is.
//
// Re-running is safe and is the point: the practitioner's edits are the
// source of truth once made, and a deploy must never quietly restore the
// original wording under a client who signed the revised one.
func (s *Service) Seed(ctx context.Context, practitionerID string) ([]agreement.Agreement, error) {
	var created []agreement.Agreement
	for _, seed := range SeedAgreements {
		if _, err := s.repo.GetByKey(ctx, practitionerID, seed.Key); err == nil {
			continue
		} else if !errors.Is(err, agreement.ErrAgreementNotFound) {
			return nil, fmt.Errorf("look up %s: %w", seed.Key, err)
		}

		body, err := seedFS.ReadFile(seed.File)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", seed.File, err)
		}
		a, err := agreement.New(practitionerID, seed.Key, seed.Title, strings.TrimSpace(string(body)), s.now())
		if err != nil {
			return nil, fmt.Errorf("build %s: %w", seed.Key, err)
		}
		stored, err := s.repo.Create(ctx, a)
		if err != nil {
			return nil, fmt.Errorf("create %s: %w", seed.Key, err)
		}
		created = append(created, stored)
	}
	return created, nil
}
