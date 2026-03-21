package agent

import (
	"crypto/sha1"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/isacssw/canopy/internal/cmdline"
)

type Status int

const (
	StatusIdle Status = iota
	StatusRunning
	StatusWaiting // agent is asking for input
	StatusDone
	StatusError
)

func (s Status) String() string {
	switch s {
	case StatusIdle:
		return "idle"
	case StatusRunning:
		return "running"
	case StatusWaiting:
		return "waiting"
	case StatusDone:
		return "done"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

// CheckTmux verifies tmux is installed and available on PATH.
func CheckTmux() error {
	if _, err := exec.LookPath("tmux"); err != nil {
		return fmt.Errorf("tmux not found: install tmux to use canopy")
	}
	return nil
}

var sessionNameRe = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

type agentFlavor int

const (
	agentFlavorUnknown agentFlavor = iota
	agentFlavorClaude
	agentFlavorCodex
)

var (
	codexChoiceLineRe  = regexp.MustCompile(`^[›>]\s*\d+\.\s+`)
	codexConfirmLineRe = regexp.MustCompile(`(?i)press enter to confirm or esc to`)
	codexSubmitLineRe  = regexp.MustCompile(`(?i)\benter to submit\b.*\besc to (interrupt|cancel)\b`)
	claudeChoiceLineRe = regexp.MustCompile(`^[❯›]\s+`)
	claudeYesNoLineRe  = regexp.MustCompile(`(?i)\byes\b.*\b(no|always)\b`)
	claudeYnPromptLine = regexp.MustCompile(`(?i)\b(y/n|y/N|Y/n|yes/no)\b`)
)

// SessionNameFor derives a deterministic, tmux-safe session name.
// A repo-root hash prefix prevents collisions across repos with the same branch name.
// Exported for use by the status subcommand and nvim plugin.
func SessionNameFor(repoRoot, branch, worktreePath string) string {
	h := sha1.Sum([]byte(repoRoot))
	repoHash := fmt.Sprintf("%x", h[:4])
	s := branch
	if s == "" {
		s = filepath.Base(worktreePath)
	}
	s = sessionNameRe.ReplaceAllString(s, "_")
	name := fmt.Sprintf("canopy_%s_%s", repoHash, s)
	if len(name) > 64 {
		name = name[:64]
	}
	return name
}

// Agent manages an AI coding process via a dedicated tmux session.
type Agent struct {
	mu                 sync.Mutex
	sessionName        string
	status             Status
	flavor             agentFlavor
	snapshot           string
	onChange           func()
	stopPoll           chan struct{}
	lastSnapshotChange time.Time
	idleTimeoutSecs    int
	pendingStatus      Status
	pendingStatusCount int
}

const (
	pollIntervalFast   = 500 * time.Millisecond
	pollIntervalMedium = time.Second
	pollIntervalSlow   = 2 * time.Second
)

// SetIdleTimeout configures the agent-agnostic idle timeout. When the tmux
// pane snapshot has not changed for secs seconds, a Running status is
// overridden to Waiting. Set to 0 to disable.
func (a *Agent) SetIdleTimeout(secs int) {
	a.mu.Lock()
	a.idleTimeoutSecs = secs
	a.mu.Unlock()
}

func New() *Agent {
	return &Agent{status: StatusIdle}
}

// SetOnChange registers a callback fired when status or snapshot changes.
func (a *Agent) SetOnChange(cb func()) {
	a.mu.Lock()
	a.onChange = cb
	a.mu.Unlock()
}

func tmuxRun(args ...string) error {
	return exec.Command("tmux", args...).Run()
}

func tmuxOutput(args ...string) ([]byte, error) {
	return exec.Command("tmux", args...).Output()
}

func tmuxCombinedOutput(args ...string) ([]byte, error) {
	return exec.Command("tmux", args...).CombinedOutput()
}

// Reconnect checks whether a tmux session for this worktree already exists
// (e.g. from a previous canopy instance) and resumes polling if so.
// Returns true if an existing session was found.
func (a *Agent) Reconnect(workdir, branch, repoRoot string) bool {
	name := SessionNameFor(repoRoot, branch, workdir)
	if err := tmuxRun("has-session", "-t", name); err != nil {
		return false
	}
	// Query pane state before taking the lock (name is a local var, no shared state)
	deadOut, _ := tmuxOutput("display-message", "-t", name, "-p", "#{pane_dead},#{pane_dead_status},#{pane_current_command}")
	deadStr := strings.TrimSpace(string(deadOut))

	initialStatus := StatusRunning
	flavor := agentFlavorUnknown
	if parts := strings.SplitN(deadStr, ",", 3); len(parts) >= 2 {
		if parts[0] == "1" {
			code, _ := strconv.Atoi(parts[1])
			if code == 0 {
				initialStatus = StatusDone
			} else {
				initialStatus = StatusError
			}
		}
		if len(parts) == 3 {
			flavor = detectAgentFlavor(parts[2])
		}
	}

	a.mu.Lock()
	a.sessionName = name
	a.status = initialStatus
	a.flavor = flavor
	a.stopPoll = make(chan struct{})
	a.pendingStatus = StatusIdle
	a.pendingStatusCount = 0
	stop := a.stopPoll
	a.mu.Unlock()

	_ = tmuxRun("set-option", "-t", name, "mouse", "on")

	if initialStatus == StatusRunning {
		go a.pollLoop(stop)
	}
	return true
}

func (a *Agent) Status() Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.status
}

// Snapshot returns the most recent trimmed capture-pane output.
func (a *Agent) Snapshot() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.snapshot
}

