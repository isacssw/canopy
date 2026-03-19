package ui

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/isacssw/canopy/internal/agent"
	"github.com/isacssw/canopy/internal/config"
	"github.com/isacssw/canopy/internal/worktree"
)

func (m *Model) refreshWorktrees() tea.Cmd {
	return func() tea.Msg {
		wts, err := worktree.List(m.cfg.RepoRoot)
		if err != nil {
			return errMsg{err}
		}

		// Preserve existing agents, unread badges, and prevStatus by path.
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
	wtPath := e.wt.Path
	if e.agent.Status() == agent.StatusRunning || e.agent.Status() == agent.StatusWaiting {
		m.statusMsg = "agent already running — press x to kill"
		return nil
	}

	e.agent.Reset()
	m.bindAgentOnChange(wtPath, e.agent)
	if err := e.agent.Start(e.wt.Path, profile.Command, e.wt.Branch, m.cfg.RepoRoot); err != nil {
		m.statusMsg = "failed to start agent: " + err.Error()
		return nil
	}
	m.statusMsg = ""

	// Immediately re-render with updated state.
	return func() tea.Msg { return agentChangedMsg{wtPath: wtPath} }
}

func (m *Model) killAgent() tea.Cmd {
	if len(m.entries) == 0 {
		return nil
	}
	idx := m.cursor
	e := &m.entries[idx]
	wtPath := e.wt.Path
	m.statusMsg = "killing…"
	return func() tea.Msg {
		e.agent.Kill()
		return agentChangedMsg{wtPath: wtPath}
	}
}

func (m *Model) attachAgent() tea.Cmd {
	e := m.entries[m.cursor]
	name := e.agent.SessionName()
	if name == "" {
		m.statusMsg = "no active session — press r to start"
		return nil
	}
	wtPath := e.wt.Path
	return tea.ExecProcess(
		exec.Command("tmux", "attach-session", "-t", name),
		func(err error) tea.Msg {
			if err != nil {
				return errMsg{err}
			}
			return agentChangedMsg{wtPath: wtPath}
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
		// Use a path alongside the repo root, named after the branch.
		safe := strings.ReplaceAll(branch, "/", "-")
		path := filepath.Join(filepath.Dir(m.cfg.RepoRoot), safe)
		if err := worktree.Create(m.cfg.RepoRoot, path, branch, base); err != nil {
			return errMsg{err}
		}
		return m.refreshWorktrees()()
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
