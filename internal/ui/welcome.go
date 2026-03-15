package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Forest theme colors — used by welcome and setup screens.
var (
	colorForest = lipgloss.Color("#5dba7e") // brand accent green
	colorBark   = lipgloss.Color("#3d6b52") // muted dark green
)

// Tagline is the canonical one-liner shown on the welcome and setup screens.
const Tagline = "your agents. one view."

// logoASCII is the "canopy" wordmark in ANSI Shadow style.
const logoASCII = ` ██████╗ █████╗ ███╗  ██╗ ██████╗ ██████╗ ██╗   ██╗
██╔════╝██╔══██╗████╗ ██║██╔═══██╗██╔══██╗╚██╗ ██╔╝
██║     ███████║██╔██╗██║██║   ██║██████╔╝ ╚████╔╝
██║     ██╔══██║██║╚████║██║   ██║██╔═══╝   ╚██╔╝
╚██████╗██║  ██║██║ ╚███║╚██████╔╝██║        ██║
 ╚═════╝╚═╝  ╚═╝╚═╝  ╚══╝ ╚═════╝ ╚═╝        ╚═╝  `

// PrintWelcome prints the logo and tagline to stdout.
// Called on first run, before the setup wizard launches.
func PrintWelcome() {
	logo := lipgloss.NewStyle().Foreground(colorForest).Bold(true).Render(logoASCII)
	tag := lipgloss.NewStyle().Foreground(colorBark).Render(Tagline)
	fmt.Println(logo)
	fmt.Println()
	fmt.Println("  " + tag)
	fmt.Println()
}