// SessionName returns the active tmux session name, or empty string when idle.
func (a *Agent) SessionName() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sessionName
}

// Start creates a tmux session and launches the agent command inside it.
func (a *Agent) Start(workdir, command, branch, repoRoot string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.status == StatusRunning || a.status == StatusWaiting {
		return nil
	}

	name := SessionNameFor(repoRoot, branch, workdir)

	// Kill any stale orphan session from a previous crash
	if err := tmuxRun("has-session", "-t", name); err == nil {
		_ = tmuxRun("kill-session", "-t", name)
	}

	out, err := tmuxCombinedOutput("new-session", "-d", "-s", name, "-c", workdir, "-x", "220", "-y", "50")
	if err != nil {
		return fmt.Errorf("tmux new-session: %w — %s", err, strings.TrimSpace(string(out)))
	}

	// Keep pane alive after the process exits so we can read the exit status
	_ = tmuxRun("set-option", "-t", name, "remain-on-exit", "on")
	_ = tmuxRun("set-option", "-t", name, "mouse", "on")
	_ = tmuxRun("set-option", "-t", name, "history-limit", "5000")

	launchCmd := strings.TrimSpace(command)
	if launchCmd == "" {
		launchCmd = "claude"
	}

	flavor := detectAgentFlavor(launchCmd)
	if err := tmuxRun("send-keys", "-t", name, "-l", launchCmd); err != nil {
		_ = tmuxRun("kill-session", "-t", name)
		return fmt.Errorf("tmux send-keys: %w", err)
	}
	if err := tmuxRun("send-keys", "-t", name, "Enter"); err != nil {
		_ = tmuxRun("kill-session", "-t", name)
		return fmt.Errorf("tmux send-keys enter: %w", err)
	}

	a.sessionName = name
	a.status = StatusRunning
	a.flavor = flavor
	a.snapshot = ""
	a.lastSnapshotChange = time.Time{}
	a.stopPoll = make(chan struct{})
	a.pendingStatus = StatusIdle
	a.pendingStatusCount = 0
	go a.pollLoop(a.stopPoll)

	return nil
}

