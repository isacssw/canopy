package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"canopy/internal/agent"
	"canopy/internal/config"
	"canopy/internal/ui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	if cfg == nil {
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

	// Always re-detect repo root from cwd (allows running from any worktree)
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
	r := bufio.NewReader(os.Stdin)

	fmt.Println("╭─────────────────────────────────────╮")
	fmt.Println("│   canopy — first-run setup          │")
	fmt.Println("╰─────────────────────────────────────╯")
	fmt.Println()

	fmt.Print("Agent command to run in each worktree\n[default: claude]: ")
	cmd, _ := r.ReadString('\n')
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		cmd = "claude"
	}

	cfg := &config.Config{
		AgentCommand: cmd,
	}

	if err := config.Save(cfg); err != nil {
		return nil, err
	}

	fmt.Printf("\n✓ Saved to %s\n\n", config.DefaultConfigPath())
	return cfg, nil
}
