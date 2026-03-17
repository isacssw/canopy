package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/isacssw/canopy/internal/agent"
	"github.com/isacssw/canopy/internal/config"
	"github.com/isacssw/canopy/internal/worktree"
)

// ── Styles ───────────────────────────────────────────────────────────────────

type styles struct {
	panelBorder lipgloss.Style
	panelTitle  lipgloss.Style
	selected    lipgloss.Style
	normal      lipgloss.Style
	muted       lipgloss.Style
	statusBar   lipgloss.Style
	key         lipgloss.Style
}

func newStyles(t Theme) styles {
	return styles{
		panelBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Border),
		panelTitle: lipgloss.NewStyle().
			Foreground(t.Accent).
			Bold(true).
			PaddingLeft(1),
		selected: lipgloss.NewStyle().
			Background(t.SelectedBg).
			Foreground(t.Accent).
			Bold(true),
		normal: lipgloss.NewStyle().Foreground(t.Text),
		muted:  lipgloss.NewStyle().Foreground(t.Muted),
		statusBar: lipgloss.NewStyle().
			Background(t.StatusBarBg).
			Foreground(t.Muted).
			PaddingLeft(1).
			PaddingRight(1),
		key: lipgloss.NewStyle().
			Background(t.KeyBg).
			Foreground(t.Accent).
			PaddingLeft(1).
			PaddingRight(1),
	}
}

