package ui

import (
	"fmt"
	"os"
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
	modeAgentPicker
	modeHelp
)

// ── Messages ─────────────────────────────────────────────────────────────────

type agentChangedMsg struct{ wtPath string }
type worktreesRefreshedMsg struct{ entries []entry }
type errMsg struct{ err error }
type diffReadyMsg struct {
	result worktree.DiffResult
	branch string
}

type diffFocus int

const (
	diffFocusFiles diffFocus = iota
	diffFocusPatch
)

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

	// Diff view state
	diffFiles      []worktree.DiffFile
	diffCursor     int
	diffFocus      diffFocus
	diffBranch     string
	diffFileVP     viewport.Model
	diffFileScroll int // scroll offset for file list panel

	input textinput.Model

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
		// Wire callbacks so UI updates as agents poll.
		for idx := range m.entries {
			m.bindAgentOnChange(m.entries[idx].wt.Path, m.entries[idx].agent)
		}
		m.syncOutputViewport()

	case agentChangedMsg:
		idx := m.indexByPath(msg.wtPath)
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
		m.diffFiles = msg.result.Files
		m.diffCursor = 0
		m.diffFocus = diffFocusFiles
		m.diffBranch = msg.branch
		m.diffFileScroll = 0
		m.mode = modeDiff
		m.syncDiffViewport()

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
	if m.mode == modeNormal {
		m.outputVP, cmd = m.outputVP.Update(msg)
	} else if m.mode == modeDiff {
		m.diffFileVP, cmd = m.diffFileVP.Update(msg)
	}
	if m.mode == modeNewWorktree || m.mode == modeNewWorktreeBase ||
		m.mode == modeSendInput {
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
		case "tab":
			if m.diffFocus == diffFocusFiles {
				m.diffFocus = diffFocusPatch
			} else {
				m.diffFocus = diffFocusFiles
			}
		case "j", "down":
			if m.diffFocus == diffFocusFiles {
				if m.diffCursor < len(m.diffFiles)-1 {
					m.diffCursor++
					m.syncDiffViewport()
				}
			} else {
				m.diffFileVP.ScrollDown(1)
			}
		case "k", "up":
			if m.diffFocus == diffFocusFiles {
				if m.diffCursor > 0 {
					m.diffCursor--
					m.syncDiffViewport()
				}
			} else {
				m.diffFileVP.ScrollUp(1)
			}
		case "J":
			m.diffFileVP.ScrollDown(3)
		case "K":
			m.diffFileVP.ScrollUp(3)
		case "g":
			m.diffFileVP.GotoTop()
		case "G":
			m.diffFileVP.GotoBottom()
		case "enter":
			if m.diffFocus == diffFocusFiles {
				m.diffFocus = diffFocusPatch
			}
		case "e":
			if len(m.diffFiles) > 0 {
				return m, m.openInEditor()
			}
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
		m.mode == modeSendInput {
		m.input, cmd = m.input.Update(msg)
	}
	return m, cmd
}

// ── View ─────────────────────────────────────────────────────────────────────

// ── Helpers ──────────────────────────────────────────────────────────────────

func (m *Model) enterInput(md mode, hint, defaultVal string) {
	m.mode = md
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
	if m.mode == modeDiff {
		m.syncDiffViewport()
	}
}

func (m *Model) syncDiffViewport() {
	// Right panel dimensions: total width minus left file list panel minus gaps
	leftW := m.diffFileListWidth()
	rightW := m.width - leftW - 5 // borders + gap
	innerH := m.height - 6        // borders + status bar + title
	if rightW < 10 {
		rightW = 10
	}
	if innerH < 1 {
		innerH = 1
	}

	m.diffFileVP = viewport.New(rightW, innerH)

	if len(m.diffFiles) == 0 {
		m.diffFileVP.SetContent(m.st.muted.Render("(no changes)"))
		return
	}

	if m.diffCursor >= len(m.diffFiles) {
		m.diffCursor = len(m.diffFiles) - 1
	}

	f := m.diffFiles[m.diffCursor]
	var content string
	if f.IsBinary && f.Patch == "" {
		content = m.st.muted.Render("Binary file changed")
	} else if f.Patch == "" {
		content = m.st.muted.Render("(no diff content)")
	} else {
		content = colorDiff(f.Patch, m.theme)
	}
	m.diffFileVP.SetContent(content)
	m.diffFileVP.GotoTop()
}

func (m *Model) diffFileListWidth() int {
	w := m.width * 2 / 5
	if w < 30 {
		w = 30
	}
	if w > 50 {
		w = 50
	}
	return w
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

func (m *Model) bindAgentOnChange(wtPath string, ag *agent.Agent) {
	ag.SetOnChange(func() {
		if m.program != nil {
			m.program.Send(agentChangedMsg{wtPath: wtPath})
		}
	})
}

func (m *Model) indexByPath(path string) int {
	for i := range m.entries {
		if m.entries[i].wt.Path == path {
			return i
		}
	}
	return -1
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
	if m.mode != modeNormal && m.mode != modePendingDelete {
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
