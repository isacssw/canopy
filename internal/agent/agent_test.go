package agent

import "testing"

func TestDetectStatusPaneDead(t *testing.T) {
	if got := detectStatus("", true, 0, agentFlavorUnknown); got != StatusDone {
		t.Fatalf("expected done, got %v", got)
	}
	if got := detectStatus("", true, 7, agentFlavorUnknown); got != StatusError {
		t.Fatalf("expected error, got %v", got)
	}
}

func TestDetectStatusClaudeAvoidsTrailingAngleFalsePositive(t *testing.T) {
	snapshot := "running checks\nredirect >"
	got := detectStatus(snapshot, false, 0, agentFlavorClaude)
	if got != StatusRunning {
		t.Fatalf("expected running, got %v", got)
	}
}

func TestDetectStatusClaudeWaitingPrompt(t *testing.T) {
	snapshot := "working\n? Do you want to proceed?"
	got := detectStatus(snapshot, false, 0, agentFlavorClaude)
	if got != StatusWaiting {
		t.Fatalf("expected waiting, got %v", got)
	}
}

func TestDetectStatusCodexRunning(t *testing.T) {
	snapshot := "• Working (0s • esc to interrupt)\n\n› Ask Codex to do anything"
	got := detectStatus(snapshot, false, 0, agentFlavorCodex)
	if got != StatusRunning {
		t.Fatalf("expected running, got %v", got)
	}
}

func TestDetectStatusCodexWaitingApproval(t *testing.T) {
	snapshot := "Would you like to run the following command?\n" +
		"› 1. Yes, proceed (y)\n" +
		"  2. No, and tell Codex what to do differently (esc)\n" +
		"Press enter to confirm or esc to cancel"
	got := detectStatus(snapshot, false, 0, agentFlavorCodex)
	if got != StatusWaiting {
		t.Fatalf("expected waiting, got %v", got)
	}
}

func TestDetectStatusCodexWaitingStatusLine(t *testing.T) {
	snapshot := "• Waiting for background terminal (0s • esc to interrupt)\n  └ cargo test"
	got := detectStatus(snapshot, false, 0, agentFlavorCodex)
	if got != StatusWaiting {
		t.Fatalf("expected waiting, got %v", got)
	}
}

func TestDetectAgentFlavor(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    agentFlavor
	}{
		{name: "claude", command: "claude", want: agentFlavorClaude},
		{name: "codex", command: "codex", want: agentFlavorCodex},
		{name: "quoted codex", command: `"codex" --model gpt-5.4`, want: agentFlavorCodex},
		{name: "npx codex", command: "npx @openai/codex", want: agentFlavorCodex},
		{name: "unknown", command: "aider", want: agentFlavorUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detectAgentFlavor(tt.command); got != tt.want {
				t.Fatalf("detectAgentFlavor(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestStabilizeInteractiveStatus(t *testing.T) {
	pending := StatusIdle
	count := 0

	if got := stabilizeInteractiveStatus(StatusRunning, StatusWaiting, &pending, &count); got != StatusRunning {
		t.Fatalf("first running->waiting transition should be held, got %v", got)
	}
	if got := stabilizeInteractiveStatus(StatusRunning, StatusWaiting, &pending, &count); got != StatusWaiting {
		t.Fatalf("second running->waiting transition should switch, got %v", got)
	}

	if got := stabilizeInteractiveStatus(StatusWaiting, StatusRunning, &pending, &count); got != StatusWaiting {
		t.Fatalf("first waiting->running transition should be held, got %v", got)
	}
	if got := stabilizeInteractiveStatus(StatusWaiting, StatusRunning, &pending, &count); got != StatusRunning {
		t.Fatalf("second waiting->running transition should switch, got %v", got)
	}

	if got := stabilizeInteractiveStatus(StatusRunning, StatusDone, &pending, &count); got != StatusDone {
		t.Fatalf("terminal status should not be delayed, got %v", got)
	}
}
