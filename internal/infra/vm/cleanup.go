// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package vm

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/stacklok/brood-box/internal/infra/process"
)

// LogSentinel is the marker file placed inside per-VM log directories
// to identify ownership by a running bbox process.
const LogSentinel = ".bbox-sentinel"

// CleanupStaleLogs removes orphaned per-VM log directories from previous
// crashes. It scans vmsDir for subdirectories with a sentinel file whose
// owning process has died.
func CleanupStaleLogs(vmsDir string, logger *slog.Logger) {
	entries, err := os.ReadDir(vmsDir)
	if err != nil {
		// Directory may not exist yet on first run — not an error.
		if os.IsNotExist(err) {
			return
		}
		logger.Warn("failed to scan for stale VM log directories", "error", err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirPath := filepath.Join(vmsDir, entry.Name())

		// Only remove directories that have our sentinel file to avoid
		// deleting unrelated directories.
		sentinelPath := filepath.Join(dirPath, LogSentinel)
		data, err := os.ReadFile(sentinelPath)
		if err != nil {
			logger.Debug("skipping VM directory without sentinel", "path", dirPath)
			continue
		}

		// If the sentinel contains a PID, check if that process is still alive.
		// Skip cleanup for directories owned by a running process.
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil || pid <= 0 {
			logger.Debug("skipping VM directory with invalid sentinel", "path", dirPath)
			continue
		}

		if process.IsAlive(pid) {
			logger.Debug("skipping VM log directory owned by running process",
				"path", dirPath, "pid", pid)
			continue
		}

		logger.Warn("removing stale VM log directory", "path", dirPath)
		if err := os.RemoveAll(dirPath); err != nil {
			logger.Error("failed to remove stale VM log directory", "path", dirPath, "error", err)
		}
	}
}

// WriteSentinel writes a PID sentinel file into the given directory to mark
// ownership by the current process. Returns an error if the write fails.
func WriteSentinel(dir string) error {
	sentinelPath := filepath.Join(dir, LogSentinel)
	content := fmt.Sprintf("%d", os.Getpid())
	if err := os.WriteFile(sentinelPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("writing log sentinel: %w", err)
	}
	return nil
}
