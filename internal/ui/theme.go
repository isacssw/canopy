package ui

import "github.com/charmbracelet/lipgloss"

// Theme holds the colour palette used throughout the UI.
type Theme struct {
	Border      lipgloss.Color
	Muted       lipgloss.Color
	Text        lipgloss.Color
	Accent      lipgloss.Color
	Green       lipgloss.Color
	Yellow      lipgloss.Color
	Red         lipgloss.Color
	Purple      lipgloss.Color
	SelectedBg  lipgloss.Color
	StatusBarBg lipgloss.Color
	KeyBg       lipgloss.Color
}

// Themes is the map of built-in named palettes.
var Themes = map[string]Theme{
	"github-dark": {
		Border:      lipgloss.Color("#30363d"),
		Muted:       lipgloss.Color("#8b949e"),
		Text:        lipgloss.Color("#e6edf3"),
		Accent:      lipgloss.Color("#58a6ff"),
		Green:       lipgloss.Color("#3fb950"),
		Yellow:      lipgloss.Color("#d29922"),
		Red:         lipgloss.Color("#f85149"),
		Purple:      lipgloss.Color("#bc8cff"),
		SelectedBg:  lipgloss.Color("#1c2128"),
		StatusBarBg: lipgloss.Color("#161b22"),
		KeyBg:       lipgloss.Color("#21262d"),
	},
	"nord": {
		Border:      lipgloss.Color("#4c566a"),
		Muted:       lipgloss.Color("#616e88"),
		Text:        lipgloss.Color("#eceff4"),
		Accent:      lipgloss.Color("#88c0d0"),
		Green:       lipgloss.Color("#a3be8c"),
		Yellow:      lipgloss.Color("#ebcb8b"),
		Red:         lipgloss.Color("#bf616a"),
		Purple:      lipgloss.Color("#b48ead"),
		SelectedBg:  lipgloss.Color("#3b4252"),
		StatusBarBg: lipgloss.Color("#2e3440"),
		KeyBg:       lipgloss.Color("#434c5e"),
	},
	"catppuccin": {
		Border:      lipgloss.Color("#45475a"),
		Muted:       lipgloss.Color("#6c7086"),
		Text:        lipgloss.Color("#cdd6f4"),
		Accent:      lipgloss.Color("#89b4fa"),
		Green:       lipgloss.Color("#a6e3a1"),
		Yellow:      lipgloss.Color("#f9e2af"),
		Red:         lipgloss.Color("#f38ba8"),
		Purple:      lipgloss.Color("#cba6f7"),
		SelectedBg:  lipgloss.Color("#313244"),
		StatusBarBg: lipgloss.Color("#1e1e2e"),
		KeyBg:       lipgloss.Color("#45475a"),
	},
	"light": {
		Border:      lipgloss.Color("#d0d7de"),
		Muted:       lipgloss.Color("#6e7781"),
		Text:        lipgloss.Color("#24292f"),
		Accent:      lipgloss.Color("#0969da"),
		Green:       lipgloss.Color("#1a7f37"),
		Yellow:      lipgloss.Color("#9a6700"),
		Red:         lipgloss.Color("#cf222e"),
		Purple:      lipgloss.Color("#8250df"),
		SelectedBg:  lipgloss.Color("#e8f0f7"),
		StatusBarBg: lipgloss.Color("#f6f8fa"),
		KeyBg:       lipgloss.Color("#eaeef2"),
	},
}

// ThemeByName returns the named theme, falling back to github-dark for unknown names.
func ThemeByName(name string) Theme {
	if t, ok := Themes[name]; ok {
		return t
	}
	return Themes["github-dark"]
}
