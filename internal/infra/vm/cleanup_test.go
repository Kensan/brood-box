// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package vm

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testLogger returns a logger that discards output unless tests are run with -v.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestCleanupStaleLogs(t *testing.T) {
	t.Parallel()

	vmsDir := filepath.Join(t.TempDir(), "vms")
	require.NoError(t, os.MkdirAll(vmsDir, 0o755))

	// Stale directory with dead PID sentinel — should be removed.
	staleDir := filepath.Join(vmsDir, "stale-vm")
	require.NoError(t, os.MkdirAll(staleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(staleDir, LogSentinel), []byte("2147483647"), 0o600))
	// Also create a log file to verify entire directory is removed.
	require.NoError(t, os.WriteFile(filepath.Join(staleDir, "broodbox.log"), []byte("old log"), 0o600))

	// Directory with live PID sentinel (our process) — should be preserved.
	liveDir := filepath.Join(vmsDir, "live-vm")
	require.NoError(t, os.MkdirAll(liveDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(liveDir, LogSentinel), []byte(fmt.Sprintf("%d", os.Getpid())), 0o600))

	// Directory without sentinel — should be preserved.
	noSentinelDir := filepath.Join(vmsDir, "no-sentinel-vm")
	require.NoError(t, os.MkdirAll(noSentinelDir, 0o755))

	// Regular file in vms/ (not a directory) — should be skipped.
	require.NoError(t, os.WriteFile(filepath.Join(vmsDir, "stray-file"), []byte("x"), 0o600))

	CleanupStaleLogs(vmsDir, testLogger())

	_, err := os.Stat(staleDir)
	assert.True(t, os.IsNotExist(err), "stale directory with dead PID should be removed")

	_, err = os.Stat(liveDir)
	assert.NoError(t, err, "directory with live PID should remain")

	_, err = os.Stat(noSentinelDir)
	assert.NoError(t, err, "directory without sentinel should remain")

	_, err = os.Stat(filepath.Join(vmsDir, "stray-file"))
	assert.NoError(t, err, "non-directory entry should remain")
}

func TestCleanupStaleLogs_InvalidSentinelContent(t *testing.T) {
	t.Parallel()

	vmsDir := filepath.Join(t.TempDir(), "vms")
	require.NoError(t, os.MkdirAll(vmsDir, 0o755))

	tests := []struct {
		name    string
		content string
	}{
		{"empty sentinel", ""},
		{"non-numeric text", "not-a-pid"},
		{"negative PID", "-1"},
		{"zero PID", "0"},
		{"floating point", "123.456"},
		{"PID with trailing garbage", "123abc"},
	}

	for _, tt := range tests {
		dir := filepath.Join(vmsDir, tt.name)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, LogSentinel), []byte(tt.content), 0o600))
	}

	CleanupStaleLogs(vmsDir, testLogger())

	// All directories with invalid sentinels should be preserved (not cleaned).
	for _, tt := range tests {
		dir := filepath.Join(vmsDir, tt.name)
		_, err := os.Stat(dir)
		assert.NoError(t, err, "directory with invalid sentinel %q should remain", tt.name)
	}
}

func TestCleanupStaleLogs_WhitespacePaddedSentinel(t *testing.T) {
	t.Parallel()

	vmsDir := filepath.Join(t.TempDir(), "vms")
	require.NoError(t, os.MkdirAll(vmsDir, 0o755))

	dir := filepath.Join(vmsDir, "whitespace-vm")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	// PID 2147483647 with leading/trailing whitespace and newline.
	require.NoError(t, os.WriteFile(filepath.Join(dir, LogSentinel), []byte("  2147483647\n"), 0o600))

	CleanupStaleLogs(vmsDir, testLogger())

	_, err := os.Stat(dir)
	assert.True(t, os.IsNotExist(err), "stale directory with whitespace-padded dead PID should be removed")
}

func TestCleanupStaleLogs_MultipleStaleDirectories(t *testing.T) {
	t.Parallel()

	vmsDir := filepath.Join(t.TempDir(), "vms")
	require.NoError(t, os.MkdirAll(vmsDir, 0o755))

	staleDirs := make([]string, 5)
	for i := range staleDirs {
		dir := filepath.Join(vmsDir, fmt.Sprintf("stale-%d", i))
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, LogSentinel), []byte("2147483647"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "broodbox.log"), []byte("log data"), 0o600))
		staleDirs[i] = dir
	}

	CleanupStaleLogs(vmsDir, testLogger())

	for _, dir := range staleDirs {
		_, err := os.Stat(dir)
		assert.True(t, os.IsNotExist(err), "stale directory %s should be removed", filepath.Base(dir))
	}
}

func TestCleanupStaleLogs_NestedDataSubdirectory(t *testing.T) {
	t.Parallel()

	vmsDir := filepath.Join(t.TempDir(), "vms")
	dir := filepath.Join(vmsDir, "nested-vm")
	dataDir := filepath.Join(dir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, LogSentinel), []byte("2147483647"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "broodbox.log"), []byte("log"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "state.json"), []byte("{}"), 0o600))

	CleanupStaleLogs(vmsDir, testLogger())

	_, err := os.Stat(dir)
	assert.True(t, os.IsNotExist(err), "entire directory tree should be removed")
}

func TestCleanupStaleLogs_EmptyVmsDir(t *testing.T) {
	t.Parallel()

	vmsDir := filepath.Join(t.TempDir(), "vms")
	require.NoError(t, os.MkdirAll(vmsDir, 0o755))

	// Should not panic or error on empty directory.
	CleanupStaleLogs(vmsDir, testLogger())
}

func TestCleanupStaleLogs_NonexistentVmsDir(t *testing.T) {
	t.Parallel()

	// Should not panic when vms/ doesn't exist at all.
	CleanupStaleLogs(filepath.Join(t.TempDir(), "nonexistent"), testLogger())
}

func TestWriteSentinel(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, WriteSentinel(dir))

	data, err := os.ReadFile(filepath.Join(dir, LogSentinel))
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%d", os.Getpid()), string(data))
}

func TestWriteSentinel_FilePermissions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, WriteSentinel(dir))

	info, err := os.Stat(filepath.Join(dir, LogSentinel))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
		"sentinel should be owner-only read/write")
}

func TestWriteSentinel_Overwrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Write a sentinel with stale content.
	require.NoError(t, os.WriteFile(filepath.Join(dir, LogSentinel), []byte("99999"), 0o600))

	// Overwrite with current PID.
	require.NoError(t, WriteSentinel(dir))

	data, err := os.ReadFile(filepath.Join(dir, LogSentinel))
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%d", os.Getpid()), string(data),
		"sentinel should contain current PID after overwrite")
}

func TestWriteSentinel_NonexistentDirectory(t *testing.T) {
	t.Parallel()

	err := WriteSentinel(filepath.Join(t.TempDir(), "nonexistent"))
	assert.Error(t, err, "writing sentinel to nonexistent directory should fail")
}
