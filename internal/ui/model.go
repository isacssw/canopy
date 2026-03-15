package ui

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"canopy/internal/agent"
	"canopy/internal/config"
	"canopy/internal/worktree"
)

// ── Colours & styles ────────────────────────────────────────────────────────

var (
	colorBg      = lipgloss.Color("#0d1117")
	colorBorder  = lipgloss.Color("#30363d")
	colorMuted   = lipgloss.Color("#8b949e")
	colorText    = lipgloss.Color("#e6edf3")
	colorAccent  = lipgloss.Color("#58a6ff")
	colorGreen   = lipgloss.Color("#3fb950")
	colorYellow  = lipgloss.Color("#d29922")
	colorRed     = lipgloss.Color("#f85149")
	colorPurple  = lipgloss.Color("#bc8cff")

	stylePanelBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorder)

	stylePanelTitle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true).
			PaddingLeft(1)

	styleSelected = lipgloss.NewStyle().
			Background(lipgloss.Color("#1c2128")).
			Foreground(colorAccent).
			Bold(true)

	styleNormal = lipgloss.NewStyle().
			Foreground(colorText)

	styleMuted = lipgloss.NewStyle().Foreground(colorMuted)

	styleStatusBar = lipgloss.NewStyle().
			Background(lipgloss.Color("#161b22")).
			Foreground(colorMuted).
			PaddingLeft(1).
			PaddingRight(1)

	styleKey = lipgloss.NewStyle().
			Background(lipgloss.Color("#21262d")).
			Foreground(colorAccent).
			PaddingLeft(1).
			PaddingRight(1)
)

func statusStyle(s agent.Status) lipgloss.Style {
	switch s {
	case agent.StatusRunning:
		return lipgloss.NewStyle().Foreground(colorGreen)
	case agent.StatusWaiting:
		return lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
	case agent.StatusDone:
		return lipgloss.NewStyle().Foreground(colorPurple)
	case agent.StatusError:
		return lipgloss.NewStyle().Foreground(colorRed)
	default:
		return lipgloss.NewStyle().Foreground(colorMuted)
	}
}

func statusIcon(s agent.Status) string {
	switch s {
	case agent.StatusRunning:
		return "●"
	case agent.StatusWaiting:
		return "⚠"
	case agent.StatusDone:
		return "✓"
	case agent.StatusError:
		return "✗"
	default:
		return "○"
	}
}

// ── State entry ─────────────────────────────────────────────────────────────

type entry struct {
	wt    worktree.Worktree
	agent *agent.Agent
}

// ── Modes ───────────────────────────────────────────────────────────────────

type mode int

const (
	modeNormal mode = iota
	modeNewWorktree
	modeNewWorktreeBase
	modeSendInput
	modeDiff
	modeConfirmDelete
	modeSetupAgent
)

// ── Messages ─────────────────────────────────────────────────────────────────

type agentChangedMsg struct{ idx int }
type worktreesRefreshedMsg struct{ entries []entry }
type errMsg struct{ err error }
type diffReadyMsg struct{ content string }

// ── Model ────────────────────────────────────────────────────────────────────

type Model struct {
	cfg       *config.Config
	entries   []entry
	cursor    int
	mode      mode
	width     int
	height    int

	outputVP  viewport.Model
	diffVP    viewport.Model

	input     textinput.Model
	inputHint string

	newBranch string // temp storage during two-step new-worktree flow

	statusMsg string
	program   *tea.Program
}

func New(cfg *config.Config) *Model {
	ti := textinput.New()
	ti.CharLimit = 80

	return &Model{
		cfg:   cfg,
		input: ti,
	}
}

func (m *Model) SetProgram(p *tea.Program) { m.program = p }

// ── Init ─────────────────────────────────────────────────────────────────────

func (m *Model) Init() tea.Cmd {
	return m.refreshWorktrees()
}

