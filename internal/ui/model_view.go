package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) View() string {
	if m.width == 0 {
		return "loading…"
	}

	if m.mode == modeHelp {
		return m.renderHelp()
	}

	if m.mode == modeAgentPicker {
		return m.renderAgentPicker()
	}

	if m.mode == modeDiff {
		return m.renderDiff()
	}

	leftW := m.leftPanelWidth()
	rightW := m.width - leftW - 3
	innerH := m.height - 4 // leave room for status bar + border

	left := m.renderWorktreePanel(leftW, innerH)
	right := m.renderOutputPanel(rightW, innerH)

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	status := m.renderStatusBar()

	switch m.mode {
	case modeNewWorktree:
		return m.renderInputModal("New Worktree", "Branch name")
	case modeNewWorktreeBase:
		return m.renderInputModal("Base Branch", "Base branch")
	case modeSendInput:
		return m.renderInputModal("Send to Agent", "Message")
	}

	return lipgloss.JoinVertical(lipgloss.Left, body, status)
}

func (m *Model) renderWorktreePanel(w, h int) string {
	title := m.st.panelTitle.Render("  worktrees")

	var rows []string
	for i, e := range m.entries {
		icon := statusIcon(e.agent.Status())
		iconS := m.statusStyle(e.agent.Status()).Render(icon)

		branch := e.wt.Branch
		if branch == "" {
			branch = filepath.Base(e.wt.Path)
		}

		base := ""
		if e.wt.BaseBranch != "" {
			base = m.st.muted.Render("  " + e.wt.BaseBranch + " ← " + branch)
		}

		statusTxt := m.statusStyle(e.agent.Status()).Render(e.agent.Status().String())

		badge := ""
		if e.unread {
			badge = " " + lipgloss.NewStyle().Foreground(m.theme.Yellow).Bold(true).Render("●")
		}
		line1 := fmt.Sprintf(" %s %s%s", iconS, m.st.normal.Render(branch), badge)
		line2 := base
		line3 := fmt.Sprintf("   %s", statusTxt)

		if e.wt.IsMain {
			line1 = fmt.Sprintf(" %s %s %s%s", iconS, m.st.normal.Render(branch), m.st.muted.Render("(main)"), badge)
			line2 = ""
		}

		block := line1
		if line2 != "" {
			block += "\n" + line2
		}
		block += "\n" + line3

		if i == m.cursor {
			block = m.st.selected.Width(w - 4).Render(block)
		} else {
			block = lipgloss.NewStyle().Width(w - 4).Render(block)
		}

		rows = append(rows, block)
		if i < len(m.entries)-1 {
			rows = append(rows, m.st.muted.Render(strings.Repeat("─", w-4)))
		}
	}

	if len(rows) == 0 {
		rows = []string{m.st.muted.Render("  no worktrees found\n  press n to create one")}
	}

	content := strings.Join(rows, "\n")
	panel := m.st.panelBorder.
		Width(w).
		Height(h).
		Render(title + "\n" + content)

	return panel
}

func (m *Model) renderOutputPanel(w, h int) string {
	title := " agent output"
	if len(m.entries) > 0 {
		e := m.entries[m.cursor]
		branch := e.wt.Branch
		if branch == "" {
			branch = filepath.Base(e.wt.Path)
		}
		title = fmt.Sprintf(" agent output  %s", m.st.muted.Render("← "+branch))
	}

	content := m.outputVP.View()
	panel := m.st.panelBorder.
		Width(w).
		Height(h).
		Render(m.st.panelTitle.Render(title) + "\n" + content)

	return panel
}

func (m *Model) renderDiff() string {
	title := m.st.panelTitle.Render(" diff")
	content := m.diffVP.View()
	panel := m.st.panelBorder.
		Width(m.width - 2).
		Height(m.height - 3).
		Render(title + "\n" + content)
	return lipgloss.JoinVertical(lipgloss.Left, panel, m.renderStatusBar())
}