func (m *Model) statusStyle(s agent.Status) lipgloss.Style {
	switch s {
	case agent.StatusRunning:
		return lipgloss.NewStyle().Foreground(m.theme.Green)
	case agent.StatusWaiting:
		return lipgloss.NewStyle().Foreground(m.theme.Yellow).Bold(true)
	case agent.StatusDone:
		return lipgloss.NewStyle().Foreground(m.theme.Purple)
	case agent.StatusError:
		return lipgloss.NewStyle().Foreground(m.theme.Red)
	default:
		return lipgloss.NewStyle().Foreground(m.theme.Muted)
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

// ── Soft-delete state ────────────────────────────────────────────────────────

type pendingDeleteState struct {
	wtPath   string
	branch   string
	ag       *agent.Agent
	secsLeft int
}

// ── State entry ─────────────────────────────────────────────────────────────

type entry struct {
	wt         worktree.Worktree
	agent      *agent.Agent
	unread     bool
	prevStatus agent.Status
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
	modePendingDelete
	modeSetupAgent
	modeAgentPicker
	modeHelp
)

// ── Messages ─────────────────────────────────────────────────────────────────

type agentChangedMsg struct{ idx int }
type worktreesRefreshedMsg struct{ entries []entry }
type errMsg struct{ err error }
type diffReadyMsg struct{ content string }
type deleteCountdownMsg struct{ secsLeft int }

// ── Model ────────────────────────────────────────────────────────────────────

type Model struct {
	cfg     *config.Config
	entries []entry
	cursor  int
	mode    mode
	width   int
	height  int

	outputVP viewport.Model
	diffVP   viewport.Model

	input     textinput.Model
	inputHint string

	newBranch string // temp storage during two-step new-worktree flow

	pickerCursor int
	pickerAgents []config.AgentProfile

	statusMsg string
	program   *tea.Program

	theme         Theme
	st            styles
	pendingDelete *pendingDeleteState
}

func New(cfg *config.Config) *Model {
	ti := textinput.New()
	ti.CharLimit = 80

	themeName := ""
	if cfg != nil {
		themeName = cfg.Theme
	}
	t := ThemeByName(themeName)

	return &Model{
		cfg:   cfg,
		input: ti,
		theme: t,
		st:    newStyles(t),
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
		// Wire OnChange for any reconnected agents so UI updates as they poll.
		p := m.program
		for idx := range m.entries {
			if m.entries[idx].agent.SessionName() != "" {
				idx := idx
				m.entries[idx].agent.OnChange = func() {
					if p != nil {
						p.Send(agentChangedMsg{idx: idx})
					}
				}
			}
		}
		m.syncOutputViewport()

	case agentChangedMsg:
		idx := msg.idx
		if idx >= 0 && idx < len(m.entries) {
			e := &m.entries[idx]
			newStatus := e.agent.Status()
			if e.prevStatus != agent.StatusWaiting && newStatus == agent.StatusWaiting {
				if idx != m.cursor {
					e.unread = true
				}
				fmt.Fprint(os.Stderr, "\a") // bell — stderr safe; Bubbletea owns stdout
			}
			e.prevStatus = newStatus
		}
		m.syncOutputViewport()

	case errMsg:
		m.statusMsg = "error: " + msg.err.Error()

	case diffReadyMsg:
		m.diffVP = viewport.New(m.width-4, m.height-6)
		m.diffVP.SetContent(colorDiff(msg.content, m.theme))
		m.mode = modeDiff
		m.statusMsg = "esc to close diff"

	case deleteCountdownMsg:
		if m.pendingDelete == nil {
			return m, nil
		}
		if msg.secsLeft > 0 {
			m.pendingDelete.secsLeft = msg.secsLeft
			next := msg.secsLeft - 1
			return m, tea.Tick(time.Second, func(t time.Time) tea.Msg {
				return deleteCountdownMsg{secsLeft: next}
			})
		}
		// secsLeft == 0: execute the delete
		return m, m.executePendingDelete()

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)
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
				m.entries[m.cursor].unread = false
				m.syncOutputViewport()
			}
		case "down", "j":
			if m.cursor < len(m.entries)-1 {
				m.cursor++
				m.entries[m.cursor].unread = false
				m.syncOutputViewport()
			}
		case "?":
			m.mode = modeHelp
			return m, nil
		case "n":
			m.enterInput(modeNewWorktree, "New branch name (e.g. feat/my-feature): ", "")
			return m, nil
		case "r":
			agents := m.cfg.ResolvedAgents()
			if len(agents) == 1 {
				return m, m.runAgentWithProfile(agents[0])
			}
			m.pickerAgents = agents
			m.pickerCursor = 0
			m.mode = modeAgentPicker
			return m, nil
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
					return m, nil
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
			wt := m.entries[m.cursor].wt
			ag := m.entries[m.cursor].agent
			m.pendingDelete = &pendingDeleteState{
				wtPath:   wt.Path,
				branch:   wt.Branch,
				ag:       ag,
				secsLeft: 5,
			}
			m.mode = modePendingDelete
			return m, tea.Tick(time.Second, func(t time.Time) tea.Msg {
				return deleteCountdownMsg{secsLeft: 4}
			})
		default:
			m.mode = modeNormal
			m.statusMsg = ""
		}

	case modePendingDelete:
		switch msg.String() {
		case "esc", "n", "u":
			m.pendingDelete = nil
			m.mode = modeNormal
			m.statusMsg = "delete cancelled"
		}

	case modeAgentPicker:
		switch msg.String() {
		case "j", "down":
			if m.pickerCursor < len(m.pickerAgents)-1 {
				m.pickerCursor++
			}
		case "k", "up":
			if m.pickerCursor > 0 {
				m.pickerCursor--
			}
		case "enter":
			profile := m.pickerAgents[m.pickerCursor]
			m.mode = modeNormal
			return m, m.runAgentWithProfile(profile)
		case "esc", "q":
			m.mode = modeNormal
		}

	case modeHelp:
		switch msg.String() {
		case "esc", "q", "?":
			m.mode = modeNormal
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
		// Preserve existing agents, unread badges, and prevStatus by path
		type entryState struct {
			agent      *agent.Agent
			unread     bool
			prevStatus agent.Status
		}
		stateMap := map[string]entryState{}
		for _, e := range m.entries {
			stateMap[e.wt.Path] = entryState{e.agent, e.unread, e.prevStatus}
		}
		entries := make([]entry, 0, len(wts))
		for _, wt := range wts {
			st, ok := stateMap[wt.Path]
			if !ok {
				a := agent.New()
				a.Reconnect(wt.Path, wt.Branch, m.cfg.RepoRoot)
				a.SetIdleTimeout(m.cfg.IdleTimeoutSecs)
				st = entryState{agent: a}
			}
			entries = append(entries, entry{wt: wt, agent: st.agent, unread: st.unread, prevStatus: st.prevStatus})
		}
		return worktreesRefreshedMsg{entries: entries}
	}
}

