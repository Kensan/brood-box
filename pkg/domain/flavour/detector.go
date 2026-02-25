// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package flavour

import "context"

// Detector scans a workspace to determine its toolchain flavour.
type Detector interface {
	// Detect inspects the workspace at the given path and returns
	// the detected flavour(s). Returns Generic when no markers match.
	Detect(ctx context.Context, workspacePath string) (Detection, error)
}
