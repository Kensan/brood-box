// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

// Package vm provides the propolis-backed VM runner implementation.
package vm

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/stacklok/propolis"
	propolisssh "github.com/stacklok/propolis/ssh"
)

// VMConfig holds the parameters needed to start a sandbox VM.
type VMConfig struct {
	// Name is a unique name for this VM instance.
	Name string

	// Image is the OCI image reference to pull and boot.
	Image string

	// CPUs is the number of vCPUs.
	CPUs uint32

	// Memory is the RAM in MiB.
	Memory uint32

	// SSHPort is the host port to forward to guest port 22.
	// If 0, an ephemeral port will be chosen.
	SSHPort uint16

	// WorkspacePath is the host directory to mount as /workspace in the VM.
	WorkspacePath string

	// EnvVars are environment variables to inject into the VM.
	EnvVars map[string]string
}

// VMRunner creates and manages sandbox VMs.
type VMRunner interface {
	// Start boots a VM with the given configuration. The returned VM must
	// be stopped when no longer needed.
	Start(ctx context.Context, cfg VMConfig) (VM, error)
}

// VM represents a running sandbox VM.
type VM interface {
	// Stop gracefully shuts down the VM.
	Stop(ctx context.Context) error

	// SSHPort returns the host port mapped to guest SSH.
	SSHPort() uint16

	// DataDir returns the VM's data directory.
	DataDir() string

	// SSHKeyPath returns the path to the ephemeral SSH private key.
	SSHKeyPath() string
}

// PropolisRunner implements VMRunner using the propolis library.
type PropolisRunner struct {
	runnerPath string
	logger     *slog.Logger
}

// NewPropolisRunner creates a VMRunner backed by propolis.
// runnerPath is the path to the propolis-runner binary.
func NewPropolisRunner(runnerPath string, logger *slog.Logger) *PropolisRunner {
	return &PropolisRunner{
		runnerPath: runnerPath,
		logger:     logger,
	}
}

// Start boots a microVM using propolis.
func (r *PropolisRunner) Start(ctx context.Context, cfg VMConfig) (VM, error) {
	r.logger.Info("starting sandbox VM",
		"name", cfg.Name,
		"image", cfg.Image,
		"cpus", cfg.CPUs,
		"memory", cfg.Memory,
	)

	// Each VM gets its own data directory so multiple VMs can run in parallel
	// without conflicting on state files or logs.
	dataDir, err := vmDataDir(cfg.Name)
	if err != nil {
		return nil, fmt.Errorf("resolving VM data directory: %w", err)
	}

	// Generate ephemeral SSH key pair in a temp dir.
	keyDir, err := os.MkdirTemp("", "sandbox-ssh-*")
	if err != nil {
		return nil, fmt.Errorf("creating ssh key dir: %w", err)
	}

	privKeyPath, pubKeyPath, err := propolisssh.GenerateKeyPair(keyDir)
	if err != nil {
		return nil, fmt.Errorf("generating ssh key pair: %w", err)
	}

	pubKey, err := propolisssh.GetPublicKeyContent(pubKeyPath)
	if err != nil {
		return nil, fmt.Errorf("reading public key: %w", err)
	}

	// Determine SSH port.
	sshPort := cfg.SSHPort
	if sshPort == 0 {
		sshPort = 0 // propolis will pick an ephemeral port
	}

	// Build propolis options.
	opts := []propolis.Option{
		propolis.WithName(cfg.Name),
		propolis.WithDataDir(dataDir),
		propolis.WithCPUs(cfg.CPUs),
		propolis.WithMemory(cfg.Memory),
		propolis.WithPorts(propolis.PortForward{Host: sshPort, Guest: 22}),
		propolis.WithRootFSHook(
			InjectSSHKeys(pubKey),
			InjectInitScript(),
			InjectEnvFile(cfg.EnvVars),
		),
		propolis.WithInitOverride("/sandbox-init.sh"),
		propolis.WithPostBoot(func(ctx context.Context, vm *propolis.VM) error {
			// Find the actual SSH port from the VM's port forwards.
			ports := vm.Ports()
			var actualPort uint16
			for _, p := range ports {
				if p.Guest == 22 {
					actualPort = p.Host
					break
				}
			}
			if actualPort == 0 {
				return fmt.Errorf("SSH port forward not found")
			}

			r.logger.Info("waiting for SSH", "port", actualPort)
			client := propolisssh.NewClient("127.0.0.1", actualPort, "root", privKeyPath)
			return client.WaitForReady(ctx)
		}),
	}

	// Add runner path if specified.
	if r.runnerPath != "" {
		opts = append(opts, propolis.WithRunnerPath(r.runnerPath))
	}

	// Add workspace mount if specified.
	if cfg.WorkspacePath != "" {
		absPath, err := filepath.Abs(cfg.WorkspacePath)
		if err != nil {
			return nil, fmt.Errorf("resolving workspace path: %w", err)
		}
		opts = append(opts, propolis.WithVirtioFS(propolis.VirtioFSMount{
			Tag:      "workspace",
			HostPath: absPath,
		}))
	}

	// Run propolis.
	pvm, err := propolis.Run(ctx, cfg.Image, opts...)
	if err != nil {
		// Clean up SSH keys on failure.
		_ = os.RemoveAll(keyDir)
		return nil, fmt.Errorf("starting VM: %w", err)
	}

	return &propolisVM{
		vm:         pvm,
		sshKeyPath: privKeyPath,
		sshKeyDir:  keyDir,
		logger:     r.logger,
	}, nil
}

// propolisVM wraps a propolis.VM to implement our VM interface.
type propolisVM struct {
	vm         *propolis.VM
	sshKeyPath string
	sshKeyDir  string
	logger     *slog.Logger
}

func (v *propolisVM) Stop(ctx context.Context) error {
	v.logger.Info("stopping sandbox VM")
	err := v.vm.Stop(ctx)
	// Clean up ephemeral SSH keys regardless of stop outcome.
	_ = os.RemoveAll(v.sshKeyDir)
	if err != nil {
		return fmt.Errorf("stopping VM: %w", err)
	}
	return nil
}

func (v *propolisVM) SSHPort() uint16 {
	ports := v.vm.Ports()
	for _, p := range ports {
		if p.Guest == 22 {
			return p.Host
		}
	}
	return 0
}

func (v *propolisVM) DataDir() string {
	return v.vm.DataDir()
}

func (v *propolisVM) SSHKeyPath() string {
	return v.sshKeyPath
}

// vmDataDir returns a per-VM data directory under ~/.config/sandbox-agent/<name>.
// This isolates state files, logs, and locks so multiple VMs can run in parallel.
func vmDataDir(name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determining home directory: %w", err)
	}
	return filepath.Join(home, ".config", "sandbox-agent", "vms", name), nil
}
