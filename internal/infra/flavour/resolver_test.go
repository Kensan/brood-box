// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package flavour

import (
	"testing"

	domflavour "github.com/stacklok/apiary/pkg/domain/flavour"
)

func TestConventionResolver_Resolve(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		agentImage string
		flavour    domflavour.Name
		want       string
	}{
		{
			name:       "go flavour with tag",
			agentImage: "ghcr.io/stacklok/apiary/claude-code:latest",
			flavour:    domflavour.Go,
			want:       "ghcr.io/stacklok/apiary/claude-code-go:latest",
		},
		{
			name:       "python flavour with tag",
			agentImage: "ghcr.io/stacklok/apiary/codex:latest",
			flavour:    domflavour.Python,
			want:       "ghcr.io/stacklok/apiary/codex-python:latest",
		},
		{
			name:       "node flavour with tag",
			agentImage: "ghcr.io/stacklok/apiary/opencode:v1.0",
			flavour:    domflavour.Node,
			want:       "ghcr.io/stacklok/apiary/opencode-node:v1.0",
		},
		{
			name:       "rust flavour without tag",
			agentImage: "ghcr.io/stacklok/apiary/claude-code",
			flavour:    domflavour.Rust,
			want:       "ghcr.io/stacklok/apiary/claude-code-rust",
		},
		{
			name:       "generic returns unchanged",
			agentImage: "ghcr.io/stacklok/apiary/claude-code:latest",
			flavour:    domflavour.Generic,
			want:       "ghcr.io/stacklok/apiary/claude-code:latest",
		},
		{
			name:       "empty flavour returns unchanged",
			agentImage: "ghcr.io/stacklok/apiary/claude-code:latest",
			flavour:    "",
			want:       "ghcr.io/stacklok/apiary/claude-code:latest",
		},
		{
			name:       "image with port in registry",
			agentImage: "localhost:5000/apiary/claude-code:latest",
			flavour:    domflavour.Go,
			want:       "localhost:5000/apiary/claude-code-go:latest",
		},
		{
			name:       "simple image name with tag",
			agentImage: "claude-code:latest",
			flavour:    domflavour.Go,
			want:       "claude-code-go:latest",
		},
		{
			name:       "invalid flavour returns unchanged",
			agentImage: "ghcr.io/stacklok/apiary/claude-code:latest",
			flavour:    domflavour.Name("../../evil"),
			want:       "ghcr.io/stacklok/apiary/claude-code:latest",
		},
		{
			name:       "unknown flavour returns unchanged",
			agentImage: "ghcr.io/stacklok/apiary/claude-code:latest",
			flavour:    domflavour.Name("java"),
			want:       "ghcr.io/stacklok/apiary/claude-code:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := NewConventionResolver()
			got := r.Resolve(tt.agentImage, tt.flavour)
			if got != tt.want {
				t.Errorf("Resolve(%q, %q) = %q, want %q", tt.agentImage, tt.flavour, got, tt.want)
			}
		})
	}
}
