// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package flavour

// ImageResolver maps an agent image reference and flavour to the
// appropriate flavoured image reference.
type ImageResolver interface {
	// Resolve returns the image reference for the given base agent image
	// and flavour. Returns the original image unchanged for Generic.
	Resolve(agentImage string, flavour Name) string
}
