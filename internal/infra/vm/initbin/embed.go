// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

// Package initbin embeds the pre-compiled apiary-init binary.
// The binary is built by `task build-init` and placed at
// internal/infra/vm/initbin/apiary-init before compiling apiary.
package initbin

import _ "embed"

//go:embed apiary-init
var Binary []byte