// ── Update ───────────────────────────────────────────────────────────────────

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizePanels()

	case worktreesRefreshedMsg:
		m.entries = msg.entries
		if m.cursor >= len(m.entries) {
			m.cursor = max(0, len(m.entries)-1)
		}
		m.syncOutputViewport()

	case agentChangedMsg:
		m.syncOutputViewport()

	case errMsg:
		m.statusMsg = "error: " + msg.err.Error()

	case diffReadyMsg:
		m.diffVP = viewport.New(m.width-4, m.height-6)
		m.diffVP.SetContent(msg.content)
		m.mode = modeDiff
		m.statusMsg = "esc to close diff"

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Propagate to active sub-components
	var cmd tea.Cmd
	if m.mode == modeNormal || m.mode == modeDiff {
		m.outputVP, cmd = m.outputVP.Update(msg)
		if m.mode == modeDiff {
			m.diffVP, _ = m.diffVP.Update(msg)
		}
	}
	if m.mode == modeNewWorktree || m.mode == modeNewWorktreeBase ||
		m.mode == modeSendInput || m.mode == modeSetupAgent {
		m.input, cmd = m.input.Update(msg)
	}
	return m, cmd
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	switch m.mode {

	case modeNormal:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.syncOutputViewport()
			}
		case "down", "j":
			if m.cursor < len(m.entries)-1 {
				m.cursor++
				m.syncOutputViewport()
			}
		case "n":
			m.enterInput(modeNewWorktree, "New branch name (e.g. feat/my-feature): ", "")
		case "r":
			return m, m.runAgent()
		case "x":
			return m, m.killAgent()
		case "d":
			return m, m.openDiff()
		case "D":
			if len(m.entries) == 0 {
				break
			}
			if m.entries[m.cursor].wt.IsMain {
				m.statusMsg = "cannot delete the main worktree"
			} else {
				m.mode = modeConfirmDelete
				m.statusMsg = fmt.Sprintf("Delete worktree %q? [y/n]", m.entries[m.cursor].wt.Branch)
			}
		case "i":
			if len(m.entries) > 0 {
				s := m.entries[m.cursor].agent.Status()
				if s == agent.StatusWaiting {
					m.enterInput(modeSendInput, "Send to agent: ", "")
				}
			}
		case "a":
			if len(m.entries) > 0 {
				return m, m.attachAgent()
			}
		case "R":
			return m, m.refreshWorktrees()
		}

	case modeNewWorktree:
		switch msg.String() {
		case "enter":
			m.newBranch = strings.TrimSpace(m.input.Value())
			if m.newBranch != "" {
				m.enterInput(modeNewWorktreeBase, "Base branch (default: main): ", "main")
			}
		case "esc":
			m.mode = modeNormal
			m.statusMsg = ""
		}

	case modeNewWorktreeBase:
		switch msg.String() {
		case "enter":
			base := strings.TrimSpace(m.input.Value())
			if base == "" {
				base = "main"
			}
			return m, m.createWorktree(m.newBranch, base)
		case "esc":
			m.mode = modeNormal
			m.statusMsg = ""
		}

	case modeSendInput:
		switch msg.String() {
		case "enter":
			text := strings.TrimSpace(m.input.Value())
			if text != "" && len(m.entries) > 0 {
				m.entries[m.cursor].agent.Send(text)
			}
			m.mode = modeNormal
			m.statusMsg = ""
		case "esc":
			m.mode = modeNormal
			m.statusMsg = ""
		}

	case modeDiff:
		switch msg.String() {
		case "esc", "q", "d":
			m.mode = modeNormal
			m.statusMsg = ""
		}

	case modeConfirmDelete:
		switch msg.String() {
		case "y", "Y":
			return m, m.deleteWorktree()
		default:
			m.mode = modeNormal
			m.statusMsg = ""
		}
	}

	var cmd tea.Cmd
	if m.mode == modeNewWorktree || m.mode == modeNewWorktreeBase ||
		m.mode == modeSendInput || m.mode == modeSetupAgent {
		m.input, cmd = m.input.Update(msg)
	}
	return m, cmd
}

// ── Commands ─────────────────────────────────────────────────────────────────

func (m *Model) refreshWorktrees() tea.Cmd {
	return func() tea.Msg {
		wts, err := worktree.List(m.cfg.RepoRoot)
		if err != nil {
			return errMsg{err}
		}
		// Preserve existing agents by path
		agentMap := map[string]*agent.Agent{}
		for _, e := range m.entries {
			agentMap[e.wt.Path] = e.agent
		}
		entries := make([]entry, 0, len(wts))
		for _, wt := range wts {
			a, ok := agentMap[wt.Path]
			if !ok {
				a = agent.New()
			}
			entries = append(entries, entry{wt: wt, agent: a})
		}
		return worktreesRefreshedMsg{entries: entries}
	}
}

