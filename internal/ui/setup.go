package ui

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/isacssw/canopy/internal/cmdline"
	"github.com/isacssw/canopy/internal/config"
)

// ── Step types ───────────────────────────────────────────────────────────────

type setupStep int

const (
	stepWelcome   setupStep = iota // logo + tagline, press any key
	stepAgent                      // configure agent command
	stepAgentName                  // name for the agent profile
	stepComplete                   // saved, show what's next
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

// ── Model ────────────────────────────────────────────────────────────────────

type setupStyles struct {
	card       lipgloss.Style
	heading    lipgloss.Style
	ok         lipgloss.Style
	warn       lipgloss.Style
	hint       lipgloss.Style
	summaryKey lipgloss.Style
	summaryVal lipgloss.Style
}

type SetupModel struct {
	step              setupStep
	width, height     int
	agentInput        textinput.Model
	agentValid        bool
	agentChecked      bool
	agentCheckPending bool
	lastCheckedVal    string
	nameInput         textinput.Model
	pendingAgentCmd   string
	result            *config.Config
	styles            setupStyles
}

func NewSetupModel() *SetupModel {
	theme := ThemeByName("")
	ti := textinput.New()
	ti.SetValue("claude")
	ti.Prompt = "> "
	ti.CharLimit = 80

	ni := textinput.New()
	ni.Prompt = "> "
	ni.CharLimit = 40

	return &SetupModel{
		step:       stepWelcome,
		agentInput: ti,
		nameInput:  ni,
		styles:     newSetupStyles(theme),
	}
}

func newSetupStyles(t Theme) setupStyles {
	return setupStyles{
		card: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Border).
			Padding(1, 3),
		heading: lipgloss.NewStyle().
			Foreground(t.Text).
			Bold(true),
		ok: lipgloss.NewStyle().
			Foreground(t.Green),
		warn: lipgloss.NewStyle().
			Foreground(t.Yellow),
		hint: lipgloss.NewStyle().
			Foreground(t.Muted),
		summaryKey: lipgloss.NewStyle().
			Foreground(t.Muted),
		summaryVal: lipgloss.NewStyle().
			Foreground(t.Accent).
			Bold(true),
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
	if m.step == stepAgentName {
		var cmd tea.Cmd
		m.nameInput, cmd = m.nameInput.Update(msg)
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
			m.pendingAgentCmd = val
			defaultName := cmdline.Executable(val)
			if defaultName == "" {
				defaultName = val
			}
			m.nameInput.SetValue(defaultName)
			m.step = stepAgentName
			return m, m.nameInput.Focus()
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

	case stepAgentName:
		switch msg.String() {
		case "esc":
			m.step = stepAgent
			return m, m.agentInput.Focus()
		case "enter":
			name := strings.TrimSpace(m.nameInput.Value())
			if name == "" {
				name = cmdline.Executable(m.pendingAgentCmd)
				if name == "" {
					name = m.pendingAgentCmd
				}
			}
			return m, m.saveConfigCmd(name, m.pendingAgentCmd)
		default:
			var cmd tea.Cmd
			m.nameInput, cmd = m.nameInput.Update(msg)
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
		exe := cmdline.Executable(command)
		if exe == "" {
			return pathCheckMsg{command: command, found: false}
		}
		_, err := exec.LookPath(exe)
		return pathCheckMsg{command: command, found: err == nil}
	}
}

func (m *SetupModel) saveConfigCmd(name, agentCmd string) tea.Cmd {
	return func() tea.Msg {
		cfg := &config.Config{
			Agents: []config.AgentProfile{{Name: name, Command: agentCmd}},
		}
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
	case stepAgentName:
		inner = m.viewAgentName()
	case stepComplete:
		inner = m.viewComplete()
	}

	card := m.styles.card.Width(cardW).Render(inner)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, card)
}

func (m *SetupModel) viewWelcome(_ int) string {
	logo := lipgloss.NewStyle().Foreground(colorForest).Bold(true).Render(logoASCII)
	tag := lipgloss.NewStyle().Foreground(colorBark).Render(Tagline)
	hint := m.styles.hint.Render("press any key to begin setup")

	return lipgloss.JoinVertical(lipgloss.Left,
		logo,
		"",
		"  "+tag,
		"",
		"  "+hint,
	)
}

func (m *SetupModel) viewAgent() string {
	step := m.styles.hint.Render("step 1 / 3")
	heading := m.styles.heading.Render("Agent Command")
	desc := m.styles.hint.Render(
		"Command used to launch an AI coding agent in each worktree.\n" +
			"Defaults to `claude` (Claude Code).",
	)

	input := m.agentInput.View()

	var validation string
	switch {
	case m.agentCheckPending:
		validation = m.styles.hint.Render("  checking…")
	case m.agentChecked && m.agentValid:
		validation = m.styles.ok.Render("  ✓ found in PATH")
	case m.agentChecked && !m.agentValid:
		validation = m.styles.warn.Render("  ⚠ not found in PATH — you can still continue")
	}

	hint := m.styles.hint.Render("enter to continue  •  esc to cancel")

	parts := []string{step, "", heading, "", desc, "", input}
	if validation != "" {
		parts = append(parts, validation)
	}
	parts = append(parts, "", hint)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m *SetupModel) viewAgentName() string {
	step := m.styles.hint.Render("step 2 / 3")
	heading := m.styles.heading.Render("Agent Name")
	desc := m.styles.hint.Render(
		"A short label for this agent profile, shown in the picker.\n" +
			"Press enter to accept the default.",
	)
	input := m.nameInput.View()
	hint := m.styles.hint.Render("enter to continue  •  esc to go back")

	return lipgloss.JoinVertical(lipgloss.Left,
		step, "", heading, "", desc, "", input, "", hint,
	)
}

func (m *SetupModel) viewComplete() string {
	heading := m.styles.ok.Bold(true).Render("Setup complete!")
	saved := m.styles.ok.Render(
		fmt.Sprintf("  ✓ configuration saved to %s", config.DefaultConfigPath()),
	)

	profile := m.result.ResolvedAgents()[0]
	summary := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top,
			m.styles.summaryKey.Render("  name     "),
			m.styles.summaryVal.Render(profile.Name),
		),
		lipgloss.JoinHorizontal(lipgloss.Top,
			m.styles.summaryKey.Render("  command  "),
			m.styles.summaryVal.Render(profile.Command),
		),
	)

	tip := m.styles.hint.Render("tip: add more agents in ~/.config/canopy/config.json")

	whatsNext := lipgloss.JoinVertical(lipgloss.Left,
		m.styles.heading.Render("what's next"),
		"",
		m.styles.hint.Render("  n   create a new worktree"),
		m.styles.hint.Render("  r   run an agent in the selected worktree"),
		m.styles.hint.Render("  a   attach to a running agent session"),
	)

	hint := m.styles.hint.Render("press enter to launch canopy")

	return lipgloss.JoinVertical(lipgloss.Left,
		heading,
		"",
		saved,
		"",
		summary,
		"",
		tip,
		"",
		whatsNext,
		"",
		hint,
	)
}
