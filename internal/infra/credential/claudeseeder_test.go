// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package credential

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"testing"
)

func TestExtractExpiresAt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data string
		want int64
	}{
		{
			name: "valid credentials",
			data: `{"claudeAiOauth":{"accessToken":"tok","expiresAt":1773402187165}}`,
			want: 1773402187165,
		},
		{
			name: "missing expiresAt",
			data: `{"claudeAiOauth":{"accessToken":"tok"}}`,
			want: 0,
		},
		{
			name: "empty object",
			data: `{}`,
			want: 0,
		},
		{
			name: "invalid JSON",
			data: `not json`,
			want: 0,
		},
		{
			name: "empty input",
			data: ``,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractExpiresAt([]byte(tt.data))
			if got != tt.want {
				t.Errorf("extractExpiresAt() = %d, want %d", got, tt.want)
			}
		})
	}
}

// makeCreds returns a JSON credential blob with the given expiresAt value.
func makeCreds(t *testing.T, expiresAt int64) []byte {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"claudeAiOauth": map[string]any{
			"accessToken":  "sk-ant-oat01-test",
			"refreshToken": "sk-ant-ort01-test",
			"expiresAt":    expiresAt,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestSeedExpiry tests the expiry-aware credential seeding logic by calling
// ClaudeCodeSeeder.Seed with injected host credentials via the readHostCreds
// package-level variable.
// NOT parallel — subtests mutate the package-level timeNowMs and readHostCreds variables.
func TestSeedExpiry(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	origTimeNowMs := timeNowMs
	origReadHostCreds := readHostCreds
	t.Cleanup(func() {
		timeNowMs = origTimeNowMs
		readHostCreds = origReadHostCreds
	})

	seeder := NewClaudeCodeSeeder(logger)

	t.Run("seeds when no stored credentials exist", func(t *testing.T) {
		baseDir := t.TempDir()
		store := NewFSStore(baseDir, logger)
		hostCreds := makeCreds(t, 9999999999999)

		readHostCreds = func() ([]byte, string, error) {
			return hostCreds, "test", nil
		}

		if err := seeder.Seed(store); err != nil {
			t.Fatalf("Seed failed: %v", err)
		}

		data, err := store.ReadFile("claude-code", claudeCodeCredPath)
		if err != nil {
			t.Fatalf("expected seeded file: %v", err)
		}
		if extractExpiresAt(data) != 9999999999999 {
			t.Fatalf("expected expiresAt=9999999999999, got %d", extractExpiresAt(data))
		}
	})

	t.Run("keeps valid stored credentials", func(t *testing.T) {
		baseDir := t.TempDir()
		store := NewFSStore(baseDir, logger)

		// Pre-seed stored credentials with a far-future expiry.
		if err := store.SeedFile("claude-code", claudeCodeCredPath, makeCreds(t, 9999999999999)); err != nil {
			t.Fatal(err)
		}

		// Host has different credentials, but stored ones are still valid.
		readHostCreds = func() ([]byte, string, error) {
			return makeCreds(t, 8888888888888), "test", nil
		}
		timeNowMs = func() int64 { return 1000000000000 }

		if err := seeder.Seed(store); err != nil {
			t.Fatalf("Seed failed: %v", err)
		}

		data, err := store.ReadFile("claude-code", claudeCodeCredPath)
		if err != nil {
			t.Fatal(err)
		}
		if extractExpiresAt(data) != 9999999999999 {
			t.Fatal("stored credentials should not have been overwritten")
		}
	})

	t.Run("overwrites expired with fresher host creds", func(t *testing.T) {
		baseDir := t.TempDir()
		store := NewFSStore(baseDir, logger)

		// Store credentials that are already expired.
		if err := store.SeedFile("claude-code", claudeCodeCredPath, makeCreds(t, 1000000000000)); err != nil {
			t.Fatal(err)
		}

		timeNowMs = func() int64 { return 2000000000000 }
		readHostCreds = func() ([]byte, string, error) {
			return makeCreds(t, 3000000000000), "test", nil
		}

		if err := seeder.Seed(store); err != nil {
			t.Fatalf("Seed failed: %v", err)
		}

		data, err := store.ReadFile("claude-code", claudeCodeCredPath)
		if err != nil {
			t.Fatal(err)
		}
		if extractExpiresAt(data) != 3000000000000 {
			t.Fatalf("expected expiresAt=3000000000000, got %d", extractExpiresAt(data))
		}
	})

	t.Run("skips when host creds not fresher", func(t *testing.T) {
		baseDir := t.TempDir()
		store := NewFSStore(baseDir, logger)

		// Store credentials that are expired.
		if err := store.SeedFile("claude-code", claudeCodeCredPath, makeCreds(t, 1000000000000)); err != nil {
			t.Fatal(err)
		}

		timeNowMs = func() int64 { return 2000000000000 }
		// Host creds are older than stored.
		readHostCreds = func() ([]byte, string, error) {
			return makeCreds(t, 500000000000), "test", nil
		}

		if err := seeder.Seed(store); err != nil {
			t.Fatalf("Seed failed: %v", err)
		}

		data, err := store.ReadFile("claude-code", claudeCodeCredPath)
		if err != nil {
			t.Fatal(err)
		}
		if extractExpiresAt(data) != 1000000000000 {
			t.Fatal("stored credentials should not have been changed")
		}
	})

	t.Run("no-op when host creds unavailable", func(t *testing.T) {
		baseDir := t.TempDir()
		store := NewFSStore(baseDir, logger)

		readHostCreds = func() ([]byte, string, error) {
			return nil, "", fmt.Errorf("no credentials available")
		}

		if err := seeder.Seed(store); err != nil {
			t.Fatalf("Seed should return nil when host creds unavailable: %v", err)
		}

		_, err := store.ReadFile("claude-code", claudeCodeCredPath)
		if err == nil {
			t.Fatal("expected no stored credentials when host creds unavailable")
		}
	})
}
