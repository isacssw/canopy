package cmdline

import (
	"reflect"
	"testing"
)

func TestFields(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    []string
		wantExe string
	}{
		{
			name:    "empty",
			in:      "",
			want:    nil,
			wantExe: "",
		},
		{
			name:    "simple",
			in:      "claude --model sonnet",
			want:    []string{"claude", "--model", "sonnet"},
			wantExe: "claude",
		},
		{
			name:    "double quotes",
			in:      `codex --prompt "fix lint errors"`,
			want:    []string{"codex", "--prompt", "fix lint errors"},
			wantExe: "codex",
		},
		{
			name:    "single quotes",
			in:      "bash -lc 'echo hello world'",
			want:    []string{"bash", "-lc", "echo hello world"},
			wantExe: "bash",
		},
		{
			name:    "escaped space",
			in:      `./my\ tool --flag`,
			want:    []string{"./my tool", "--flag"},
			wantExe: "./my tool",
		},
		{
			name:    "npx scoped",
			in:      "npx @openai/codex --dangerously-bypass-approvals-and-sandbox",
			want:    []string{"npx", "@openai/codex", "--dangerously-bypass-approvals-and-sandbox"},
			wantExe: "npx",
		},
		{
			name:    "quoted executable",
			in:      `"my tool" --arg value`,
			want:    []string{"my tool", "--arg", "value"},
			wantExe: "my tool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Fields(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Fields(%q) = %#v, want %#v", tt.in, got, tt.want)
			}
			if gotExe := Executable(tt.in); gotExe != tt.wantExe {
				t.Fatalf("Executable(%q) = %q, want %q", tt.in, gotExe, tt.wantExe)
			}
		})
	}
}
