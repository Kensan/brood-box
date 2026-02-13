// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

// Package agent provides the built-in agent registry.
package agent

import (
	"sort"

	domainagent "github.com/stacklok/sandbox-agent/internal/domain/agent"
)

// builtinAgents returns the default set of built-in coding agents.
func builtinAgents() map[string]domainagent.Agent {
	return map[string]domainagent.Agent{
		"claude-code": {
			Name:          "claude-code",
			Image:         "ghcr.io/stacklok/sandbox-agent/claude-code:latest",
			Command:       []string{"claude"},
			EnvForward:    []string{"ANTHROPIC_API_KEY", "CLAUDE_*"},
			DefaultCPUs:   2,
			DefaultMemory: 2048,
		},
		"codex": {
			Name:          "codex",
			Image:         "ghcr.io/stacklok/sandbox-agent/codex:latest",
			Command:       []string{"codex"},
			EnvForward:    []string{"OPENAI_API_KEY", "CODEX_*"},
			DefaultCPUs:   2,
			DefaultMemory: 2048,
		},
		"opencode": {
			Name:          "opencode",
			Image:         "ghcr.io/stacklok/sandbox-agent/opencode:latest",
			Command:       []string{"opencode"},
			EnvForward:    []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "OPENROUTER_API_KEY", "OPENCODE_*"},
			DefaultCPUs:   2,
			DefaultMemory: 2048,
		},
	}
}

// Registry implements agent.Registry with an in-memory map of agents.
type Registry struct {
	agents map[string]domainagent.Agent
}

// NewRegistry creates a new registry pre-loaded with built-in agents.
func NewRegistry() *Registry {
	return &Registry{
		agents: builtinAgents(),
	}
}

// Add registers or overrides an agent in the registry.
func (r *Registry) Add(a domainagent.Agent) {
	r.agents[a.Name] = a
}

// Get returns the agent with the given name, or ErrNotFound.
func (r *Registry) Get(name string) (domainagent.Agent, error) {
	a, ok := r.agents[name]
	if !ok {
		return domainagent.Agent{}, &domainagent.ErrNotFound{Name: name}
	}
	return a, nil
}

// List returns all registered agents sorted by name.
func (r *Registry) List() []domainagent.Agent {
	result := make([]domainagent.Agent, 0, len(r.agents))
	for _, a := range r.agents {
		result = append(result, a)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}
