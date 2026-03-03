// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package process

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsAlive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		pid   int
		alive bool
	}{
		{
			name:  "current process is alive",
			pid:   os.Getpid(),
			alive: true,
		},
		{
			name:  "parent process is alive",
			pid:   os.Getppid(),
			alive: true,
		},
		{
			name:  "impossible PID is not alive",
			pid:   2147483647,
			alive: false,
		},
		{
			name:  "negative PID is not alive",
			pid:   -1,
			alive: false,
		},
		{
			name:  "large negative PID is not alive",
			pid:   -99999,
			alive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.alive, IsAlive(tt.pid))
		})
	}
}
