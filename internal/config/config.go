package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type AgentProfile struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}

type Config struct {
	AgentCommand    string         `json:"agent_command"`    // legacy, kept for compat
	Agents          []AgentProfile `json:"agents,omitempty"` // new
	RepoRoot        string         `json:"repo_root"`
	LeftPanelWidth  int            `json:"left_panel_width,omitempty"`  // 0 = default 38
	Theme           string         `json:"theme,omitempty"`             // "", "github-dark", "nord", "catppuccin", "light"
	IdleTimeoutSecs int            `json:"idle_timeout_secs,omitempty"` // 0 = disabled
}

// ResolvedAgents returns the list of agent profiles to use.
// If the new Agents field is set it is returned as-is; otherwise the legacy
// AgentCommand field is wrapped in a single-element slice (defaulting to
// "claude" when both are empty).
func (c *Config) ResolvedAgents() []AgentProfile {
	if len(c.Agents) > 0 {
		return c.Agents
	}
	cmd := c.AgentCommand
	if cmd == "" {
		cmd = "claude"
	}
	return []AgentProfile{{Name: cmd, Command: cmd}}
}

func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "canopy", "config.json")
}

func Load() (*Config, error) {
	path := DefaultConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // not yet configured
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	path := DefaultConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func DetectRepoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not inside a git repository")
	}
	return strings.TrimSpace(string(out)), nil
}
