// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package flavour

import (
	"strings"

	domflavour "github.com/stacklok/apiary/pkg/domain/flavour"
)

// ConventionResolver resolves flavoured image names using a naming convention.
// Given "ghcr.io/stacklok/apiary/claude-code:latest" and flavour "go",
// it produces "ghcr.io/stacklok/apiary/claude-code-go:latest".
type ConventionResolver struct{}

// NewConventionResolver creates a new convention-based image resolver.
func NewConventionResolver() *ConventionResolver {
	return &ConventionResolver{}
}

// Resolve returns the flavoured image reference. For Generic or
// unrecognized flavour names, returns the original image unchanged.
func (r *ConventionResolver) Resolve(agentImage string, flavour domflavour.Name) string {
	if flavour == domflavour.Generic || flavour == "" || !flavour.IsValid() {
		return agentImage
	}

	// Split on the last colon to separate name from tag.
	// "ghcr.io/stacklok/apiary/claude-code:latest" → name="ghcr.io/stacklok/apiary/claude-code", tag="latest"
	name, tag := splitImageRef(agentImage)

	// Insert flavour suffix: "claude-code" → "claude-code-go"
	name = name + "-" + string(flavour)

	if tag != "" {
		return name + ":" + tag
	}
	return name
}

// splitImageRef splits an image reference into name and tag.
// Handles the case where the reference contains a port (e.g., "localhost:5000/image:tag").
func splitImageRef(ref string) (string, string) {
	// Find the last slash to isolate the image name portion.
	lastSlash := strings.LastIndex(ref, "/")

	// Look for a colon only after the last slash (to avoid splitting on port numbers).
	nameAndTag := ref
	prefix := ""
	if lastSlash >= 0 {
		prefix = ref[:lastSlash+1]
		nameAndTag = ref[lastSlash+1:]
	}

	if idx := strings.LastIndex(nameAndTag, ":"); idx >= 0 {
		return prefix + nameAndTag[:idx], nameAndTag[idx+1:]
	}
	return ref, ""
}
