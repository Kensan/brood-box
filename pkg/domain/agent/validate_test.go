// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"strings"
	"testing"
)

func TestValidateName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// Valid names — built-in agents.
		{name: "claude-code", input: "claude-code"},
		{name: "codex", input: "codex"},
		{name: "opencode", input: "opencode"},

		// Valid names — custom agents.
		{name: "underscore", input: "my_agent"},
		{name: "mixed case", input: "MyAgent"},
		{name: "digits only after start", input: "a123"},
		{name: "all digits", input: "123"},
		{name: "single char", input: "a"},
		{name: "single digit", input: "1"},
		{name: "hyphens and underscores", input: "my-custom_agent-2"},
		{name: "max length", input: strings.Repeat("a", MaxNameLength)},

		// Invalid — empty.
		{name: "empty", input: "", wantErr: true},

		// Invalid — path traversal.
		{name: "dot-dot-slash", input: "../etc", wantErr: true},
		{name: "embedded traversal", input: "foo/../bar", wantErr: true},

		// Invalid — starts with non-alphanumeric.
		{name: "leading dot", input: ".hidden", wantErr: true},
		{name: "leading hyphen", input: "-start", wantErr: true},
		{name: "leading underscore", input: "_start", wantErr: true},

		// Invalid — special characters.
		{name: "space", input: "a b", wantErr: true},
		{name: "exclamation", input: "agent!", wantErr: true},
		{name: "asterisk", input: "*", wantErr: true},
		{name: "slash", input: "a/b", wantErr: true},
		{name: "backslash", input: "a\\b", wantErr: true},
		{name: "colon", input: "a:b", wantErr: true},
		{name: "at sign", input: "agent@1", wantErr: true},
		{name: "semicolon", input: "agent;rm", wantErr: true},
		{name: "pipe", input: "agent|cmd", wantErr: true},
		{name: "dollar", input: "agent$HOME", wantErr: true},
		{name: "backtick", input: "agent`id`", wantErr: true},

		// Invalid — control characters and encoding tricks.
		{name: "null byte", input: "agent\x00name", wantErr: true},
		{name: "newline", input: "agent\nname", wantErr: true},
		{name: "tab", input: "agent\tname", wantErr: true},
		{name: "carriage return", input: "agent\rname", wantErr: true},
		{name: "unicode slash homoglyph", input: "agent\u2215name", wantErr: true},
		{name: "non-ascii letter", input: "caf\u00e9", wantErr: true},

		// Valid — boundary cases with trailing special chars.
		{name: "trailing hyphen", input: "agent-"},
		{name: "trailing underscore", input: "agent_"},
		{name: "consecutive hyphens", input: "a---"},
		{name: "double hyphen", input: "my--agent"},

		// Invalid — too long.
		{name: "over max length", input: strings.Repeat("a", MaxNameLength+1), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateName(tt.input)
			if tt.wantErr && err == nil {
				t.Errorf("ValidateName(%q) = nil, want error", tt.input)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateName(%q) = %v, want nil", tt.input, err)
			}
		})
	}
}
