# sandbox-agent

CLI tool for running coding agents (Claude Code, Codex, OpenCode) inside hardware-isolated microVMs.
Wraps the propolis framework with an opinionated CLI.

Module: `github.com/stacklok/sandbox-agent`

## Commands — ALWAYS use `task` (Taskfile.yaml)

**IMPORTANT**: ALWAYS use `task <target>` for building, testing, linting, formatting, and running. NEVER invoke `go build`, `go test`, `golangci-lint`, `go fmt`, `goimports`, or `podman build` directly — the Taskfile wraps these with the correct flags, ldflags, env vars, and dependency ordering. Running raw commands will produce incorrect builds or miss steps.

```bash
task build             # Build sandbox-agent (pure Go, no CGO)
task build-init        # Cross-compile sandbox-init for guest VM
task build-dev         # Build sandbox-agent + propolis-runner (requires libkrun-devel)
task test              # go test -v -race ./...
task test-coverage     # Run tests with coverage report
task lint              # golangci-lint run ./...
task lint-fix          # Auto-fix lint issues
task fmt               # go fmt + goimports
task tidy              # go mod tidy
task verify            # fmt + lint + test
task run               # Build and run
task clean             # Remove bin/ and coverage files
task image-base        # Build base guest image
task image-claude-code # Build claude-code guest image
task image-codex       # Build codex guest image
task image-opencode    # Build opencode guest image
task image-all         # Build all guest images
task image-push        # Push all images to GHCR
```

The only exception is running a single test, where raw `go test` is acceptable:
`go test -v -race -run TestName ./path/to/package`

## Architecture — Strict DDD (Domain-Driven Design)

This project follows DDD layered architecture with dependency injection **strictly and without exception**. Every new type, interface, and function MUST be placed in the correct layer. Violating layer boundaries is a blocking issue — do not merge code that breaks these rules.

### Layers

**Domain** (`internal/domain/`) — Pure types and interfaces. ZERO I/O, ZERO external dependencies, ZERO side effects. Domain packages define _what_ things are and _what_ operations exist, never _how_ they are performed:
- `internal/domain/agent/` — Agent value object, env forwarding
- `internal/domain/config/` — Config types, merge logic
- `internal/domain/vm/` — VMRunner, VM, VMConfig interfaces
- `internal/domain/session/` — TerminalSession interface
- `internal/domain/workspace/` — WorkspaceCloner interface, Snapshot type
- `internal/domain/snapshot/` — FileChange, ExcludeConfig, Matcher, Differ, Reviewer, Flusher

**Application** (`internal/app/`) — Orchestration only. Depends on domain interfaces, never on infrastructure. Contains no I/O implementations:
- `internal/app/` — SandboxRunner orchestrator (application service)

**Infrastructure** (`internal/infra/`) — Concrete implementations of domain interfaces. This is the only layer that touches I/O, external libraries, and system calls:
- `internal/infra/vm/` — Propolis VMRunner implementation, rootfs hooks
- `internal/infra/ssh/` — Interactive PTY terminal session
- `internal/infra/config/` — YAML config loader
- `internal/infra/agent/` — Built-in agent registry
- `internal/infra/exclude/` — Gitignore-compatible exclude pattern loading + two-tier matching
- `internal/infra/workspace/` — COW workspace cloning (FICLONE on Linux, clonefile on macOS, copy fallback)
- `internal/infra/diff/` — SHA-256 based file diff engine
- `internal/infra/review/` — Interactive per-file terminal review + flusher with hash verification

**Guest VM** (`internal/guest/`, Linux only — runs inside the microVM):
- `internal/guest/` — Boot, mount, network, env, sshd, reaper packages
- `cmd/sandbox-init/` — Guest PID 1 init binary (compiled Go)

**CLI + Composition Root** (`cmd/`):
- `cmd/sandbox-agent/main.go` — Composition root, wires dependencies, Cobra CLI
- `internal/version/` — Version/commit info via ldflags

### DDD Rules (non-negotiable)

- **`domain/` NEVER imports from `infra/` or `app/`.** Interfaces live in domain, implementations in infra. No exceptions.
- **`app/` NEVER imports from `infra/`.** The application layer depends only on domain interfaces; concrete implementations are injected by the composition root (`cmd/`).
- **New interfaces go in `domain/`**, new implementations go in `infra/`**. If you need a new capability, define the interface in the appropriate domain package first, then implement it in infra.
- **No business logic in `infra/`**. Infrastructure adapts external systems to domain interfaces — it does not make business decisions.
- **No I/O in `domain/`**. Domain types must be testable without mocks, fakes, or network access.

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

## Workspace Snapshot Isolation

By default, the workspace is mounted as a COW snapshot. After the agent finishes, you review changes per-file before they touch the real workspace.

- `--no-review` — Disable snapshot isolation, mount workspace directly
- `--exclude "pattern"` — Additional gitignore-style exclude patterns (repeatable)
- `.sandboxignore` — Per-workspace exclude file (gitignore syntax) in workspace root
- `.sandbox-agent.yaml` — Per-workspace config file (merged into global config; `review.enabled` is **ignored** for security)
- Security patterns (`.env*`, `*.pem`, `.ssh/`, `.sandbox-agent.yaml`, etc.) are **non-overridable** — cannot be negated
- Performance patterns (`node_modules/`, `vendor/`, etc.) can be negated in `.sandboxignore`

Global config (`~/.config/sandbox-agent/config.yaml`):
```yaml
review:
  enabled: true
  exclude_patterns:
    - "*.log"
    - "tmp/"
```

Execution order: create snapshot → start VM → terminal → stop VM → diff → review → flush → cleanup.

## Things That Will Bite You

- **propolis is a local replace**: `go.mod` uses `replace github.com/stacklok/propolis => ../propolis`. The propolis checkout must be at `../propolis`.
- **CGO boundary**: sandbox-agent itself is pure Go (`CGO_ENABLED=0`). Only propolis-runner needs CGO.
- **Domain purity**: `internal/domain/` must never import from `internal/infra/` or `internal/app/`. This is the most important architectural invariant — break it and you break the entire DDD foundation.
- **Always use `task`**: Never run `go build`, `go test ./...`, `golangci-lint`, `go fmt`, or `goimports` directly. The Taskfile sets critical env vars and flags. Raw commands will silently produce wrong results.

## Verification

After any code change:
```bash
task fmt && task lint    # Format and lint
task test                # Full test suite with race detector
```
