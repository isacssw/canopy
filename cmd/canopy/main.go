package main

import (
	"fmt"
	"os"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/isacssw/canopy/internal/agent"
	"github.com/isacssw/canopy/internal/config"
	"github.com/isacssw/canopy/internal/status"
	"github.com/isacssw/canopy/internal/ui"
	"github.com/isacssw/canopy/internal/worktree"
)

var version = "dev"

func versionString() string {
	v := version
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" && len(s.Value) >= 7 {
				v += " (" + s.Value[:7] + ")"
				break
			}
		}
	}
	return v
}

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println(versionString())
		return
	}

	if len(os.Args) >= 2 && os.Args[1] == "status" {
		runStatus()
		return
	}

	if len(os.Args) == 2 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println("canopy - manage Claude Code agents across git worktrees")
		fmt.Println()
		fmt.Println("Usage: canopy [--version] [--help] [status]")
		fmt.Println()
		fmt.Println("Subcommands:")
		fmt.Println("  status     output worktree and agent status as JSON")
		fmt.Println()
		fmt.Println("Keybindings:")
		fmt.Println("  Navigation")
		fmt.Println("    j / ↓    move down")
		fmt.Println("    k / ↑    move up")
		fmt.Println()
		fmt.Println("  Worktrees")
		fmt.Println("    n        new worktree")
		fmt.Println("    D        delete worktree")
		fmt.Println("    R        refresh list")
		fmt.Println()
		fmt.Println("  Agents")
		fmt.Println("    r        run agent")
		fmt.Println("    a        attach to session")
		fmt.Println("    x        kill agent")
		fmt.Println("    i        send input")
		fmt.Println()
		fmt.Println("  Diff view")
		fmt.Println("    d        open diff view")
		fmt.Println("    e        open file in $EDITOR (nvim-aware)")
		fmt.Println()
		fmt.Println("  Other")
		fmt.Println("    ?        toggle help")
		fmt.Println("    q        quit")
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	if cfg == nil {
		ui.PrintWelcome()
		cfg, err = runSetup()
		if err != nil {
			fmt.Fprintf(os.Stderr, "setup failed: %v\n", err)
			os.Exit(1)
		}
	}

	if err := agent.CheckTmux(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Always re-detect repo root from cwd (allows running from any worktree).
	root, err := config.DetectRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "not in a git repo: %v\n", err)
		os.Exit(1)
	}
	cfg.RepoRoot = root

	if wt, ok := nestedAgentSession(root); ok {
		branch := wt.Branch
		if branch == "" {
			branch = wt.Path
		}
		fmt.Fprintf(os.Stderr, "error: canopy is running inside the agent tmux session for %q; detach first or start canopy from another terminal\n", branch)
		os.Exit(1)
	}

	model := ui.New(cfg)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	model.SetProgram(p)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runStatus() {
	if err := agent.CheckTmux(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	root, err := config.DetectRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "not in a git repo: %v\n", err)
		os.Exit(1)
	}

	cfg := &config.Config{RepoRoot: root}
	if err := status.Run(os.Stdout, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runSetup() (*config.Config, error) {
	m := ui.NewSetupModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return nil, err
	}
	cfg := m.Result()
	if cfg == nil {
		return nil, fmt.Errorf("setup cancelled")
	}
	return cfg, nil
}

func nestedAgentSession(repoRoot string) (worktree.Worktree, bool) {
	current := agent.CurrentTmuxSessionName()
	if current == "" {
		return worktree.Worktree{}, false
	}

	wts, err := worktree.List(repoRoot)
	if err != nil {
		return worktree.Worktree{}, false
	}

	return matchingWorktreeForSession(repoRoot, current, wts)
}

func matchingWorktreeForSession(repoRoot, sessionName string, wts []worktree.Worktree) (worktree.Worktree, bool) {
	for _, wt := range wts {
		if agent.SessionNameFor(repoRoot, wt.Branch, wt.Path) == sessionName {
			return wt, true
		}
	}

	return worktree.Worktree{}, false
}
