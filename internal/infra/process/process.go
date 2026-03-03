// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

// Package process provides shared process-level utilities for infrastructure
// packages that need to check process liveness (e.g. stale cleanup).
package process

import (
	"os"
	"syscall"
)

// IsAlive checks if a process with the given PID is still running.
// Uses signal 0 which checks for process existence without sending a signal.
func IsAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 checks existence without actually sending a signal.
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
