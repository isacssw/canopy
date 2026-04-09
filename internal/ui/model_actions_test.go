package ui

import (
	"errors"
	"reflect"
	"testing"
)

func TestBuildNvimRemoteOpenCommand(t *testing.T) {
	wtPath := "/tmp/my wt"
	filePath := `/tmp/my wt/a "b".go`

	got := buildNvimRemoteOpenCommand(wtPath, filePath, 42)
	want := `<C-\><C-N>:close | execute 'cd ' . fnameescape("/tmp/my wt") | execute 'edit +42 ' . fnameescape("/tmp/my wt/a \"b\".go")<CR>`

	if got != want {
		t.Fatalf("buildNvimRemoteOpenCommand() mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestBuildTmuxAttachCommandOutsideTmux(t *testing.T) {
	t.Setenv("TMUX", "")

	cmd, err := buildTmuxAttachCommand("canopy_session")
	if err != nil {
		t.Fatalf("buildTmuxAttachCommand() error = %v", err)
	}

	if got, want := cmd.Args, []string{"tmux", "attach-session", "-t", "canopy_session"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("buildTmuxAttachCommand() args = %v, want %v", got, want)
	}
	if cmd.Env != nil {
		t.Fatalf("buildTmuxAttachCommand() env = %v, want nil outside tmux", cmd.Env)
	}
}

func TestBuildTmuxAttachCommandInsideTmux(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,123,0")
	t.Setenv("PATH", "/usr/bin")

	orig := currentTmuxSocketPath
	currentTmuxSocketPath = func() (string, error) {
		return "/tmp/tmux-1000/custom.sock", nil
	}
	defer func() { currentTmuxSocketPath = orig }()

	cmd, err := buildTmuxAttachCommand("canopy_session")
	if err != nil {
		t.Fatalf("buildTmuxAttachCommand() error = %v", err)
	}

	if got, want := cmd.Args, []string{"tmux", "-S", "/tmp/tmux-1000/custom.sock", "attach-session", "-t", "canopy_session"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("buildTmuxAttachCommand() args = %v, want %v", got, want)
	}
	if containsEnvVar(cmd.Env, "TMUX=") {
		t.Fatalf("buildTmuxAttachCommand() env unexpectedly contains TMUX: %v", cmd.Env)
	}
	if !containsEnvVar(cmd.Env, "PATH=/usr/bin") {
		t.Fatalf("buildTmuxAttachCommand() env lost unrelated vars: %v", cmd.Env)
	}
}

func TestBuildTmuxAttachCommandInsideTmuxSocketFailure(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,123,0")

	orig := currentTmuxSocketPath
	currentTmuxSocketPath = func() (string, error) {
		return "", errors.New("boom")
	}
	defer func() { currentTmuxSocketPath = orig }()

	_, err := buildTmuxAttachCommand("canopy_session")
	if err == nil {
		t.Fatal("buildTmuxAttachCommand() error = nil, want failure")
	}
	if got, want := err.Error(), "boom"; got != want {
		t.Fatalf("buildTmuxAttachCommand() error = %q, want %q", got, want)
	}
}

func containsEnvVar(env []string, prefix string) bool {
	for _, entry := range env {
		if len(entry) >= len(prefix) && entry[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}
