package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/isacssw/canopy/internal/worktree"
)

func TestSyncDiffViewportUsesPanelContentArea(t *testing.T) {
	m := &Model{
		width:  120,
		height: 30,
		mode:   modeDiff,
		theme:  ThemeByName(""),
		st:     newStyles(ThemeByName("")),
		diffFiles: []worktree.DiffFile{
			{Name: "README.md", Patch: "diff --git a/README.md b/README.md\n@@ -1 +1 @@\n-old\n+new\n"},
		},
	}

	m.syncDiffViewport()

	leftW := m.diffFileListWidth()
	rightPanelW := m.width - leftW - 1
	panelH := m.height - 1
	if got, want := m.diffFileVP.Width, panelInnerWidth(rightPanelW); got != want {
		t.Fatalf("diff viewport width = %d, want %d", got, want)
	}
	if got, want := m.diffFileVP.Height, panelBodyHeight(panelH); got != want {
		t.Fatalf("diff viewport height = %d, want %d", got, want)
	}
}

func TestRenderDiffKeepsCrowdedLayoutWithinTerminalBounds(t *testing.T) {
	theme := ThemeByName("")
	m := &Model{
		width:      120,
		height:     18,
		mode:       modeDiff,
		theme:      theme,
		st:         newStyles(theme),
		diffCursor: 0,
		diffFocus:  diffFocusFiles,
		diffFiles: []worktree.DiffFile{
			{
				Name:   "testdata/diff-crowding/17-an-extremely-long-file-name-designed-to-push-the-file-list-title-stats-and-selection-row-together-for-visual-inspection.md",
				Status: "?",
				Added:  4,
				Patch:  "diff --git a/fixture b/fixture\n@@ -0,0 +1,3 @@\n+alpha\n+beta\n+gamma\n",
			},
			{
				Name:   "testdata/diff-crowding/nested/even-deeper/12-layout-regression-candidate-c.md",
				Status: "?",
				Added:  3,
				Patch:  "diff --git a/fixture2 b/fixture2\n@@ -0,0 +1,2 @@\n+one\n+two\n",
			},
		},
	}

	m.syncDiffViewport()
	view := m.View()

	if got, want := lipgloss.Height(view), m.height; got != want {
		t.Fatalf("rendered diff height = %d, want %d", got, want)
	}
	if !strings.Contains(view, "diff --git") {
		t.Fatal("expected patch pane content to remain visible in crowded diff view")
	}
	if !strings.Contains(view, "...") {
		t.Fatal("expected crowded diff view to truncate long file names")
	}
	for _, line := range strings.Split(view, "\n") {
		if w := lipgloss.Width(line); w > m.width {
			t.Fatalf("rendered line width = %d, exceeds terminal width %d\nline: %q", w, m.width, line)
		}
	}
}
