package worktree

import "testing"

func TestParseMarksMainByRepoRoot(t *testing.T) {
	raw := `worktree /tmp/repo
HEAD 1111111111111111111111111111111111111111
branch refs/heads/main

worktree /tmp/repo-feature
HEAD 2222222222222222222222222222222222222222
branch refs/heads/feat/callback-safety
`

	got := parse(raw, "/tmp/repo")
	if len(got) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(got))
	}
	if !got[0].IsMain {
		t.Fatalf("expected first worktree to be main")
	}
	if got[1].IsMain {
		t.Fatalf("expected second worktree to be non-main")
	}
	if got[1].BaseBranch != "main" {
		t.Fatalf("expected inferred base to be main, got %q", got[1].BaseBranch)
	}
}

func TestParseDetachedWorktree(t *testing.T) {
	raw := `worktree /tmp/repo
HEAD 1111111111111111111111111111111111111111
branch refs/heads/main

worktree /tmp/repo-detached
HEAD 3333333333333333333333333333333333333333
detached
`

	got := parse(raw, "/tmp/repo")
	if len(got) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(got))
	}
	if got[1].Branch != "" {
		t.Fatalf("expected detached worktree branch to be empty, got %q", got[1].Branch)
	}
	if got[1].BaseBranch != "" {
		t.Fatalf("expected detached worktree base to be empty, got %q", got[1].BaseBranch)
	}
}

func TestDetectBase(t *testing.T) {
	tests := []struct {
		branch string
		want   string
	}{
		{branch: "feat/new-ui", want: "main"},
		{branch: "fix/bug-123", want: "main"},
		{branch: "refactor/model-split", want: "main"},
		{branch: "main", want: ""},
		{branch: "dev", want: ""},
	}

	for _, tt := range tests {
		if got := detectBase(tt.branch); got != tt.want {
			t.Fatalf("detectBase(%q) = %q, want %q", tt.branch, got, tt.want)
		}
	}
}
