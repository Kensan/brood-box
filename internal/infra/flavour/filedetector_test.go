// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package flavour

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	domflavour "github.com/stacklok/brood-box/pkg/domain/flavour"
)

func TestFileDetector_Detect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		files         []string
		wantPrimary   domflavour.Name
		wantSecondary []domflavour.Name
	}{
		{
			name:        "go project",
			files:       []string{"go.mod"},
			wantPrimary: domflavour.Go,
		},
		{
			name:        "node project",
			files:       []string{"package.json"},
			wantPrimary: domflavour.Node,
		},
		{
			name:        "python project with pyproject.toml",
			files:       []string{"pyproject.toml"},
			wantPrimary: domflavour.Python,
		},
		{
			name:        "python project with requirements.txt",
			files:       []string{"requirements.txt"},
			wantPrimary: domflavour.Python,
		},
		{
			name:        "python project with setup.py",
			files:       []string{"setup.py"},
			wantPrimary: domflavour.Python,
		},
		{
			name:        "rust project",
			files:       []string{"Cargo.toml"},
			wantPrimary: domflavour.Rust,
		},
		{
			name:        "no markers returns generic",
			files:       []string{"README.md"},
			wantPrimary: domflavour.Generic,
		},
		{
			name:        "empty workspace returns generic",
			files:       nil,
			wantPrimary: domflavour.Generic,
		},
		{
			name:          "go + node multi-language",
			files:         []string{"go.mod", "package.json"},
			wantPrimary:   domflavour.Go,
			wantSecondary: []domflavour.Name{domflavour.Node},
		},
		{
			name:          "go + python + rust multi-language",
			files:         []string{"go.mod", "requirements.txt", "Cargo.toml"},
			wantPrimary:   domflavour.Go,
			wantSecondary: []domflavour.Name{domflavour.Python, domflavour.Rust},
		},
		{
			name:          "node + python (node wins by priority)",
			files:         []string{"package.json", "pyproject.toml"},
			wantPrimary:   domflavour.Node,
			wantSecondary: []domflavour.Name{domflavour.Python},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			for _, f := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, f), []byte{}, 0o644); err != nil {
					t.Fatalf("creating marker file %s: %v", f, err)
				}
			}

			d := NewFileDetector()
			got, err := d.Detect(context.Background(), dir)
			if err != nil {
				t.Fatalf("Detect() error: %v", err)
			}

			if got.Primary != tt.wantPrimary {
				t.Errorf("Primary = %q, want %q", got.Primary, tt.wantPrimary)
			}

			if len(got.Secondary) != len(tt.wantSecondary) {
				t.Fatalf("Secondary = %v, want %v", got.Secondary, tt.wantSecondary)
			}
			for i, want := range tt.wantSecondary {
				if got.Secondary[i] != want {
					t.Errorf("Secondary[%d] = %q, want %q", i, got.Secondary[i], want)
				}
			}
		})
	}
}
