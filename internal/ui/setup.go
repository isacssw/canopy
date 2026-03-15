package ui

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/isacssw/canopy/internal/config"
)

// ── Step types ───────────────────────────────────────────────────────────────

type setupStep int

const (
	stepWelcome  setupStep = iota // logo + tagline, press any key
	stepAgent                     // configure agent command
	stepComplete                  // saved, show what's next
)

// ── Messages ─────────────────────────────────────────────────────────────────

type pathCheckMsg struct {
	command string
	found   bool
}

type configSavedMsg struct {
	cfg *config.Config
	err error
}

// ── Styles ───────────────────────────────────────────────────────────────────

var (
	styleSetupCard = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ThemeByName("").Border).
			Padding(1, 3)

	styleSetupHeading = lipgloss.NewStyle().Foreground(ThemeByName("").Text).Bold(true)
	styleSetupOk      = lipgloss.NewStyle().Foreground(ThemeByName("").Green)
	styleSetupWarn    = lipgloss.NewStyle().Foreground(ThemeByName("").Yellow)
	styleSetupHint    = lipgloss.NewStyle().Foreground(ThemeByName("").Muted)
	styleSummaryKey   = lipgloss.NewStyle().Foreground(ThemeByName("").Muted)
	styleSummaryVal   = lipgloss.NewStyle().Foreground(ThemeByName("").Accent).Bold(true)
)

// ── Model ────────────────────────────────────────────────────────────────────

type SetupModel struct {
	step              setupStep
	width, height     int
	agentInput        textinput.Model
	agentValid        bool
	agentChecked      bool
	agentCheckPending bool
	lastCheckedVal    string
	result            *config.Config
}

func NewSetupModel() *SetupModel {
	ti := textinput.New()
	ti.SetValue("claude")
	ti.Prompt = "> "
	ti.CharLimit = 80

	return &SetupModel{
		step:       stepWelcome,
		agentInput: ti,
	}
}

// Result returns the saved config, or nil if the user cancelled.
func (m *SetupModel) Result() *config.Config { return m.result }

// ── Init ─────────────────────────────────────────────────────────────────────

func (m *SetupModel) Init() tea.Cmd { return nil }

// ── Update ───────────────────────────────────────────────────────────────────

func (m *SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		return m.handleKey(msg)

	case pathCheckMsg:
		// Ignore stale responses from superseded checks.
		if msg.command == m.agentInput.Value() {
			m.agentCheckPending = false
			m.agentChecked = true
			m.agentValid = msg.found
		}

	case configSavedMsg:
		if msg.err != nil {
			// Surface error as a status but stay on the agent step.
			m.agentCheckPending = false
			m.agentChecked = true
			m.agentValid = false
			return m, nil
		}
		m.result = msg.cfg
		m.step = stepComplete
	}

	// Propagate non-key messages (e.g. cursor blink) to the active input.
	if m.step == stepAgent {
		var cmd tea.Cmd
		m.agentInput, cmd = m.agentInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *SetupModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	switch m.step {

	case stepWelcome:
		if msg.String() == "esc" {
			return m, tea.Quit
		}
		m.step = stepAgent
		blinkCmd := m.agentInput.Focus()
		m.lastCheckedVal = m.agentInput.Value()
		m.agentCheckPending = true
		return m, tea.Batch(blinkCmd, m.checkPathCmd(m.agentInput.Value()))

	case stepAgent:
		switch msg.String() {
		case "esc":
			return m, tea.Quit
		case "enter":
			val := strings.TrimSpace(m.agentInput.Value())
			if val == "" {
				val = "claude"
			}
			return m, m.saveConfigCmd(val)
		default:
			var cmd tea.Cmd
			m.agentInput, cmd = m.agentInput.Update(msg)
			newVal := m.agentInput.Value()
			if newVal != m.lastCheckedVal {
				m.lastCheckedVal = newVal
				m.agentChecked = false
				m.agentCheckPending = true
				return m, tea.Batch(cmd, m.checkPathCmd(newVal))
			}
			return m, cmd
		}

	case stepComplete:
		if msg.String() == "enter" || msg.String() == "esc" {
			return m, tea.Quit
		}
	}

	return m, nil
}

