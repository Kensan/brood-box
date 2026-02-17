// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package git

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "allowed sections pass through unchanged",
			input: strings.Join([]string{
				"[core]",
				"\trepositoryformatversion = 0",
				"\tfilemode = true",
				"\tbare = false",
				`[remote "origin"]`,
				"\turl = https://github.com/org/repo.git",
				"\tfetch = +refs/heads/*:refs/remotes/origin/*",
				`[branch "main"]`,
				"\tremote = origin",
				"\tmerge = refs/heads/main",
			}, "\n"),
			expected: strings.Join([]string{
				"[core]",
				"\trepositoryformatversion = 0",
				"\tfilemode = true",
				"\tbare = false",
				`[remote "origin"]`,
				"\turl = https://github.com/org/repo.git",
				"\tfetch = +refs/heads/*:refs/remotes/origin/*",
				`[branch "main"]`,
				"\tremote = origin",
				"\tmerge = refs/heads/main",
			}, "\n"),
		},
		{
			name: "blocked sections stripped entirely including alias with shell commands",
			input: strings.Join([]string{
				"[core]",
				"\tbare = false",
				"[alias]",
				"\tco = !git checkout",
				"\tst = status",
				"[credential]",
				"\thelper = store",
				"[color]",
				"\tui = auto",
			}, "\n"),
			expected: strings.Join([]string{
				"[core]",
				"\tbare = false",
				"[color]",
				"\tui = auto",
			}, "\n"),
		},
		{
			name: "case-insensitive blocking",
			input: strings.Join([]string{
				"[CREDENTIAL]",
				"\thelper = store",
				"[Http]",
				"\tsslVerify = false",
				"[core]",
				"\tbare = false",
			}, "\n"),
			expected: strings.Join([]string{
				"[core]",
				"\tbare = false",
			}, "\n"),
		},
		{
			name: "subsection does not affect section name blocking",
			input: strings.Join([]string{
				`[credential "https://github.com"]`,
				"\thelper = osxkeychain",
				"[core]",
				"\tbare = false",
			}, "\n"),
			expected: strings.Join([]string{
				"[core]",
				"\tbare = false",
			}, "\n"),
		},
		{
			name: "includeIf correctly blocked",
			input: strings.Join([]string{
				`[includeIf "gitdir:/home/user/work/"]`,
				"\tpath = /home/user/.gitconfig-work",
				"[core]",
				"\tbare = false",
			}, "\n"),
			expected: strings.Join([]string{
				"[core]",
				"\tbare = false",
			}, "\n"),
		},
		{
			name: "URL credentials removed from remote",
			input: strings.Join([]string{
				`[remote "origin"]`,
				"\turl = https://user:token@github.com/org/repo.git",
				"\tfetch = +refs/heads/*:refs/remotes/origin/*",
			}, "\n"),
			expected: strings.Join([]string{
				`[remote "origin"]`,
				"\turl = https://github.com/org/repo.git",
				"\tfetch = +refs/heads/*:refs/remotes/origin/*",
			}, "\n"),
		},
		{
			name: "SCP-like URLs pass through",
			input: strings.Join([]string{
				`[remote "origin"]`,
				"\turl = git@github.com:org/repo.git",
				"\tfetch = +refs/heads/*:refs/remotes/origin/*",
			}, "\n"),
			expected: strings.Join([]string{
				`[remote "origin"]`,
				"\turl = git@github.com:org/repo.git",
				"\tfetch = +refs/heads/*:refs/remotes/origin/*",
			}, "\n"),
		},
		{
			name: "malformed HTTPS URL with credentials fail closed",
			input: strings.Join([]string{
				`[remote "origin"]`,
				"\turl = http{s://user:pass@github.com/org/repo.git",
				"\tfetch = +refs/heads/*:refs/remotes/origin/*",
			}, "\n"),
			expected: strings.Join([]string{
				`[remote "origin"]`,
				"\tfetch = +refs/heads/*:refs/remotes/origin/*",
			}, "\n"),
		},
		{
			name: "user section name and email pass through signingkey stripped",
			input: strings.Join([]string{
				"[user]",
				"\tname = John Doe",
				"\temail = john@example.com",
				"\tsigningkey = ABCDEF1234567890",
			}, "\n"),
			expected: strings.Join([]string{
				"[user]",
				"\tname = John Doe",
				"\temail = john@example.com",
			}, "\n"),
		},
		{
			name: "backslash continuation stays in same section context",
			input: strings.Join([]string{
				"[core]",
				"\tpager = less \\",
				"\t-FRSX",
				"\tbare = false",
			}, "\n"),
			expected: strings.Join([]string{
				"[core]",
				"\tpager = less \\",
				"\t-FRSX",
				"\tbare = false",
			}, "\n"),
		},
		{
			name: "backslash continuation fake section inside continued value is not parsed as header",
			input: strings.Join([]string{
				"[core]",
				"\tpager = some-value \\",
				"[credential]",
				"\tbare = false",
			}, "\n"),
			expected: strings.Join([]string{
				"[core]",
				"\tpager = some-value \\",
				"[credential]",
				"\tbare = false",
			}, "\n"),
		},
		{
			name:     "empty input produces empty output",
			input:    "",
			expected: "",
		},
		{
			name: "comments and blank lines preserved in allowed sections dropped in blocked",
			input: strings.Join([]string{
				"[core]",
				"\t# This is a comment",
				"\tbare = false",
				"",
				"[credential]",
				"\t# Credential comment",
				"\thelper = store",
				"",
				"[color]",
				"\t; semicolon comment",
				"\tui = auto",
			}, "\n"),
			expected: strings.Join([]string{
				"[core]",
				"\t# This is a comment",
				"\tbare = false",
				"",
				"[color]",
				"\t; semicolon comment",
				"\tui = auto",
			}, "\n"),
		},
		{
			name: "unknown sections not in either list are dropped fail-safe",
			input: strings.Join([]string{
				"[core]",
				"\tbare = false",
				"[somethingnew]",
				"\tkey = value",
				"[color]",
				"\tui = auto",
			}, "\n"),
			expected: strings.Join([]string{
				"[core]",
				"\tbare = false",
				"[color]",
				"\tui = auto",
			}, "\n"),
		},
		{
			name: "submodule URL sanitization works like remote",
			input: strings.Join([]string{
				`[submodule "lib"]`,
				"\tpath = lib",
				"\turl = https://user:pass@github.com/org/lib.git",
			}, "\n"),
			expected: strings.Join([]string{
				`[submodule "lib"]`,
				"\tpath = lib",
				"\turl = https://github.com/org/lib.git",
			}, "\n"),
		},
		{
			name: "remote pushurl is also sanitized",
			input: strings.Join([]string{
				`[remote "origin"]`,
				"\turl = https://github.com/org/repo.git",
				"\tpushurl = https://deploy:token@github.com/org/repo.git",
			}, "\n"),
			expected: strings.Join([]string{
				`[remote "origin"]`,
				"\turl = https://github.com/org/repo.git",
				"\tpushurl = https://github.com/org/repo.git",
			}, "\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := SanitizeConfig(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProcess_ReadsAndWritesSanitizedConfig(t *testing.T) {
	t.Parallel()

	originalDir := t.TempDir()
	snapshotDir := t.TempDir()

	// Set up .git/config in the original workspace.
	gitDir := filepath.Join(originalDir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0o755))

	configContent := strings.Join([]string{
		"[core]",
		"\tbare = false",
		"[credential]",
		"\thelper = store",
		`[remote "origin"]`,
		"\turl = https://user:token@github.com/org/repo.git",
	}, "\n")
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "config"), []byte(configContent), 0o644))

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sanitizer := NewConfigSanitizer(logger)

	err := sanitizer.Process(context.Background(), originalDir, snapshotDir)
	require.NoError(t, err)

	// Read the sanitized config from the snapshot.
	result, err := os.ReadFile(filepath.Join(snapshotDir, ".git", "config"))
	require.NoError(t, err)

	expected := strings.Join([]string{
		"[core]",
		"\tbare = false",
		`[remote "origin"]`,
		"\turl = https://github.com/org/repo.git",
	}, "\n")
	assert.Equal(t, expected, string(result))
}

func TestProcess_NoGitConfig_NoOp(t *testing.T) {
	t.Parallel()

	originalDir := t.TempDir()
	snapshotDir := t.TempDir()

	// No .git directory at all.
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sanitizer := NewConfigSanitizer(logger)

	err := sanitizer.Process(context.Background(), originalDir, snapshotDir)
	require.NoError(t, err)

	// Snapshot should not have a .git directory.
	_, err = os.Stat(filepath.Join(snapshotDir, ".git", "config"))
	assert.True(t, os.IsNotExist(err))
}