// Send writes text to the agent's tmux session as literal keystrokes.
func (a *Agent) Send(text string) {
	a.mu.Lock()
	name := a.sessionName
	a.mu.Unlock()
	if name == "" {
		return
	}
	// -l sends literal characters, preventing tmux from interpreting key names
	_ = tmuxRun("send-keys", "-t", name, "-l", text)
	_ = tmuxRun("send-keys", "-t", name, "Enter")
}

// Kill terminates the tmux session and stops polling.
func (a *Agent) Kill() {
	a.mu.Lock()
	name := a.sessionName
	a.mu.Unlock()
	if name == "" {
		return
	}

	a.stopPolling()
	_ = tmuxRun("kill-session", "-t", name)

	a.mu.Lock()
	a.status = StatusIdle
	a.sessionName = ""
	a.flavor = agentFlavorUnknown
	a.pendingStatus = StatusIdle
	a.pendingStatusCount = 0
	cb := a.onChange
	a.mu.Unlock()

	if cb != nil {
		cb()
	}
}

// Reset clears state and kills any existing session.
func (a *Agent) Reset() {
	a.mu.Lock()
	name := a.sessionName
	a.mu.Unlock()

	a.stopPolling()
	if name != "" {
		_ = tmuxRun("kill-session", "-t", name)
	}

	a.mu.Lock()
	a.status = StatusIdle
	a.snapshot = ""
	a.sessionName = ""
	a.flavor = agentFlavorUnknown
	a.pendingStatus = StatusIdle
	a.pendingStatusCount = 0
	a.mu.Unlock()
}

// stopPolling closes the poll channel exactly once; safe to call concurrently.
func (a *Agent) stopPolling() {
	a.mu.Lock()
	ch := a.stopPoll
	a.stopPoll = nil
	a.mu.Unlock()
	if ch != nil {
		close(ch)
	}
}

func (a *Agent) pollLoop(stop <-chan struct{}) {
	interval := pollIntervalFast
	idleTicks := 0
	timer := time.NewTimer(interval)
	defer timer.Stop()
	for {
		select {
		case <-stop:
			return
		case <-timer.C:
			keepPolling, changed, status := a.tick()
			if !keepPolling {
				return
			}
			if changed {
				idleTicks = 0
			} else {
				idleTicks++
			}

			switch {
			case status == StatusDone || status == StatusError:
				interval = pollIntervalMedium
			case idleTicks >= 20:
				interval = pollIntervalSlow
			case idleTicks >= 6:
				interval = pollIntervalMedium
			default:
				interval = pollIntervalFast
			}
			timer.Reset(interval)
		}
	}
}

