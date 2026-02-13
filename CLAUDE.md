# sandbox-agent

CLI tool for running coding agents (Claude Code, Codex, OpenCode) inside hardware-isolated microVMs.
Wraps the propolis framework with an opinionated CLI.

Module: `github.com/stacklok/sandbox-agent`

## Commands

```bash
task build         # Build sandbox-agent (pure Go, no CGO)
task build-dev     # Build sandbox-agent + propolis-runner (requires libkrun-devel)
task test          # go test -v -race ./...
task lint          # golangci-lint run ./...
task lint-fix      # Auto-fix lint issues
task fmt           # go fmt + goimports
task tidy          # go mod tidy
task verify        # fmt + lint + test
task run           # Build and run
task clean         # Remove bin/ and coverage files
```

Run a single test: `go test -v -race -run TestName ./path/to/package`

## Architecture

DDD layered architecture with dependency injection:

- `cmd/sandbox-agent/main.go` — Composition root, wires dependencies, Cobra CLI
- `internal/domain/agent/` — Agent value object, env forwarding (pure domain, no I/O)
- `internal/domain/config/` — Config types, merge logic (pure data)
- `internal/app/` — SandboxRunner orchestrator (application service)
- `internal/infra/vm/` — Propolis VMRunner implementation, rootfs hooks
- `internal/infra/ssh/` — Interactive PTY terminal session
- `internal/infra/config/` — YAML config loader
- `internal/infra/agent/` — Built-in agent registry

**Rule**: `domain/` NEVER imports from `infra/` or `app/`. Interfaces live in domain, implementations in infra.

## Conventions

- **SPDX headers required** on every `.go` and `.yaml` file:
  ```
  // SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
  // SPDX-License-Identifier: Apache-2.0
  ```
- Use `log/slog` exclusively — no `fmt.Println` or `log.Printf` in library code.
- Wrap errors with `fmt.Errorf("context: %w", err)` forming readable chains.
- Prefer table-driven tests. Test files go alongside the code they test.
- Imperative mood commit messages, capitalize, no trailing period, limit subject to 50 chars.
- IMPORTANT: Never use `git add -A`. Stage specific files only.

## Things That Will Bite You

- **propolis is a local replace**: `go.mod` uses `replace github.com/stacklok/propolis => ../propolis`. The propolis checkout must be at `../propolis`.
- **CGO boundary**: sandbox-agent itself is pure Go (`CGO_ENABLED=0`). Only propolis-runner needs CGO.
- **Domain purity**: `internal/domain/` must never import from `internal/infra/` or `internal/app/`.

## Verification

After any code change:
```bash
task fmt && task lint    # Format and lint
task test                # Full test suite with race detector
```
