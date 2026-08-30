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
