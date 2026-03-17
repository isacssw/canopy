package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func colorDiff(content string, t Theme) string {
	if content == "" {
		return content
	}
	addedStyle := lipgloss.NewStyle().Foreground(t.Green)
	removedStyle := lipgloss.NewStyle().Foreground(t.Red)
	hunkStyle := lipgloss.NewStyle().Foreground(t.Accent)
	headerStyle := lipgloss.NewStyle().Foreground(t.Muted).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(t.Text)

	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			out = append(out, addedStyle.Render(line))
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			out = append(out, removedStyle.Render(line))
		case strings.HasPrefix(line, "@@"):
			out = append(out, hunkStyle.Render(line))
		case strings.HasPrefix(line, "diff --git"),
			strings.HasPrefix(line, "index "),
			strings.HasPrefix(line, "---"),
			strings.HasPrefix(line, "+++"),
			strings.HasPrefix(line, "new file"),
			strings.HasPrefix(line, "deleted file"),
			strings.HasPrefix(line, "rename "),
			strings.HasPrefix(line, "similarity "),
			strings.HasPrefix(line, "Binary files"):
			out = append(out, headerStyle.Render(line))
		default:
			out = append(out, normalStyle.Render(line))
		}
	}
	return strings.Join(out, "\n")
}
