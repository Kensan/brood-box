// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

// Package logging provides slog handlers for file logging and fan-out.
package logging

import (
	"context"
	"io"
	"log/slog"
)

// NewFileHandler creates a JSON slog.Handler that writes to w.
func NewFileHandler(w io.Writer, level slog.Leveler) slog.Handler {
	return slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
}

// FanoutHandler writes every log record to all underlying handlers.
type FanoutHandler struct {
	handlers []slog.Handler
}

// NewFanoutHandler creates a handler that fans out to all provided handlers.
func NewFanoutHandler(handlers ...slog.Handler) *FanoutHandler {
	return &FanoutHandler{handlers: handlers}
}

// Enabled returns true if any underlying handler is enabled for the level.
func (f *FanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range f.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle writes the record to every underlying handler.
func (f *FanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range f.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r); err != nil {
				return err
			}
		}
	}
	return nil
}

// WithAttrs returns a new FanoutHandler with the attrs added to each underlying handler.
func (f *FanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		handlers[i] = h.WithAttrs(attrs)
	}
	return &FanoutHandler{handlers: handlers}
}

// WithGroup returns a new FanoutHandler with the group applied to each underlying handler.
func (f *FanoutHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		handlers[i] = h.WithGroup(name)
	}
	return &FanoutHandler{handlers: handlers}
}
