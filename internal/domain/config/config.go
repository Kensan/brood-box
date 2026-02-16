// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

// Package config defines configuration types for sandbox-agent.
// All types are pure data with no I/O dependencies.
package config

import "github.com/stacklok/sandbox-agent/internal/domain/agent"

// LocalConfigFile is the per-workspace config file name.
const LocalConfigFile = ".sandbox-agent.yaml"

// Config is the top-level user configuration.
type Config struct {
	// Defaults specifies default resource limits.
	Defaults DefaultsConfig `yaml:"defaults"`

	// Review configures workspace snapshot isolation.
	Review ReviewConfig `yaml:"review"`

	// Agents maps agent names to configuration overrides.
	Agents map[string]AgentOverride `yaml:"agents"`
}

// ReviewConfig configures workspace snapshot isolation and review behavior.
type ReviewConfig struct {
	// Enabled controls whether snapshot isolation is active.
	// When nil, defaults to true (enabled).
	Enabled *bool `yaml:"enabled,omitempty"`

	// ExcludePatterns are additional gitignore-style patterns to exclude
	// from the workspace snapshot.
	ExcludePatterns []string `yaml:"exclude_patterns,omitempty"`
}

// DefaultsConfig specifies default VM resource limits.
type DefaultsConfig struct {
	// CPUs is the default number of vCPUs.
	CPUs uint32 `yaml:"cpus"`

	// Memory is the default RAM in MiB.
	Memory uint32 `yaml:"memory"`
}

// AgentOverride allows users to override built-in agent settings.
type AgentOverride struct {
	// Image overrides the OCI image reference.
	Image string `yaml:"image,omitempty"`

	// Command overrides the entrypoint command.
	Command []string `yaml:"command,omitempty"`

	// EnvForward overrides the env forwarding patterns.
	EnvForward []string `yaml:"env_forward,omitempty"`

	// CPUs overrides the vCPU count.
	CPUs uint32 `yaml:"cpus,omitempty"`

	// Memory overrides the RAM in MiB.
	Memory uint32 `yaml:"memory,omitempty"`
}

// MergeConfigs merges a local (per-workspace) config into a global config.
// Rules:
//   - Scalars (CPUs, Memory): local overrides global when non-zero.
//   - Review.Enabled: local value is IGNORED (security constraint).
//   - Review.ExcludePatterns: additive (global + local).
//   - Agents map: local extends/overrides global per key.
//
// Returns global unchanged when local is nil.
func MergeConfigs(global, local *Config) *Config {
	if local == nil {
		return global
	}

	result := *global

	// Scalars: local overrides global when non-zero.
	if local.Defaults.CPUs > 0 {
		result.Defaults.CPUs = local.Defaults.CPUs
	}
	if local.Defaults.Memory > 0 {
		result.Defaults.Memory = local.Defaults.Memory
	}

	// Review.Enabled: local value is IGNORED (global preserved).
	// Review.ExcludePatterns: additive.
	if len(global.Review.ExcludePatterns) > 0 || len(local.Review.ExcludePatterns) > 0 {
		result.Review.ExcludePatterns = append(
			append([]string{}, global.Review.ExcludePatterns...),
			local.Review.ExcludePatterns...,
		)
	}

	// Agents: local extends/overrides global per key.
	if len(local.Agents) > 0 {
		if result.Agents == nil {
			result.Agents = make(map[string]AgentOverride)
		} else {
			// Copy the global map to avoid mutating the original.
			merged := make(map[string]AgentOverride, len(result.Agents)+len(local.Agents))
			for k, v := range result.Agents {
				merged[k] = v
			}
			result.Agents = merged
		}
		for k, v := range local.Agents {
			result.Agents[k] = v
		}
	}

	return &result
}

// Merge combines an agent definition with user overrides and defaults.
// Override fields take precedence when non-zero. Defaults are used as fallback
// when neither the agent nor the override specifies a value.
func Merge(a agent.Agent, override AgentOverride, defaults DefaultsConfig) agent.Agent {
	result := a

	if override.Image != "" {
		result.Image = override.Image
	}
	if len(override.Command) > 0 {
		result.Command = override.Command
	}
	if len(override.EnvForward) > 0 {
		result.EnvForward = override.EnvForward
	}

	// CPUs: override > agent default > global default
	if override.CPUs > 0 {
		result.DefaultCPUs = override.CPUs
	}
	if result.DefaultCPUs == 0 && defaults.CPUs > 0 {
		result.DefaultCPUs = defaults.CPUs
	}

	// Memory: override > agent default > global default
	if override.Memory > 0 {
		result.DefaultMemory = override.Memory
	}
	if result.DefaultMemory == 0 && defaults.Memory > 0 {
		result.DefaultMemory = defaults.Memory
	}

	return result
}
