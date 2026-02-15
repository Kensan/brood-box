// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package review

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/stacklok/sandbox-agent/internal/domain/snapshot"
	"github.com/stacklok/sandbox-agent/internal/infra/diff"
	"github.com/stacklok/sandbox-agent/internal/infra/workspace"
)

// Flusher copies accepted changes from the snapshot back to the original
// workspace.
type Flusher interface {
	// Flush applies accepted changes from snapshotDir to originalDir.
	Flush(originalDir, snapshotDir string, accepted []snapshot.FileChange) error
}

// FSFlusher implements Flusher using filesystem operations.
type FSFlusher struct{}

// NewFSFlusher creates a new filesystem-based flusher.
func NewFSFlusher() *FSFlusher {
	return &FSFlusher{}
}

// Flush copies accepted added/modified files from snapshotDir to originalDir,
// and deletes files marked as Deleted from originalDir.
//
// Security: each target path is validated to be within originalDir, and each
// snapshot file's SHA-256 is re-verified against the hash recorded at diff time.
func (f *FSFlusher) Flush(originalDir, snapshotDir string, accepted []snapshot.FileChange) error {
	for _, ch := range accepted {
		targetPath := filepath.Join(originalDir, ch.RelPath)

		// Validate target path stays within original workspace bounds.
		if err := workspace.ValidateInBounds(originalDir, targetPath); err != nil {
			return fmt.Errorf("target path traversal rejected for %s: %w", ch.RelPath, err)
		}

		switch ch.Kind {
		case snapshot.Added, snapshot.Modified:
			snapPath := filepath.Join(snapshotDir, ch.RelPath)

			// Validate source path stays within snapshot bounds.
			if err := workspace.ValidateInBounds(snapshotDir, snapPath); err != nil {
				return fmt.Errorf("snapshot path traversal rejected for %s: %w", ch.RelPath, err)
			}

			// Re-verify hash before copying.
			currentHash, err := diff.HashFile(snapPath)
			if err != nil {
				return fmt.Errorf("re-hashing %s: %w", ch.RelPath, err)
			}
			if currentHash != ch.Hash {
				return fmt.Errorf("hash mismatch for %s: file modified between diff and flush", ch.RelPath)
			}

			// Ensure parent directory exists.
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("creating parent dir for %s: %w", ch.RelPath, err)
			}

			if err := copyFilePreserveMode(snapPath, targetPath); err != nil {
				return fmt.Errorf("flushing %s: %w", ch.RelPath, err)
			}

		case snapshot.Deleted:
			// Re-verify original file hash before deleting to detect
			// modifications in the real workspace since the diff.
			if ch.Hash != "" {
				currentHash, err := diff.HashFile(targetPath)
				if err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("re-hashing original %s: %w", ch.RelPath, err)
				}
				if currentHash != "" && currentHash != ch.Hash {
					return fmt.Errorf("hash mismatch for %s: original modified since diff, refusing to delete", ch.RelPath)
				}
			}
			if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("deleting %s: %w", ch.RelPath, err)
			}
		}
	}

	return nil
}

// copyFilePreserveMode copies src to dst preserving file permissions.
// Setuid/setgid bits are stripped for security.
func copyFilePreserveMode(src, dst string) error {
	sf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = sf.Close() }()

	info, err := sf.Stat()
	if err != nil {
		return err
	}

	// Strip setuid/setgid/sticky bits — only preserve rwx permissions.
	mode := info.Mode().Perm()

	df, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}

	if _, err := io.Copy(df, sf); err != nil {
		_ = df.Close()
		return err
	}

	// Explicitly close and return any error (e.g., NFS write-back failure).
	return df.Close()
}
