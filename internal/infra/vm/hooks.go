// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package vm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/stacklok/propolis/image"

	domainagent "github.com/stacklok/sandbox-agent/internal/domain/agent"
)

// initScript is the guest init script that starts networking, SSH, and
// mounts the workspace via virtio-fs.
const initScript = `#!/bin/sh
set -e
ip link set lo up
udhcpc -i eth0 -s /etc/udhcpc/default.script 2>/dev/null || true
mkdir -p /root/.ssh && chmod 700 /root/.ssh
mkdir -p /run/sshd
/usr/sbin/sshd -D &
mkdir -p /workspace
mount -t virtiofs workspace /workspace 2>/dev/null || true
wait
`

// InjectSSHKeys returns a RootFS hook that writes the given public key
// into /root/.ssh/authorized_keys in the guest rootfs.
func InjectSSHKeys(pubKey string) func(string, *image.OCIConfig) error {
	return func(rootfsPath string, _ *image.OCIConfig) error {
		sshDir := filepath.Join(rootfsPath, "root", ".ssh")
		if err := os.MkdirAll(sshDir, 0o700); err != nil {
			return fmt.Errorf("creating .ssh dir: %w", err)
		}

		authKeysPath := filepath.Join(sshDir, "authorized_keys")
		if err := os.WriteFile(authKeysPath, []byte(pubKey+"\n"), 0o600); err != nil {
			return fmt.Errorf("writing authorized_keys: %w", err)
		}

		return nil
	}
}

// InjectInitScript returns a RootFS hook that writes the sandbox init
// script to /sandbox-init.sh in the guest rootfs.
func InjectInitScript() func(string, *image.OCIConfig) error {
	return func(rootfsPath string, _ *image.OCIConfig) error {
		initPath := filepath.Join(rootfsPath, "sandbox-init.sh")
		if err := os.WriteFile(initPath, []byte(initScript), 0o755); err != nil {
			return fmt.Errorf("writing init script: %w", err)
		}
		return nil
	}
}

// InjectEnvFile returns a RootFS hook that writes forwarded environment
// variables as an /etc/sandbox-env file that can be sourced by the SSH session.
func InjectEnvFile(envVars map[string]string) func(string, *image.OCIConfig) error {
	return func(rootfsPath string, _ *image.OCIConfig) error {
		if len(envVars) == 0 {
			return nil
		}

		etcDir := filepath.Join(rootfsPath, "etc")
		if err := os.MkdirAll(etcDir, 0o755); err != nil {
			return fmt.Errorf("creating /etc dir: %w", err)
		}

		var sb strings.Builder
		for k, v := range envVars {
			fmt.Fprintf(&sb, "export %s=%s\n", k, domainagent.ShellEscape(v))
		}

		envPath := filepath.Join(etcDir, "sandbox-env")
		if err := os.WriteFile(envPath, []byte(sb.String()), 0o644); err != nil {
			return fmt.Errorf("writing env file: %w", err)
		}

		return nil
	}
}
