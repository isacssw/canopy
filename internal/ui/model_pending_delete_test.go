package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/isacssw/canopy/internal/agent"
	"github.com/isacssw/canopy/internal/worktree"
)

func newPendingDeleteTestModel() *Model {
	m := &Model{
		entries: []entry{
			{wt: worktree.Worktree{Path: "/tmp/wt-a", Branch: "feat/a"}, agent: agent.New()},
			{wt: worktree.Worktree{Path: "/tmp/wt-b", Branch: "feat/b"}, agent: agent.New()},
		},
		cursor: 0,
		mode:   modePendingDelete,
		pendingDelete: &pendingDeleteState{
			wtPath:   "/tmp/wt-a",
			branch:   "feat/a",
			ag:       agent.New(),
			secsLeft: 5,
		},
	}
	m.outputVP = viewport.New(80, 20)
	return m
}

func TestPendingDeleteAllowsNavigation(t *testing.T) {
	m := newPendingDeleteTestModel()

	m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 1 {
		t.Fatalf("expected cursor to move down to 1, got %d", m.cursor)
	}
	if m.mode != modePendingDelete {
		t.Fatalf("expected modePendingDelete to remain active, got %v", m.mode)
	}
	if m.pendingDelete == nil {
		t.Fatal("expected pending delete to stay active after navigation")
	}

	m.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 0 {
		t.Fatalf("expected cursor to move back up to 0, got %d", m.cursor)
	}
}

func TestPendingDeleteUndoCancelsDelete(t *testing.T) {
	m := newPendingDeleteTestModel()

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})

	if m.pendingDelete != nil {
		t.Fatal("expected pending delete to be cleared after undo")
	}
	if m.mode != modeNormal {
		t.Fatalf("expected modeNormal after undo, got %v", m.mode)
	}
	if m.statusMsg != "delete cancelled" {
		t.Fatalf("expected status message %q, got %q", "delete cancelled", m.statusMsg)
	}
}