// tick captures pane state and updates status.
// Returns keepPolling, changed, status.
func (a *Agent) tick() (bool, bool, Status) {
	a.mu.Lock()
	name := a.sessionName
	a.mu.Unlock()
	if name == "" {
		return false, false, StatusIdle
	}

	// Read pane_dead and exit status set by remain-on-exit
	deadOut, err := tmuxOutput("display-message", "-t", name, "-p", "#{pane_dead},#{pane_dead_status},#{pane_current_command}")
	if err != nil {
		// Session disappeared without remain-on-exit — treat as done.
		a.mu.Lock()
		if a.sessionName == name { // guard against concurrent Kill()
			a.status = StatusDone
			a.sessionName = ""
		}
		cb := a.onChange
		a.mu.Unlock()
		if cb != nil {
			cb()
		}
		return false, true, StatusDone
	}
	deadStr := strings.TrimSpace(string(deadOut))

	paneDead := false
	paneDeadStatus := 0
	paneCurrentCommand := ""
	if parts := strings.SplitN(deadStr, ",", 3); len(parts) >= 2 {
		paneDead = parts[0] == "1"
		paneDeadStatus, _ = strconv.Atoi(parts[1])
		if len(parts) == 3 {
			paneCurrentCommand = parts[2]
		}
	}

	snapshot := ""
	if !paneDead {
		// Capture visible screen plus 200 lines of scrollback (with ANSI color codes)
		snapOut, _ := tmuxOutput("capture-pane", "-t", name, "-p", "-e", "-S", "-200")
		snapshot = trimSnapshot(string(snapOut))
	}

	a.mu.Lock()
	if a.sessionName != name {
		// Kill() was called while we were polling — don't overwrite its state
		a.mu.Unlock()
		return false, false, StatusIdle
	}
	if flavor := detectAgentFlavor(paneCurrentCommand); flavor != agentFlavorUnknown {
		a.flavor = flavor
	}
	newStatus := detectStatus("", paneDead, paneDeadStatus, a.flavor)
	if !paneDead {
		plainSnapshot := ansi.Strip(snapshot)
		newStatus = detectStatus(plainSnapshot, false, 0, a.flavor)
	}
	if !paneDead && a.snapshot != snapshot {
		a.lastSnapshotChange = time.Now()
	}
	if newStatus == StatusRunning &&
		a.idleTimeoutSecs > 0 &&
		!a.lastSnapshotChange.IsZero() &&
		time.Since(a.lastSnapshotChange) >= time.Duration(a.idleTimeoutSecs)*time.Second {
		newStatus = StatusWaiting
	}
	newStatus = stabilizeInteractiveStatus(
		a.status,
		newStatus,
		&a.pendingStatus,
		&a.pendingStatusCount,
	)
	changed := a.status != newStatus
	if !paneDead {
		changed = changed || a.snapshot != snapshot
		a.snapshot = snapshot
	}
	a.status = newStatus
	cb := a.onChange
	a.mu.Unlock()

	if changed && cb != nil {
		cb()
	}

	if newStatus == StatusDone || newStatus == StatusError {
		a.stopPolling()
		return false, changed, newStatus
	}
	return true, changed, newStatus
}

// detectStatus infers agent state from the tmux pane snapshot.
func detectStatus(snapshot string, paneDead bool, paneDeadStatus int, flavor agentFlavor) Status {
	if paneDead {
		if paneDeadStatus == 0 {
			return StatusDone
		}
		return StatusError
	}

	lines := snapshotTail(snapshot, 12)
	switch flavor {
	case agentFlavorClaude:
		if hasClaudeWaitingPrompt(lines) {
			return StatusWaiting
		}
	case agentFlavorCodex:
		if hasCodexWaitingPrompt(lines) {
			return StatusWaiting
		}
	default:
		if hasClaudeWaitingPrompt(lines) || hasCodexWaitingPrompt(lines) {
			return StatusWaiting
		}
	}

	return StatusRunning
}

func snapshotTail(snapshot string, maxLines int) []string {
	lines := strings.Split(snapshot, "\n")
	if len(lines) <= maxLines {
		return lines
	}
	return lines[len(lines)-maxLines:]
}

func hasCodexWaitingPrompt(lines []string) bool {
	hasChoice := false
	hasFooter := false
	hasPromptHeading := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		l := strings.ToLower(line)
		if strings.HasPrefix(l, "would you like to ") {
			hasPromptHeading = true
		}
		if strings.HasPrefix(l, "• waiting") {
			return true
		}
		if codexConfirmLineRe.MatchString(line) || codexSubmitLineRe.MatchString(line) {
			hasFooter = true
		}
		if codexChoiceLineRe.MatchString(line) {
			hasChoice = true
		}
	}

	return hasPromptHeading || hasFooter || (hasChoice && hasFooter)
}

