// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoader_Load_FileNotExist(t *testing.T) {
	t.Parallel()
	loader := NewLoader("/nonexistent/path/config.yaml")
	cfg, err := loader.Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Zero(t, cfg.Defaults.CPUs)
	assert.Zero(t, cfg.Defaults.Memory)
	assert.Nil(t, cfg.Agents)
}

func TestLoader_Load_ValidConfig(t *testing.T) {
	t.Parallel()

	content := `
defaults:
  cpus: 4
  memory: 4096

agents:
  claude-code:
    env_forward:
      - ANTHROPIC_API_KEY
      - "CLAUDE_*"
      - GITHUB_TOKEN
  my-custom-agent:
    image: ghcr.io/me/my-agent:latest
    command: ["my-agent", "--interactive"]
    env_forward:
      - MY_API_KEY
    cpus: 2
    memory: 1024
`

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	loader := NewLoader(path)
	cfg, err := loader.Load()
	require.NoError(t, err)

	assert.Equal(t, uint32(4), cfg.Defaults.CPUs)
	assert.Equal(t, uint32(4096), cfg.Defaults.Memory)

	require.Contains(t, cfg.Agents, "claude-code")
	cc := cfg.Agents["claude-code"]
	assert.Equal(t, []string{"ANTHROPIC_API_KEY", "CLAUDE_*", "GITHUB_TOKEN"}, cc.EnvForward)

	require.Contains(t, cfg.Agents, "my-custom-agent")
	custom := cfg.Agents["my-custom-agent"]
	assert.Equal(t, "ghcr.io/me/my-agent:latest", custom.Image)
	assert.Equal(t, []string{"my-agent", "--interactive"}, custom.Command)
	assert.Equal(t, uint32(2), custom.CPUs)
	assert.Equal(t, uint32(1024), custom.Memory)
}

func TestLoader_Load_InvalidYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("{{invalid yaml"), 0o644))

	loader := NewLoader(path)
	_, err := loader.Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing config file")
}

func TestLoader_DefaultPath(t *testing.T) {
	t.Parallel()
	loader := NewLoader("")
	assert.Contains(t, loader.Path(), "sandbox-agent")
	assert.Contains(t, loader.Path(), "config.yaml")
}
