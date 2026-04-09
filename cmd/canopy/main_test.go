package main

import (
	"testing"

	"github.com/isacssw/canopy/internal/agent"
	"github.com/isacssw/canopy/internal/worktree"
)

func TestCurrentSessionMatchesWorktreeSession(t *testing.T) {
	repoRoot := "/tmp/project"
	wts := []worktree.Worktree{
		{Path: "/tmp/project", Branch: "main", IsMain: true},
		{Path: "/tmp/project-feature", Branch: "feat/nested"},
	}

	current := agent.SessionNameFor(repoRoot, "feat/nested", "/tmp/project-feature")
	if wt, ok := matchingWorktreeForSession(repoRoot, current, wts); !ok {
		t.Fatalf("expected session collision to be detected")
	} else if wt.Path != "/tmp/project-feature" {
		t.Fatalf("matched worktree path = %q, want %q", wt.Path, "/tmp/project-feature")
	}
}

func TestCurrentSessionNoMatch(t *testing.T) {
	repoRoot := "/tmp/project"
	wts := []worktree.Worktree{
		{Path: "/tmp/project", Branch: "main", IsMain: true},
		{Path: "/tmp/project-feature", Branch: "feat/nested"},
	}

	if _, ok := matchingWorktreeForSession(repoRoot, "other-session", wts); ok {
		t.Fatalf("expected non-matching session to be ignored")
	}
}
