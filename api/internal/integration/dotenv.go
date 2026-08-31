// Package integration holds tests that run against the real external
// services — Atlas, Cloudflare, Cloudinary, Resend, Stripe — using the
// credentials in api/.env.
//
// Everything else in this repository is tested against fakes, and that is
// the right default: fakes are fast, deterministic, and need no accounts.
// What they cannot tell you is whether the adapter and the provider agree
// about anything. A bson tag that does not match, a signature computed
// over the wrong parameter set, an endpoint that moved — all of those are
// invisible to a fake and fatal in production.
//
// These tests are OFF unless TERIOS_INTEGRATION=1. They cost real network
// calls, they need credentials CI does not have, and one of them writes to
// a real database. `go test ./...` must stay clean without them.
package integration

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// loadEnv reads a .env file into a map, without touching the process
// environment.
//
// It does not set os.Setenv on purpose. A test that mutates the ambient
// environment leaks into every test that runs after it in the same binary,
// and the failure looks like flakiness rather than like a bug.
func loadEnv(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		// Quotes are stripped, matching what a shell would do. Nothing in
		// this project's env files is quoted, but a value pasted from a
		// dashboard sometimes arrives that way.
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

// repoEnv loads api/.env relative to this package.
func repoEnv() (map[string]string, error) {
	// internal/integration → api
	path := filepath.Join("..", "..", ".env")
	values, err := loadEnv(path)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", path, err)
	}
	return values, nil
}

// placeholder reports whether a value is still an unfilled template, so a
// test skips with a useful message instead of failing with a confusing
// authentication error.
func placeholder(value string) bool {
	return value == "" || (strings.Contains(value, "<") && strings.Contains(value, ">"))
}
