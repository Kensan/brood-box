// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"fmt"
	"io"
	"time"

	"github.com/stacklok/brood-box/pkg/domain/progress"
)

// Ensure TimingObserver implements progress.Observer.
var _ progress.Observer = (*TimingObserver)(nil)

// TimingObserver wraps another Observer and records elapsed time per phase.
// Call Summary to print a table of phase durations after the run completes.
type TimingObserver struct {
	inner      progress.Observer
	phaseStart time.Time
	records    []timingRecord
}

type timingRecord struct {
	label   string
	elapsed time.Duration
	failed  bool
}

// NewTimingObserver wraps inner with elapsed-time tracking.
func NewTimingObserver(inner progress.Observer) *TimingObserver {
	return &TimingObserver{inner: inner}
}

// Start records the phase start time and delegates to the inner observer.
func (t *TimingObserver) Start(phase progress.Phase, msg string) {
	t.phaseStart = time.Now()
	t.inner.Start(phase, msg)
}

// Complete records elapsed time for the current phase and delegates.
func (t *TimingObserver) Complete(msg string) {
	t.finish(msg, false)
	t.inner.Complete(msg)
}

// Info delegates to the inner observer without affecting phase timing.
func (t *TimingObserver) Info(msg string) {
	t.inner.Info(msg)
}

// Warn delegates to the inner observer without affecting phase timing.
func (t *TimingObserver) Warn(msg string) {
	t.inner.Warn(msg)
}

// Fail records elapsed time for the current phase and delegates.
func (t *TimingObserver) Fail(msg string) {
	t.finish(msg, true)
	t.inner.Fail(msg)
}

// Summary writes a table of per-phase elapsed times to w.
func (t *TimingObserver) Summary(w io.Writer) {
	if len(t.records) == 0 {
		return
	}
	_, _ = fmt.Fprintln(w, "\nTiming summary:")
	for _, r := range t.records {
		suffix := ""
		if r.failed {
			suffix = " (failed)"
		}
		_, _ = fmt.Fprintf(w, "  %-40s %v%s\n", r.label, r.elapsed.Round(time.Millisecond), suffix)
	}
}

func (t *TimingObserver) finish(label string, failed bool) {
	if t.phaseStart.IsZero() {
		return
	}
	t.records = append(t.records, timingRecord{
		label:   label,
		elapsed: time.Since(t.phaseStart),
		failed:  failed,
	})
	t.phaseStart = time.Time{}
}
