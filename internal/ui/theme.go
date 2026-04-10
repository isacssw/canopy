package ui

import "github.com/charmbracelet/lipgloss"

// Theme holds the colour palette used throughout the UI.
type Theme struct {
	Border         lipgloss.Color
	Muted          lipgloss.Color
	Text           lipgloss.Color
	Accent         lipgloss.Color
	Green          lipgloss.Color
	Yellow         lipgloss.Color
	Red            lipgloss.Color
	Purple         lipgloss.Color
	SelectedBg     lipgloss.Color
	SelectedText   lipgloss.Color
	SelectedMuted  lipgloss.Color
	SelectedAccent lipgloss.Color
	StatusBarBg    lipgloss.Color
	StatusBarText  lipgloss.Color
	KeyBg          lipgloss.Color
	KeyText        lipgloss.Color
}

// Themes is the map of built-in named palettes.
var Themes = map[string]Theme{
	"github-dark": {
		Border:         lipgloss.Color("#30363d"),
		Muted:          lipgloss.Color("#8b949e"),
		Text:           lipgloss.Color("#e6edf3"),
		Accent:         lipgloss.Color("#58a6ff"),
		Green:          lipgloss.Color("#3fb950"),
		Yellow:         lipgloss.Color("#d29922"),
		Red:            lipgloss.Color("#f85149"),
		Purple:         lipgloss.Color("#bc8cff"),
		SelectedBg:     lipgloss.Color("#1f2937"),
		SelectedText:   lipgloss.Color("#f0f6fc"),
		SelectedMuted:  lipgloss.Color("#b6c2cf"),
		SelectedAccent: lipgloss.Color("#79c0ff"),
		StatusBarBg:    lipgloss.Color("#161b22"),
		StatusBarText:  lipgloss.Color("#c9d1d9"),
		KeyBg:          lipgloss.Color("#21262d"),
		KeyText:        lipgloss.Color("#79c0ff"),
	},
	"nord": {
		Border:         lipgloss.Color("#4c566a"),
		Muted:          lipgloss.Color("#616e88"),
		Text:           lipgloss.Color("#eceff4"),
		Accent:         lipgloss.Color("#88c0d0"),
		Green:          lipgloss.Color("#a3be8c"),
		Yellow:         lipgloss.Color("#ebcb8b"),
		Red:            lipgloss.Color("#bf616a"),
		Purple:         lipgloss.Color("#b48ead"),
		SelectedBg:     lipgloss.Color("#434c5e"),
		SelectedText:   lipgloss.Color("#f4f7fb"),
		SelectedMuted:  lipgloss.Color("#d8dee9"),
		SelectedAccent: lipgloss.Color("#8fbcbb"),
		StatusBarBg:    lipgloss.Color("#2e3440"),
		StatusBarText:  lipgloss.Color("#d8dee9"),
		KeyBg:          lipgloss.Color("#434c5e"),
		KeyText:        lipgloss.Color("#8fbcbb"),
	},
	"catppuccin": {
		Border:         lipgloss.Color("#45475a"),
		Muted:          lipgloss.Color("#6c7086"),
		Text:           lipgloss.Color("#cdd6f4"),
		Accent:         lipgloss.Color("#89b4fa"),
		Green:          lipgloss.Color("#a6e3a1"),
		Yellow:         lipgloss.Color("#f9e2af"),
		Red:            lipgloss.Color("#f38ba8"),
		Purple:         lipgloss.Color("#cba6f7"),
		SelectedBg:     lipgloss.Color("#3a3d52"),
		SelectedText:   lipgloss.Color("#eff1f5"),
		SelectedMuted:  lipgloss.Color("#cdd6f4"),
		SelectedAccent: lipgloss.Color("#b4befe"),
		StatusBarBg:    lipgloss.Color("#1e1e2e"),
		StatusBarText:  lipgloss.Color("#bac2de"),
		KeyBg:          lipgloss.Color("#45475a"),
		KeyText:        lipgloss.Color("#b4befe"),
	},
	"light": {
		Border:         lipgloss.Color("#d0d7de"),
		Muted:          lipgloss.Color("#374151"),
		Text:           lipgloss.Color("#0f172a"),
		Accent:         lipgloss.Color("#0969da"),
		Green:          lipgloss.Color("#1a7f37"),
		Yellow:         lipgloss.Color("#9a6700"),
		Red:            lipgloss.Color("#cf222e"),
		Purple:         lipgloss.Color("#8250df"),
		SelectedBg:     lipgloss.Color("#d6e7ff"),
		SelectedText:   lipgloss.Color("#0b1220"),
		SelectedMuted:  lipgloss.Color("#334155"),
		SelectedAccent: lipgloss.Color("#0550ae"),
		StatusBarBg:    lipgloss.Color("#f6f8fa"),
		StatusBarText:  lipgloss.Color("#0f172a"),
		KeyBg:          lipgloss.Color("#eaeef2"),
		KeyText:        lipgloss.Color("#0550ae"),
	},
}

// ThemeByName returns the named theme, falling back to github-dark for unknown names.
func ThemeByName(name string) Theme {
	if t, ok := Themes[name]; ok {
		return t
	}
	return Themes["github-dark"]
}
