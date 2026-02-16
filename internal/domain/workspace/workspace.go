// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

// Package workspace defines domain interfaces and types for workspace
// snapshot management.
package workspace

import (
	"context"
	"os"

	"github.com/stacklok/sandbox-agent/internal/domain/snapshot"
)

// Snapshot holds references to the original and snapshot workspace paths.
type Snapshot struct {
	// OriginalPath is the real workspace directory.
	OriginalPath string

	// SnapshotPath is the COW clone directory.
	SnapshotPath string
}

// Cleanup removes the snapshot directory.
func (s *Snapshot) Cleanup() error {
	if s.SnapshotPath == "" {
		return nil
	}
	return os.RemoveAll(s.SnapshotPath)
}

// WorkspaceCloner creates workspace snapshots.
type WorkspaceCloner interface {
	// CreateSnapshot creates a COW snapshot of the workspace.
	CreateSnapshot(ctx context.Context, workspacePath string, matcher snapshot.Matcher) (*Snapshot, error)
}
