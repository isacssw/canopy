package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/isacssw/canopy/internal/agent"
	"github.com/isacssw/canopy/internal/config"
	"github.com/isacssw/canopy/internal/worktree"
)

func TestResolveOutputColorModeDefaultsToAdaptiveForLightTheme(t *testing.T) {
	if got := resolveOutputColorMode(&config.Config{Theme: "light"}, "light"); got != outputColorModeAdaptive {
		t.Fatalf("resolveOutputColorMode() = %q, want %q", got, outputColorModeAdaptive)
	}
}

func TestResolveOutputColorModeHonorsExplicitConfig(t *testing.T) {
	if got := resolveOutputColorMode(&config.Config{Theme: "light", OutputColors: "preserve"}, "light"); got != outputColorModePreserve {
		t.Fatalf("resolveOutputColorMode() = %q, want %q", got, outputColorModePreserve)
	}
}

func TestRenderSnapshotPlainStripsANSI(t *testing.T) {
	theme := ThemeByName("light")
	m := &Model{
		theme:        theme,
		outputColors: outputColorModePlain,
		st:           newStyles(theme),
	}

	got := m.renderSnapshot("\x1b[31merror\x1b[0m\nok")

	if strings.Contains(got, "\x1b[") {
		t.Fatalf("renderSnapshot() left ANSI escapes in plain mode: %q", got)
	}
	if !strings.Contains(got, "error\nok") {
		t.Fatalf("renderSnapshot() = %q, want stripped content", got)
	}
}

func TestRenderSnapshotPreserveKeepsANSI(t *testing.T) {
	theme := ThemeByName("github-dark")
	m := &Model{
		theme:        theme,
		outputColors: outputColorModePreserve,
		st:           newStyles(theme),
	}

	got := m.renderSnapshot("\x1b[31merror\x1b[0m")

	if !strings.Contains(got, "\x1b[31m") {
		t.Fatalf("renderSnapshot() = %q, want ANSI-preserved output", got)
	}
}

func TestRenderSnapshotAdaptiveRemapsLightModeANSI(t *testing.T) {
	theme := ThemeByName("light")
	m := &Model{
		theme:        theme,
		outputColors: outputColorModeAdaptive,
		st:           newStyles(theme),
	}

	got := m.renderSnapshot("\x1b[97mbright\x1b[41m alert\x1b[0m")

	if strings.Contains(got, "\x1b[97m") || strings.Contains(got, "\x1b[41m") {
		t.Fatalf("renderSnapshot() left unsafe ANSI colors in adaptive mode: %q", got)
	}
	if !strings.Contains(got, "\x1b[38;2;15;23;42mbright") {
		t.Fatalf("renderSnapshot() = %q, want remapped foreground color", got)
	}
	if !strings.Contains(got, "\x1b[49m") {
		t.Fatalf("renderSnapshot() = %q, want default background reset", got)
	}
}

func TestRenderWorktreePanelSelectedEntryUsesFocusRail(t *testing.T) {
	theme := ThemeByName("light")
	m := &Model{
		width:  100,
		height: 12,
		cursor: 0,
		theme:  theme,
		st:     newStyles(theme),
		entries: []entry{
			{
				wt:    worktree.Worktree{Branch: "themes-fix", Path: "/tmp/themes-fix"},
				agent: agent.New(),
			},
		},
	}

	view := m.renderWorktreePanel(38, 11)

	if !strings.Contains(view, "┃") {
		t.Fatalf("renderWorktreePanel() missing selected focus rail:\n%s", view)
	}
	if got := lipgloss.Width(view); got <= 0 {
		t.Fatalf("renderWorktreePanel() width = %d, want positive width", got)
	}
}

func TestRenderRootFillsAppSurface(t *testing.T) {
	theme := ThemeByName("light")
	m := &Model{
		width:  20,
		height: 4,
		theme:  theme,
		st:     newStyles(theme),
	}

	view := m.renderRoot("hi")

	lines := strings.Split(view, "\n")
	if len(lines) != 4 {
		t.Fatalf("renderRoot() line count = %d, want 4", len(lines))
	}
	if got, want := lipgloss.Height(view), 4; got != want {
		t.Fatalf("renderRoot() height = %d, want %d", got, want)
	}
	for _, line := range lines {
		if got, want := lipgloss.Width(line), 20; got != want {
			t.Fatalf("renderRoot() line width = %d, want %d", got, want)
		}
	}
}
