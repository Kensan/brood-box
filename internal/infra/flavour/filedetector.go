// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

// Package flavour provides workspace toolchain detection and image resolution.
package flavour

import (
	"context"
	"os"
	"path/filepath"

	domflavour "github.com/stacklok/apiary/pkg/domain/flavour"
)

// markerRule maps filesystem markers to a flavour.
type markerRule struct {
	flavour domflavour.Name
	files   []string // any match triggers this flavour
}

// detectionRules are checked in priority order. The first match becomes
// the primary flavour; subsequent matches become secondary.
var detectionRules = []markerRule{
	{flavour: domflavour.Go, files: []string{"go.mod"}},
	{flavour: domflavour.Node, files: []string{"package.json"}},
	{flavour: domflavour.Python, files: []string{"pyproject.toml", "requirements.txt", "setup.py"}},
	{flavour: domflavour.Rust, files: []string{"Cargo.toml"}},
}

// FileDetector detects workspace flavour by checking for marker files.
type FileDetector struct{}

// NewFileDetector creates a new stat-based flavour detector.
func NewFileDetector() *FileDetector {
	return &FileDetector{}
}

// Detect scans the workspace root for marker files and returns the
// detected flavour(s). Returns Generic when no markers match.
func (d *FileDetector) Detect(_ context.Context, workspacePath string) (domflavour.Detection, error) {
	var primary domflavour.Name
	var secondary []domflavour.Name

	for _, rule := range detectionRules {
		if matchesAny(workspacePath, rule.files) {
			if primary == "" {
				primary = rule.flavour
			} else {
				secondary = append(secondary, rule.flavour)
			}
		}
	}

	if primary == "" {
		primary = domflavour.Generic
	}

	return domflavour.Detection{
		Primary:   primary,
		Secondary: secondary,
	}, nil
}

// matchesAny returns true if any of the given files exist in dir.
func matchesAny(dir string, files []string) bool {
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			return true
		}
	}
	return false
}