// isClaudeWaiting detects Claude Code's specific input prompt patterns.
// These appear at the bottom of the pane when Claude needs user input.
func hasClaudeWaitingPrompt(lines []string) bool {
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Claude Code's ? prompt for confirmations
		// e.g. "? Do you want to proceed?"
		if strings.HasPrefix(line, "? ") {
			return true
		}

		// ❯/› marks selections or input cursor in Claude Code prompts.
		if claudeChoiceLineRe.MatchString(line) {
			return true
		}

		// Yes/No/Always option rows rendered for tool-use confirmations.
		if claudeYesNoLineRe.MatchString(line) || claudeYnPromptLine.MatchString(line) {
			return true
		}

		// Bare input cursor prompt.
		if line == ">" {
			return true
		}

		// Pager-style continue prompt.
		if strings.Contains(line, "Press Enter to continue") {
			return true
		}
	}

	return false
}

func stabilizeInteractiveStatus(current, detected Status, pending *Status, count *int) Status {
	if current == detected {
		*pending = StatusIdle
		*count = 0
		return detected
	}
	if !isInteractiveStatus(current) || !isInteractiveStatus(detected) {
		*pending = StatusIdle
		*count = 0
		return detected
	}

	if *pending != detected {
		*pending = detected
		*count = 1
		return current
	}

	*count++
	if *count < 2 {
		return current
	}
	*pending = StatusIdle
	*count = 0
	return detected
}

func isInteractiveStatus(s Status) bool {
	return s == StatusRunning || s == StatusWaiting
}

func detectAgentFlavor(command string) agentFlavor {
	command = strings.TrimSpace(command)
	if command == "" {
		return agentFlavorUnknown
	}

	fields := cmdline.Fields(command)
	candidates := make([]string, 0, 2)
	if len(fields) > 0 {
		candidates = append(candidates, fields[0])
	}
	if len(fields) > 1 && (strings.EqualFold(fields[0], "npx") || strings.EqualFold(fields[0], "pnpm")) {
		candidates = append(candidates, fields[1])
	}

	for _, c := range candidates {
		base := strings.ToLower(filepath.Base(c))
		base = strings.TrimSuffix(base, ".exe")
		switch base {
		case "claude", "claude-code":
			return agentFlavorClaude
		case "codex", "codex-cli":
			return agentFlavorCodex
		}
	}

	lower := strings.ToLower(command)
	switch {
	case strings.Contains(lower, "claude"):
		return agentFlavorClaude
	case strings.Contains(lower, "codex"):
		return agentFlavorCodex
	default:
		return agentFlavorUnknown
	}
}

// trimSnapshot strips trailing whitespace per line and trailing blank lines.
// tmux capture-pane pads every line to the full pane width.
func trimSnapshot(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// ProbeSession checks whether a tmux session exists and determines its status
// without instantiating a full Agent. Returns the status string and whether the
// session is active. This is used by the `canopy status --json` subcommand.
func ProbeSession(sessionName string) (status string, active bool) {
	if err := tmuxRun("has-session", "-t", sessionName); err != nil {
		return "idle", false
	}

	deadOut, err := tmuxOutput("display-message", "-t", sessionName, "-p", "#{pane_dead},#{pane_dead_status},#{pane_current_command}")
	if err != nil {
		return "idle", false
	}
	deadStr := strings.TrimSpace(string(deadOut))

	paneDead := false
	paneDeadStatus := 0
	flavor := agentFlavorUnknown

	if parts := strings.SplitN(deadStr, ",", 3); len(parts) >= 2 {
		paneDead = parts[0] == "1"
		paneDeadStatus, _ = strconv.Atoi(parts[1])
		if len(parts) == 3 {
			flavor = detectAgentFlavor(parts[2])
		}
	}

	if paneDead {
		if paneDeadStatus == 0 {
			return "done", true
		}
		return "error", true
	}

	// Capture pane and check for waiting prompts.
	snapOut, _ := tmuxOutput("capture-pane", "-t", sessionName, "-p", "-S", "-50")
	snapshot := trimSnapshot(string(snapOut))
	plainSnapshot := ansi.Strip(snapshot)
	detected := detectStatus(plainSnapshot, false, 0, flavor)
	return detected.String(), true
}