func (m *Model) renderStatusBar() string {
	if m.mode == modePendingDelete && m.pendingDelete != nil {
		pd := m.pendingDelete
		deleteMsg := lipgloss.NewStyle().Foreground(m.theme.Red).Render(
			fmt.Sprintf("Deleting %q in %ds…  [u]ndo", pd.branch, pd.secsLeft),
		)
		return m.st.statusBar.Width(m.width).Render(" " + deleteMsg)
	}

	keys := []string{
		m.key("?", "help"),
		m.key("n", "new"),
		m.key("r", "run"),
		m.key("a", "attach"),
		m.key("x", "kill"),
		m.key("d", "diff"),
		m.key("D", "delete"),
		m.key("i", "send input"),
		m.key("R", "refresh"),
		m.key("q", "quit"),
	}
	left := strings.Join(keys, "  ")

	var rightParts []string
	if counts := m.statusCounts(); counts != "" {
		rightParts = append(rightParts, counts)
	}
	if m.statusMsg != "" {
		rightParts = append(rightParts, lipgloss.NewStyle().Foreground(m.theme.Yellow).Render(m.statusMsg))
	}
	right := strings.Join(rightParts, "   ")

	contentWidth := m.width - 2 // PaddingLeft(1) + PaddingRight(1)
	gap := contentWidth - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		gap = 2
	}
	bar := left + strings.Repeat(" ", gap) + right
	return m.st.statusBar.Width(m.width).Render(bar)
}

func (m *Model) renderInputModal(title, hint string) string {
	w := m.width - 8
	if w > 56 {
		w = 56
	}

	titleLine := lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true).Render(title)
	hintLine := m.st.muted.Render(hint)
	inputLine := "> " + m.input.View()
	keysLine := m.st.muted.Render("enter ↵ confirm   esc quit")

	inner := lipgloss.NewStyle().Width(w - 4).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			titleLine,
			"",
			hintLine,
			inputLine,
			"",
			keysLine,
		),
	)

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Border).
		Padding(1, 2).
		Render(inner)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, card)
}

func (m *Model) key(k, label string) string {
	return m.st.key.Render(k) + " " + m.st.muted.Render(label)
}

func (m *Model) renderAgentPicker() string {
	w := 52
	if m.width-8 < w {
		w = m.width - 8
	}

	titleLine := lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true).Render("Choose Agent")

	var rows []string
	for i, p := range m.pickerAgents {
		block := m.st.normal.Render(p.Name)
		if p.Command != p.Name {
			block += "\n" + m.st.muted.Render(p.Command)
		}
		if i == m.pickerCursor {
			block = m.st.selected.Width(w - 8).Render(block)
		}
		rows = append(rows, block)
	}

	list := strings.Join(rows, "\n")
	footer := m.st.muted.Render("j/k navigate   enter select   esc cancel")

	inner := lipgloss.NewStyle().Width(w - 4).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			titleLine,
			"",
			list,
			"",
			footer,
		),
	)

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Border).
		Padding(1, 2).
		Render(inner)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, card)
}

func (m *Model) renderHelp() string {
	w := 48
	if m.width-8 < w {
		w = m.width - 8
	}

	type binding struct{ key, desc string }
	sections := []struct {
		title    string
		bindings []binding
	}{
		{"Navigation", []binding{
			{"↑ / k", "move up"},
			{"↓ / j", "move down"},
		}},
		{"Worktrees", []binding{
			{"n", "new worktree"},
			{"D", "delete worktree"},
			{"R", "refresh list"},
		}},
		{"Agents", []binding{
			{"r", "run agent"},
			{"x", "kill agent"},
			{"a", "attach to session"},
			{"i", "send input"},
		}},
		{"View", []binding{
			{"d", "view diff"},
			{"?", "toggle help"},
			{"q", "quit"},
		}},
	}

	var lines []string
	lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true).Render("Keybindings"), "")
	for _, section := range sections {
		lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Muted).Bold(true).Render(section.title))
		for _, b := range section.bindings {
			lines = append(lines, fmt.Sprintf("  %s  %s", m.st.key.Render(b.key), m.st.muted.Render(b.desc)))
		}
		lines = append(lines, "")
	}
	lines = append(lines, m.st.muted.Render("esc / q / ?  close"))

	inner := lipgloss.NewStyle().Width(w - 4).Render(strings.Join(lines, "\n"))
	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Border).
		Padding(1, 2).
		Render(inner)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, card)
}
