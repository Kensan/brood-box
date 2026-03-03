// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stacklok/brood-box/pkg/domain/progress"
)

func TestTimingObserver_DelegatesAllMethods(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	inner := NewSimpleObserver(&buf)
	obs := NewTimingObserver(inner)

	obs.Start(progress.PhaseStartingVM, "Starting VM...")
	obs.Complete("Sandbox ready")
	obs.Start(progress.PhaseResolvingAgent, "Resolving agent...")
	obs.Info("some info")
	obs.Warn("some warning")
	obs.Fail("something failed")

	out := buf.String()
	assert.Contains(t, out, "Starting VM...")
	assert.Contains(t, out, "Sandbox ready")
	assert.Contains(t, out, "Resolving agent...")
	assert.Contains(t, out, "some info")
	assert.Contains(t, out, "some warning")
	assert.Contains(t, out, "something failed")
}

func TestTimingObserver_RecordsCompletedPhases(t *testing.T) {
	t.Parallel()

	obs := NewTimingObserver(NewSimpleObserver(&bytes.Buffer{}))

	obs.Start(progress.PhaseCreatingSnapshot, "Creating snapshot...")
	obs.Complete("Snapshot created")

	obs.Start(progress.PhaseStartingVM, "Starting VM...")
	obs.Complete("Sandbox ready")

	var summary bytes.Buffer
	obs.Summary(&summary)

	out := summary.String()
	require.Contains(t, out, "Timing summary:")
	assert.Contains(t, out, "Snapshot created")
	assert.Contains(t, out, "Sandbox ready")
}

func TestTimingObserver_RecordsFailedPhase(t *testing.T) {
	t.Parallel()

	obs := NewTimingObserver(NewSimpleObserver(&bytes.Buffer{}))

	obs.Start(progress.PhaseStartingVM, "Starting VM...")
	obs.Fail("VM boot failed")

	var summary bytes.Buffer
	obs.Summary(&summary)

	out := summary.String()
	assert.Contains(t, out, "VM boot failed")
	assert.Contains(t, out, "(failed)")
}

func TestTimingObserver_InfoAndWarnDoNotCreateRecords(t *testing.T) {
	t.Parallel()

	obs := NewTimingObserver(NewSimpleObserver(&bytes.Buffer{}))

	obs.Start(progress.PhaseResolvingAgent, "Resolving...")
	obs.Info("mid-phase info")
	obs.Warn("mid-phase warning")
	obs.Complete("Resolved")

	var summary bytes.Buffer
	obs.Summary(&summary)

	out := summary.String()
	// Only one record: the Complete, not the Info/Warn
	assert.Equal(t, 1, strings.Count(out, "Resolved"))
	assert.NotContains(t, out, "mid-phase info")
	assert.NotContains(t, out, "mid-phase warning")
}

func TestTimingObserver_SummaryEmptyWithNoPhases(t *testing.T) {
	t.Parallel()

	obs := NewTimingObserver(NewSimpleObserver(&bytes.Buffer{}))

	var summary bytes.Buffer
	obs.Summary(&summary)

	assert.Empty(t, summary.String())
}

func TestTimingObserver_CompleteWithoutStartIsNoop(t *testing.T) {
	t.Parallel()

	obs := NewTimingObserver(NewSimpleObserver(&bytes.Buffer{}))

	// Complete without a preceding Start should not panic or record anything.
	obs.Complete("orphaned complete")

	var summary bytes.Buffer
	obs.Summary(&summary)

	assert.Empty(t, summary.String())
}

func TestTimingObserver_PreservesPhaseOrder(t *testing.T) {
	t.Parallel()

	obs := NewTimingObserver(NewSimpleObserver(&bytes.Buffer{}))

	obs.Start(progress.PhaseResolvingAgent, "Resolving agent...")
	obs.Complete("Resolved")

	obs.Start(progress.PhaseCreatingSnapshot, "Creating snapshot...")
	obs.Complete("Snapshot done")

	obs.Start(progress.PhaseStartingVM, "Starting VM...")
	obs.Complete("Sandbox ready")

	var summary bytes.Buffer
	obs.Summary(&summary)

	out := summary.String()
	resolvedPos := strings.Index(out, "Resolved")
	snapshotPos := strings.Index(out, "Snapshot done")
	sandboxPos := strings.Index(out, "Sandbox ready")

	assert.Less(t, resolvedPos, snapshotPos, "phases should appear in start order")
	assert.Less(t, snapshotPos, sandboxPos, "phases should appear in start order")
}