// ── Commands ─────────────────────────────────────────────────────────────────

func (m *SetupModel) checkPathCmd(command string) tea.Cmd {
	return func() tea.Msg {
		_, err := exec.LookPath(command)
		return pathCheckMsg{command: command, found: err == nil}
	}
}

func (m *SetupModel) saveConfigCmd(agentCmd string) tea.Cmd {
	return func() tea.Msg {
		cfg := &config.Config{AgentCommand: agentCmd}
		if err := config.Save(cfg); err != nil {
			return configSavedMsg{err: err}
		}
		return configSavedMsg{cfg: cfg}
	}
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m *SetupModel) View() string {
	if m.width == 0 {
		return ""
	}

	cardW := m.width - 4
	if cardW > 72 {
		cardW = 72
	}

	var inner string
	switch m.step {
	case stepWelcome:
		inner = m.viewWelcome(cardW)
	case stepAgent:
		inner = m.viewAgent()
	case stepComplete:
		inner = m.viewComplete()
	}

	card := styleSetupCard.Width(cardW).Render(inner)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, card)
}

func (m *SetupModel) viewWelcome(_ int) string {
	logo := lipgloss.NewStyle().Foreground(colorForest).Bold(true).Render(logoASCII)
	tag := lipgloss.NewStyle().Foreground(colorBark).Render(Tagline)
	hint := styleSetupHint.Render("press any key to begin setup")

	return lipgloss.JoinVertical(lipgloss.Left,
		logo,
		"",
		"  "+tag,
		"",
		"  "+hint,
	)
}

func (m *SetupModel) viewAgent() string {
	step := styleSetupHint.Render("step 1 / 2")
	heading := styleSetupHeading.Render("Agent Command")
	desc := styleSetupHint.Render(
		"Command used to launch an AI coding agent in each worktree.\n" +
			"Defaults to `claude` (Claude Code).",
	)

	input := m.agentInput.View()

	var validation string
	switch {
	case m.agentCheckPending:
		validation = styleSetupHint.Render("  checking…")
	case m.agentChecked && m.agentValid:
		validation = styleSetupOk.Render("  ✓ found in PATH")
	case m.agentChecked && !m.agentValid:
		validation = styleSetupWarn.Render("  ⚠ not found in PATH — you can still continue")
	}

	hint := styleSetupHint.Render("enter to continue  •  esc to cancel")

	parts := []string{step, "", heading, "", desc, "", input}
	if validation != "" {
		parts = append(parts, validation)
	}
	parts = append(parts, "", hint)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m *SetupModel) viewComplete() string {
	heading := styleSetupOk.Bold(true).Render("Setup complete!")
	saved := styleSetupOk.Render(
		fmt.Sprintf("  ✓ configuration saved to %s", config.DefaultConfigPath()),
	)

	summary := lipgloss.JoinHorizontal(lipgloss.Top,
		styleSummaryKey.Render("  agent  "),
		styleSummaryVal.Render(m.result.AgentCommand),
	)

	whatsNext := lipgloss.JoinVertical(lipgloss.Left,
		styleSetupHeading.Render("what's next"),
		"",
		styleSetupHint.Render("  n   create a new worktree"),
		styleSetupHint.Render("  r   run an agent in the selected worktree"),
		styleSetupHint.Render("  a   attach to a running agent session"),
	)

	hint := styleSetupHint.Render("press enter to launch canopy")

	return lipgloss.JoinVertical(lipgloss.Left,
		heading,
		"",
		saved,
		"",
		summary,
		"",
		whatsNext,
		"",
		hint,
	)
}
