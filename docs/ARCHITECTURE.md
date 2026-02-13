# Architecture

sandbox-agent follows Domain-Driven Design (DDD) with strict layering
and dependency injection throughout.

## Layers

```
cmd/sandbox-agent/main.go   (composition root — wires everything)
        │
        ▼
   app/sandbox.go            (application service — orchestrates domain + infra)
        │
   ┌────┼────────────────┐
   ▼    ▼                ▼
domain/agent/        domain/config/       (pure domain — no imports from infra)
   │                     │
   │    ┌────────────────┤
   ▼    ▼                ▼
infra/vm/          infra/ssh/         infra/config/     infra/agent/
(propolis)         (PTY terminal)     (YAML loader)     (built-in registry)
```

### Domain Layer (`internal/domain/`)

Pure business logic with zero infrastructure dependencies. This layer
defines the core types and interfaces.

- **`agent/agent.go`** -- `Agent` value object (name, image, command,
  env patterns, resource defaults) and `Registry` interface.
- **`agent/env.go`** -- `ForwardEnv()` collects host env vars matching
  patterns. `ShellEscape()` quotes values for safe shell injection.
  Both are pure functions with injectable `EnvProvider` for testing.
- **`config/config.go`** -- `Config`, `DefaultsConfig`, `AgentOverride`
  structs (pure data, YAML tags). `Merge()` combines agent + override +
  defaults with clear precedence rules.

**Rule**: `domain/` NEVER imports from `infra/` or `app/`.

### Application Layer (`internal/app/`)

The `SandboxRunner` orchestrates the full lifecycle:

1. Resolve agent from registry
2. Load config and merge overrides
3. Collect forwarded env vars
4. Start VM via `VMRunner`
5. Run interactive terminal session
6. Stop VM on exit

All dependencies are injected via the `SandboxDeps` struct. The
orchestrator has no direct dependency on propolis, SSH libraries, or
the filesystem — only on interfaces.

### Infrastructure Layer (`internal/infra/`)

Concrete implementations of domain interfaces and system integration.

- **`vm/runner.go`** -- `PropolisRunner` implements `VMRunner` using
  `propolis.Run()` with options for ports, virtio-fs, rootfs hooks,
  init override, and post-boot SSH readiness check.
- **`vm/hooks.go`** -- Three `RootFSHook` factories: `InjectSSHKeys`,
  `InjectInitScript`, `InjectEnvFile`. These modify the extracted rootfs
  before the VM boots.
- **`ssh/terminal.go`** -- `InteractiveSession` implements PTY-forwarded
  SSH sessions with raw terminal mode, SIGWINCH handling, and context
  cancellation support.
- **`config/loader.go`** -- `Loader` reads YAML config from
  `$XDG_CONFIG_HOME/sandbox-agent/config.yaml` with graceful fallback
  when the file doesn't exist.
- **`agent/registry.go`** -- In-memory `Registry` pre-loaded with
  built-in agents (claude-code, codex, opencode). Supports adding
  custom agents from config.

### Composition Root (`cmd/sandbox-agent/main.go`)

Wires all concrete implementations together:

- Creates the agent registry and registers custom agents from config
- Creates `PropolisRunner`, `InteractiveSession`, `Loader`, `OSEnvProvider`
- Injects everything into `SandboxRunner`
- Cobra CLI with positional agent name arg and flags
- Signal handling (SIGINT/SIGTERM) via `signal.NotifyContext`

## Dependency Injection

Every struct accepts interfaces via constructor injection. No global state.

```go
// Domain defines interfaces
type Registry interface {
    Get(name string) (Agent, error)
    List() []Agent
}

type EnvProvider interface {
    Environ() []string
}

// App accepts all deps via struct
type SandboxDeps struct {
    Registry    agent.Registry
    VMRunner    vm.VMRunner
    Terminal    infrassh.TerminalSession
    CfgLoader   *infraconfig.Loader
    EnvProvider agent.EnvProvider
    Logger      *slog.Logger
}

// Infra provides implementations
// vm.PropolisRunner implements vm.VMRunner
// ssh.InteractiveSession implements ssh.TerminalSession
// agent.Registry implements agent.Registry
```

