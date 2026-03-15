package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/isacssw/canopy/internal/agent"
	"github.com/isacssw/canopy/internal/config"
	"github.com/isacssw/canopy/internal/ui"
)

func main() {
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
