package config

import (
	"reflect"
	"testing"
)

func TestResolvedAgentsDefaultWhenNilConfig(t *testing.T) {
	var cfg *Config
	got := cfg.ResolvedAgents()
	want := []AgentProfile{{Name: "claude", Command: "claude"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolvedAgents() = %#v, want %#v", got, want)
	}
}

func TestResolvedAgentsNormalizesProfiles(t *testing.T) {
	cfg := &Config{
		Agents: []AgentProfile{
			{Name: " ", Command: " codex --model gpt-5.4 "},
			{Name: "broken", Command: "   "},
		},
	}

	got := cfg.ResolvedAgents()
	want := []AgentProfile{
		{Name: "codex", Command: "codex --model gpt-5.4"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolvedAgents() = %#v, want %#v", got, want)
	}
}

func TestResolvedAgentsFallsBackToLegacyCommand(t *testing.T) {
	cfg := &Config{AgentCommand: "  npx @openai/codex  "}
	got := cfg.ResolvedAgents()
	want := []AgentProfile{
		{Name: "npx", Command: "npx @openai/codex"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolvedAgents() = %#v, want %#v", got, want)
	}
}

func TestNormalizeClampsValues(t *testing.T) {
	cfg := &Config{
		LeftPanelWidth:  -12,
		IdleTimeoutSecs: -3,
		Theme:           "  github-dark  ",
		AgentCommand:    "  claude  ",
		Agents: []AgentProfile{
			{Name: "", Command: "  claude  "},
			{Name: "invalid", Command: ""},
		},
	}

	cfg.Normalize()

	if cfg.LeftPanelWidth != 0 {
		t.Fatalf("LeftPanelWidth = %d, want 0", cfg.LeftPanelWidth)
	}
	if cfg.IdleTimeoutSecs != 0 {
		t.Fatalf("IdleTimeoutSecs = %d, want 0", cfg.IdleTimeoutSecs)
	}
	if cfg.Theme != "github-dark" {
		t.Fatalf("Theme = %q, want github-dark", cfg.Theme)
	}
	if cfg.AgentCommand != "claude" {
		t.Fatalf("AgentCommand = %q, want claude", cfg.AgentCommand)
	}
	if !reflect.DeepEqual(cfg.Agents, []AgentProfile{{Name: "claude", Command: "claude"}}) {
		t.Fatalf("Agents normalized incorrectly: %#v", cfg.Agents)
	}
}