This makes the app layer fully testable with mocks — see
`internal/app/sandbox_test.go` for examples.

## VM Lifecycle

```
sandbox-agent claude-code
        │
        ▼
   Pull OCI image (propolis handles caching)
        │
        ▼
   Extract rootfs from layers
        │
        ▼
   Run rootfs hooks:
     1. InjectSSHKeys  → /root/.ssh/authorized_keys
     2. InjectInitScript → /sandbox-init.sh
     3. InjectEnvFile  → /etc/sandbox-env
        │
        ▼
   Write .krun_config.json (init override → /sandbox-init.sh)
        │
        ▼
   Start networking (in-process, gvisor-tap-vsock)
        │
        ▼
   Spawn propolis-runner (libkrun microVM)
        │
        ▼
   Guest boots:
     /sandbox-init.sh:
       - ip link set lo up
       - udhcpc (DHCP)
       - sshd -D (background)
       - mount virtiofs workspace → /workspace
       - wait
        │
        ▼
   Post-boot hook: WaitForReady (SSH poll)
        │
        ▼
   SSH session:
     source /etc/sandbox-env
     cd /workspace
     exec claude   (or codex, opencode, etc.)
        │
        ▼
   Agent exits → SSH session ends → VM stopped → cleanup
```

## Guest Environment

Inside the VM:

| Path | Contents |
|------|----------|
| `/workspace` | Host workspace directory (virtio-fs mount) |
| `/etc/sandbox-env` | `export KEY='value'` lines for forwarded vars |
| `/root/.ssh/authorized_keys` | Ephemeral public key for SSH access |
| `/sandbox-init.sh` | Init script (networking, sshd, mounts) |
| `/.krun_config.json` | libkrun config pointing to `/sandbox-init.sh` |

## Security Model

- **Hardware isolation**: VMs run under KVM (Linux) via libkrun. This
  provides stronger isolation than containers.
- **Ephemeral SSH keys**: Generated per session (ECDSA P-256), deleted
  on exit. Never written to persistent storage.
- **Localhost-only SSH**: Port forwards bind to `127.0.0.1` only.
- **Shell-escaped env injection**: All environment variable values are
  single-quote escaped to prevent injection.
- **No persistent state**: sandbox-agent doesn't maintain any state
  between runs. Each invocation is fully ephemeral.

## Relationship to Propolis

sandbox-agent is a consumer of the [propolis](https://github.com/stacklok/propolis)
library. It uses propolis via a local `replace` directive in `go.mod`:

```
replace github.com/stacklok/propolis => ../propolis
```

### Propolis APIs Used

| API | Usage |
|-----|-------|
| `propolis.Run()` | Orchestrate the full OCI-to-VM pipeline |
| `WithName` | Name the VM `sandbox-<agent>` |
| `WithCPUs` / `WithMemory` | Set VM resources |
| `WithPorts` | Forward SSH (host → guest:22) |
| `WithVirtioFS` | Mount workspace directory |
| `WithRootFSHook` | Inject SSH keys, init script, env file |
| `WithInitOverride` | Replace OCI CMD with `/sandbox-init.sh` |
| `WithPostBoot` | Wait for SSH readiness |
| `WithRunnerPath` | Locate propolis-runner binary |
| `ssh.GenerateKeyPair` | Create ephemeral SSH keys |
| `ssh.GetPublicKeyContent` | Read public key for injection |
| `ssh.NewClient` | Create SSH client for readiness check |
| `VM.Stop` | Graceful VM shutdown |
| `VM.Ports` | Discover actual SSH port |
| `VM.DataDir` | Get VM data directory |