func (m *Model) runAgent() tea.Cmd {
	if len(m.entries) == 0 {
		return nil
	}
	idx := m.cursor
	e := &m.entries[idx]
	if e.agent.Status() == agent.StatusRunning || e.agent.Status() == agent.StatusWaiting {
		m.statusMsg = "agent already running — press x to kill"
		return nil
	}
	e.agent.Reset()
	p := m.program
	e.agent.OnChange = func() {
		if p != nil {
			p.Send(agentChangedMsg{idx: idx})
		}
	}
	if err := e.agent.Start(e.wt.Path, m.cfg.AgentCommand, e.wt.Branch, m.cfg.RepoRoot); err != nil {
		m.statusMsg = "failed to start agent: " + err.Error()
		return nil
	}
	m.statusMsg = ""
	// Return a cmd that immediately triggers a re-render by sending a change msg
	return func() tea.Msg { return agentChangedMsg{idx: idx} }
}

func (m *Model) killAgent() tea.Cmd {
	if len(m.entries) == 0 {
		return nil
	}
	idx := m.cursor
	e := &m.entries[idx]
	m.statusMsg = "killing…"
	return func() tea.Msg {
		e.agent.Kill()
		return agentChangedMsg{idx: idx}
	}
}

func (m *Model) attachAgent() tea.Cmd {
	e := m.entries[m.cursor]
	name := e.agent.SessionName()
	if name == "" {
		m.statusMsg = "no active session — press r to start"
		return nil
	}
	idx := m.cursor
	return tea.ExecProcess(
		exec.Command("tmux", "attach-session", "-t", name),
		func(err error) tea.Msg {
			if err != nil {
				return errMsg{err}
			}
			return agentChangedMsg{idx: idx}
		},
	)
}

func (m *Model) openDiff() tea.Cmd {
	if len(m.entries) == 0 {
		return nil
	}
	wt := m.entries[m.cursor].wt
	return func() tea.Msg {
		diff, err := worktree.Diff(wt.Path)
		if err != nil {
			return errMsg{err}
		}
		if diff == "" {
			diff = "(no changes)"
		}
		return diffReadyMsg{content: diff}
	}
}

func (m *Model) createWorktree(branch, base string) tea.Cmd {
	m.mode = modeNormal
	m.statusMsg = fmt.Sprintf("creating worktree %q from %q…", branch, base)
	return func() tea.Msg {
		// Use a path alongside the repo root, named after the branch
		safe := strings.ReplaceAll(branch, "/", "-")
		path := filepath.Join(filepath.Dir(m.cfg.RepoRoot), safe)
		if err := worktree.Create(m.cfg.RepoRoot, path, branch, base); err != nil {
			return errMsg{err}
		}
		wts, err := worktree.List(m.cfg.RepoRoot)
		if err != nil {
			return errMsg{err}
		}
		agentMap := map[string]*agent.Agent{}
		for _, e := range m.entries {
			agentMap[e.wt.Path] = e.agent
		}
		entries := make([]entry, 0, len(wts))
		for _, wt := range wts {
			a, ok := agentMap[wt.Path]
			if !ok {
				a = agent.New()
			}
			entries = append(entries, entry{wt: wt, agent: a})
		}
		return worktreesRefreshedMsg{entries: entries}
	}
}

func (m *Model) deleteWorktree() tea.Cmd {
	m.mode = modeNormal
	if len(m.entries) == 0 {
		return nil
	}
	wt := m.entries[m.cursor].wt
	a := m.entries[m.cursor].agent
	return func() tea.Msg {
		a.Kill()
		if err := worktree.Delete(m.cfg.RepoRoot, wt.Path); err != nil {
			return errMsg{err}
		}
		return m.refreshWorktrees()()
	}
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m *Model) View() string {
	if m.width == 0 {
		return "loading…"
	}

	if m.mode == modeDiff {
		return m.renderDiff()
	}

	leftW := 38
	rightW := m.width - leftW - 3
	innerH := m.height - 4 // leave room for status bar + border

	left := m.renderWorktreePanel(leftW, innerH)
	right := m.renderOutputPanel(rightW, innerH)

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)

	status := m.renderStatusBar()

	if m.mode == modeNewWorktree || m.mode == modeNewWorktreeBase || m.mode == modeSendInput {
		return lipgloss.JoinVertical(lipgloss.Left, body, m.renderInputBar(), status)
	}

	return lipgloss.JoinVertical(lipgloss.Left, body, status)
}

