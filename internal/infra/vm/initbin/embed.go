// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

// Package initbin embeds the pre-compiled bbox-init binary.
// The binary is built by `task build-init` and placed at
// internal/infra/vm/initbin/bbox-init before compiling bbox.
package initbin

import _ "embed"

//go:embed bbox-init
var Binary []byte
