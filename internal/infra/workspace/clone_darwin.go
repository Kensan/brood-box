// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package workspace

import (
	"errors"

	"golang.org/x/sys/unix"
)

// darwinCloner uses clonefile(2) for APFS COW with regular copy fallback.
type darwinCloner struct{}

// NewPlatformCloner returns a FileCloner that uses macOS clonefile for COW.
func NewPlatformCloner() FileCloner {
	return &darwinCloner{}
}

// CloneFile attempts clonefile(2) for COW, falling back to regular copy.
func (c *darwinCloner) CloneFile(src, dst string) error {
	err := unix.Clonefile(src, dst, unix.CLONE_NOFOLLOW)
	if err == nil {
		return nil
	}

	// Fallback for non-APFS volumes, cross-device, or destination exists.
	if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EXDEV) || errors.Is(err, unix.EEXIST) {
		return copyFile(src, dst)
	}

	return err
}
