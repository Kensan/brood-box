// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFileHandler_WritesJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	h := NewFileHandler(&buf, slog.LevelInfo)
	logger := slog.New(h)

	logger.Info("hello", "key", "value")

	output := buf.String()
	assert.Contains(t, output, `"msg":"hello"`)
	assert.Contains(t, output, `"key":"value"`)
}

func TestFanoutHandler_WritesToAllHandlers(t *testing.T) {
	t.Parallel()

	var buf1, buf2 bytes.Buffer
	h1 := slog.NewTextHandler(&buf1, &slog.HandlerOptions{Level: slog.LevelInfo})
	h2 := slog.NewJSONHandler(&buf2, &slog.HandlerOptions{Level: slog.LevelInfo})

	fanout := NewFanoutHandler(h1, h2)
	logger := slog.New(fanout)

	logger.Info("test message", "k", "v")

	assert.Contains(t, buf1.String(), "test message")
	assert.Contains(t, buf2.String(), `"msg":"test message"`)
}

func TestFanoutHandler_RespectsLevel(t *testing.T) {
	t.Parallel()

	var buf1, buf2 bytes.Buffer
	h1 := slog.NewTextHandler(&buf1, &slog.HandlerOptions{Level: slog.LevelInfo})
	h2 := slog.NewTextHandler(&buf2, &slog.HandlerOptions{Level: slog.LevelWarn})

	fanout := NewFanoutHandler(h1, h2)
	logger := slog.New(fanout)

	logger.Info("info only")

	assert.Contains(t, buf1.String(), "info only")
	assert.Empty(t, buf2.String())
}

func TestFanoutHandler_Enabled(t *testing.T) {
	t.Parallel()

	h1 := slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn})
	h2 := slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError})

	fanout := NewFanoutHandler(h1, h2)

	// Warn is enabled (h1 accepts Warn).
	assert.True(t, fanout.Enabled(context.Background(), slog.LevelWarn))
	// Info is not enabled (neither handler accepts Info).
	assert.False(t, fanout.Enabled(context.Background(), slog.LevelInfo))
}

func TestFanoutHandler_WithAttrs(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	fanout := NewFanoutHandler(h)

	withAttrs := fanout.WithAttrs([]slog.Attr{slog.String("component", "test")})
	logger := slog.New(withAttrs)

	logger.Info("msg")

	assert.Contains(t, buf.String(), "component=test")
}

func TestFanoutHandler_WithGroup(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	fanout := NewFanoutHandler(h)

	withGroup := fanout.WithGroup("grp")
	logger := slog.New(withGroup)

	logger.Info("msg", "key", "val")

	assert.Contains(t, buf.String(), "grp.key=val")
}

func TestFanoutHandler_NoHandlers(t *testing.T) {
	t.Parallel()

	fanout := NewFanoutHandler()
	logger := slog.New(fanout)

	// Should not panic with zero handlers.
	require.NotPanics(t, func() {
		logger.Info("no handlers")
	})
}

func TestFanoutHandler_DebugFiltered(t *testing.T) {
	t.Parallel()

	var debugBuf, infoBuf bytes.Buffer
	debugH := slog.NewTextHandler(&debugBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	infoH := slog.NewTextHandler(&infoBuf, &slog.HandlerOptions{Level: slog.LevelInfo})

	fanout := NewFanoutHandler(debugH, infoH)
	logger := slog.New(fanout)

	logger.Debug("debug msg")
	logger.Info("info msg")

	// Debug handler gets both.
	assert.True(t, strings.Contains(debugBuf.String(), "debug msg"))
	assert.True(t, strings.Contains(debugBuf.String(), "info msg"))

	// Info handler only gets info.
	assert.False(t, strings.Contains(infoBuf.String(), "debug msg"))
	assert.True(t, strings.Contains(infoBuf.String(), "info msg"))
}