func (m *Model) runAgentWithProfile(profile config.AgentProfile) tea.Cmd {
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
	if err := e.agent.Start(e.wt.Path, profile.Command, e.wt.Branch, m.cfg.RepoRoot); err != nil {
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
				a.SetIdleTimeout(m.cfg.IdleTimeoutSecs)
			}
			entries = append(entries, entry{wt: wt, agent: a})
		}
		return worktreesRefreshedMsg{entries: entries}
	}
}

func (m *Model) executePendingDelete() tea.Cmd {
	pd := m.pendingDelete
	m.pendingDelete = nil
	m.mode = modeNormal
	m.statusMsg = ""
	if pd == nil {
		return nil
	}
	return func() tea.Msg {
		pd.ag.Kill()
		if err := worktree.Delete(m.cfg.RepoRoot, pd.wtPath, pd.branch); err != nil {
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
	rightW := m.width - m.leftPanelWidth() - 6
	innerH := m.height - 6
	m.outputVP = viewport.New(rightW, innerH)
	m.syncOutputViewport()
}

func (m *Model) syncOutputViewport() {
	if len(m.entries) == 0 {
		m.outputVP.SetContent(m.st.muted.Render("no agent output yet — press r to run"))
		return
	}
	snap := m.entries[m.cursor].agent.Snapshot()
	if snap == "" {
		m.outputVP.SetContent(m.st.muted.Render("no output yet — press r to run agent"))
		return
	}
	m.outputVP.SetContent(snap)
	m.outputVP.GotoBottom()
}

func (m *Model) leftPanelWidth() int {
	if m.cfg != nil && m.cfg.LeftPanelWidth >= 20 {
		return m.cfg.LeftPanelWidth
	}
	return 38
}

func (m *Model) statusCounts() string {
	counts := map[agent.Status]int{}
	for _, e := range m.entries {
		counts[e.agent.Status()]++
	}
	var parts []string
	if n := counts[agent.StatusRunning]; n > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(m.theme.Green).Render(fmt.Sprintf("%d running", n)))
	}
	if n := counts[agent.StatusWaiting]; n > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(m.theme.Yellow).Render(fmt.Sprintf("%d waiting", n)))
	}
	if n := counts[agent.StatusDone]; n > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(m.theme.Purple).Render(fmt.Sprintf("%d done", n)))
	}
	if n := counts[agent.StatusError]; n > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(m.theme.Red).Render(fmt.Sprintf("%d error", n)))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, m.st.muted.Render(" · "))
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

// entryAtY maps a terminal row (0-based) in the left panel to an entry index.
// Must stay in sync with renderWorktreePanel block heights.
func (m *Model) entryAtY(y int) int {
	const panelContentStartRow = 2 // border-top (1) + title row (1)
	row := y - panelContentStartRow
	if row < 0 {
		return -1
	}
	for i, e := range m.entries {
		blockH := 2 // line1 + line3
		if !e.wt.IsMain && e.wt.BaseBranch != "" {
			blockH = 3 // line1 + line2 (base) + line3
		}
		if row < blockH {
			return i
		}
		row -= blockH
		if i < len(m.entries)-1 {
			row-- // separator line
		}
		if row < 0 {
			return -1 // clicked on separator
		}
	}
	return -1
}

func (m *Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.mode != modeNormal {
		return m, nil
	}
	leftW := m.leftPanelWidth()
	switch msg.Button {
	case tea.MouseButtonLeft:
		if msg.Action == tea.MouseActionRelease && msg.X < leftW+2 {
			if idx := m.entryAtY(msg.Y); idx >= 0 {
				m.cursor = idx
				m.entries[m.cursor].unread = false
				m.syncOutputViewport()
			}
		}
	case tea.MouseButtonWheelUp:
		m.outputVP.ScrollUp(3)
	case tea.MouseButtonWheelDown:
		m.outputVP.ScrollDown(3)
	}
	return m, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
