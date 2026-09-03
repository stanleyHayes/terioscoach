package main

import "testing"

func TestSeedScope(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		input string
		want  string
		ok    bool
	}{
		{"", "all", true},
		{"all", "all", true},
		{"content", "content", true},
		{"catalog", "catalog", true},
		{"accounts", "", false},
	} {
		t.Run(test.input, func(t *testing.T) {
			got, err := seedScope(test.input)
			if (err == nil) != test.ok {
				t.Fatalf("seedScope(%q) error = %v, want ok %v", test.input, err, test.ok)
			}
			if got != test.want {
				t.Fatalf("seedScope(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestProductionCatalogUsesOnlySuppliedPricingFacts(t *testing.T) {
	t.Parallel()
	if len(productionServices) != 1 {
		t.Fatalf("production services = %d, want only the supplied obligation-free entry", len(productionServices))
	}
	service := productionServices[0]
	if service.priceMinor != 0 || service.durationMinutes != 30 || service.currency != "USD" {
		t.Fatalf("production service = %+v, want a free 30-minute USD introduction", service)
	}
}