func (m *Model) renderWorktreePanel(w, h int) string {
	title := stylePanelTitle.Render("  worktrees")

	var rows []string
	for i, e := range m.entries {
		icon := statusIcon(e.agent.Status())
		iconS := statusStyle(e.agent.Status()).Render(icon)

		branch := e.wt.Branch
		if branch == "" {
			branch = filepath.Base(e.wt.Path)
		}

		base := ""
		if e.wt.BaseBranch != "" {
			base = styleMuted.Render("  " + e.wt.BaseBranch + " ← " + branch)
		}

		statusTxt := statusStyle(e.agent.Status()).Render(e.agent.Status().String())

		line1 := fmt.Sprintf(" %s %s", iconS, styleNormal.Render(branch))
		line2 := base
		line3 := fmt.Sprintf("   %s", statusTxt)

		if e.wt.IsMain {
			line1 = fmt.Sprintf(" %s %s %s", iconS, styleNormal.Render(branch), styleMuted.Render("(main)"))
			line2 = ""
		}

		block := line1
		if line2 != "" {
			block += "\n" + line2
		}
		block += "\n" + line3

		if i == m.cursor {
			block = styleSelected.Width(w - 4).Render(block)
		} else {
			block = lipgloss.NewStyle().Width(w - 4).Render(block)
		}

		rows = append(rows, block)
		if i < len(m.entries)-1 {
			rows = append(rows, styleMuted.Render(strings.Repeat("─", w-4)))
		}
	}

	if len(rows) == 0 {
		rows = []string{styleMuted.Render("  no worktrees found\n  press n to create one")}
	}

	content := strings.Join(rows, "\n")

	panel := stylePanelBorder.
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
		title = fmt.Sprintf(" agent output  %s", styleMuted.Render("← "+branch))
	}

	content := m.outputVP.View()

	panel := stylePanelBorder.
		Width(w).
		Height(h).
		Render(stylePanelTitle.Render(title) + "\n" + content)

	return panel
}

func (m *Model) renderDiff() string {
	title := stylePanelTitle.Render(" diff")
	content := m.diffVP.View()
	panel := stylePanelBorder.
		Width(m.width - 2).
		Height(m.height - 3).
		Render(title + "\n" + content)
	return lipgloss.JoinVertical(lipgloss.Left, panel, m.renderStatusBar())
}

func (m *Model) renderStatusBar() string {
	keys := []string{
		key("n", "new"),
		key("r", "run"),
		key("a", "attach"),
		key("x", "kill"),
		key("d", "diff"),
		key("D", "delete"),
		key("i", "send input"),
		key("R", "refresh"),
		key("q", "quit"),
	}
	bar := strings.Join(keys, "  ")
	if m.statusMsg != "" {
		bar = bar + "   " + lipgloss.NewStyle().Foreground(colorYellow).Render(m.statusMsg)
	}
	return styleStatusBar.Width(m.width).Render(bar)
}

func (m *Model) renderInputBar() string {
	hint := styleMuted.Render(m.inputHint)
	return lipgloss.NewStyle().
		Background(lipgloss.Color("#161b22")).
		PaddingLeft(1).
		Width(m.width).
		Render(hint + m.input.View())
}

func key(k, label string) string {
	return styleKey.Render(k) + " " + styleMuted.Render(label)
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func (m *Model) enterInput(md mode, hint, defaultVal string) {
	m.mode = md
	m.inputHint = hint
	m.input.Reset()
	m.input.SetValue(defaultVal)
	m.input.Focus()
	m.statusMsg = ""
}

func (m *Model) resizePanels() {
	rightW := m.width - 38 - 6
	innerH := m.height - 6
	m.outputVP = viewport.New(rightW, innerH)
	m.syncOutputViewport()
}

func (m *Model) syncOutputViewport() {
	if len(m.entries) == 0 {
		m.outputVP.SetContent(styleMuted.Render("no agent output yet — press r to run"))
		return
	}
	snap := m.entries[m.cursor].agent.Snapshot()
	if snap == "" {
		m.outputVP.SetContent(styleMuted.Render("no output yet — press r to run agent"))
		return
	}
	m.outputVP.SetContent(snap)
	m.outputVP.GotoBottom()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
