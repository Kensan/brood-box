// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

// Package version provides build-time version information.
package version

// Version and Commit are set via ldflags at build time.
var (
	Version = "dev"
	Commit  = "unknown"
)
