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
	leftW := m.diffFileListWidth()
	rightW := m.width - leftW - 3
	innerH := m.height - 4

	left := m.renderDiffFileList(leftW, innerH)
	right := m.renderDiffPatch(rightW, innerH)

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	return lipgloss.JoinVertical(lipgloss.Left, body, m.renderDiffStatusBar())
}

func (m *Model) renderDiffFileList(w, h int) string {
	totalAdded := 0
	totalRemoved := 0
	for _, f := range m.diffFiles {
		totalAdded += f.Added
		totalRemoved += f.Removed
	}

	headerText := fmt.Sprintf(" %d files", len(m.diffFiles))
	stats := ""
	if totalAdded > 0 || totalRemoved > 0 {
		stats = fmt.Sprintf("  %s %s",
			lipgloss.NewStyle().Foreground(m.theme.Green).Render(fmt.Sprintf("+%d", totalAdded)),
			lipgloss.NewStyle().Foreground(m.theme.Red).Render(fmt.Sprintf("-%d", totalRemoved)),
		)
	}
	title := m.st.panelTitle.Render(headerText) + stats

	var rows []string
	visibleH := h - 1 // minus title line

	// Ensure cursor is visible within scroll window
	if m.diffCursor < m.diffFileScroll {
		m.diffFileScroll = m.diffCursor
	}
	if m.diffCursor >= m.diffFileScroll+visibleH {
		m.diffFileScroll = m.diffCursor - visibleH + 1
	}

	for i, f := range m.diffFiles {
		if i < m.diffFileScroll {
			continue
		}
		if len(rows) >= visibleH {
			break
		}

		icon := f.Status
		var iconStyle lipgloss.Style
		switch f.Status {
		case "A":
			iconStyle = lipgloss.NewStyle().Foreground(m.theme.Green)
		case "D":
			iconStyle = lipgloss.NewStyle().Foreground(m.theme.Red)
		case "R":
			iconStyle = lipgloss.NewStyle().Foreground(m.theme.Purple)
		default:
			iconStyle = lipgloss.NewStyle().Foreground(m.theme.Yellow)
		}

		name := f.Name
		if f.Status == "R" && f.OldName != "" {
			name = f.OldName + " -> " + f.Name
		}

		statStr := ""
		if f.IsBinary {
			statStr = m.st.muted.Render(" bin")
		} else if f.Added > 0 || f.Removed > 0 {
			statStr = " " +
				lipgloss.NewStyle().Foreground(m.theme.Green).Render(fmt.Sprintf("+%d", f.Added)) + " " +
				lipgloss.NewStyle().Foreground(m.theme.Red).Render(fmt.Sprintf("-%d", f.Removed))
		}

		line := fmt.Sprintf(" %s %s%s", iconStyle.Render(icon), m.st.normal.Render(name), statStr)

		lineW := w - 4
		if i == m.diffCursor {
			line = m.st.selected.Width(lineW).Render(line)
		} else {
			line = lipgloss.NewStyle().Width(lineW).Render(line)
		}
		rows = append(rows, line)
	}

	if len(rows) == 0 {
		rows = []string{m.st.muted.Render("  (no changes)")}
	}

	content := strings.Join(rows, "\n")

	borderStyle := m.st.panelBorder
	if m.diffFocus == diffFocusFiles {
		borderStyle = borderStyle.BorderForeground(m.theme.Accent)
	}

	return borderStyle.
		Width(w).
		Height(h).
		Render(title + "\n" + content)
}

func (m *Model) renderDiffPatch(w, h int) string {
	fileName := "(no file selected)"
	scrollPct := ""
	if len(m.diffFiles) > 0 && m.diffCursor < len(m.diffFiles) {
		fileName = m.diffFiles[m.diffCursor].Name
		pct := m.diffFileVP.ScrollPercent()
		scrollPct = m.st.muted.Render(fmt.Sprintf(" %d%%", int(pct*100)))
	}
	title := m.st.panelTitle.Render(" "+fileName) + scrollPct

	content := m.diffFileVP.View()

	borderStyle := m.st.panelBorder
	if m.diffFocus == diffFocusPatch {
		borderStyle = borderStyle.BorderForeground(m.theme.Accent)
	}

	return borderStyle.
		Width(w).
		Height(h).
		Render(title + "\n" + content)
}

func (m *Model) renderDiffStatusBar() string {
	keys := []string{
		m.key("tab", "switch panel"),
		m.key("j/k", "navigate"),
		m.key("J/K", "fast scroll"),
		m.key("g/G", "top/bottom"),
		m.key("esc", "close"),
	}
	left := strings.Join(keys, "  ")

	branch := ""
	if m.diffBranch != "" {
		branch = m.st.muted.Render(m.diffBranch)
	}

	contentWidth := m.width - 2
	gap := contentWidth - lipgloss.Width(left) - lipgloss.Width(branch)
	if gap < 2 {
		gap = 2
	}
	bar := left + strings.Repeat(" ", gap) + branch
	return m.st.statusBar.Width(m.width).Render(bar)
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
		{"Diff View", []binding{
			{"tab", "switch panel"},
			{"j / k", "navigate files / scroll"},
			{"J / K", "fast scroll patch"},
			{"g / G", "top / bottom of patch"},
			{"enter", "focus patch"},
			{"esc", "close diff"},
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
