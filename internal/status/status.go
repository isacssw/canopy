package status

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/isacssw/canopy/internal/agent"
	"github.com/isacssw/canopy/internal/config"
	"github.com/isacssw/canopy/internal/worktree"
)

// AgentInfo holds the agent state for a single worktree.
type AgentInfo struct {
	Session string `json:"session"`
	Status  string `json:"status"`
	Active  bool   `json:"active"`
}

// WorktreeInfo holds worktree metadata and its associated agent state.
type WorktreeInfo struct {
	Path       string    `json:"path"`
	Branch     string    `json:"branch"`
	IsMain     bool      `json:"is_main"`
	BaseBranch string    `json:"base_branch"`
	Agent      AgentInfo `json:"agent"`
}

// Summary holds aggregate agent counts.
type Summary struct {
	Total   int `json:"total"`
	Running int `json:"running"`
	Waiting int `json:"waiting"`
	Done    int `json:"done"`
	Error   int `json:"error"`
	Idle    int `json:"idle"`
}

// StatusOutput is the top-level JSON output for `canopy status --json`.
type StatusOutput struct {
	RepoRoot  string         `json:"repo_root"`
	Worktrees []WorktreeInfo `json:"worktrees"`
	Summary   Summary        `json:"summary"`
}

// Run gathers worktree and agent status and writes JSON to w.
func Run(w io.Writer, cfg *config.Config) error {
	wts, err := worktree.List(cfg.RepoRoot)
	if err != nil {
		return fmt.Errorf("listing worktrees: %w", err)
	}

	out := StatusOutput{
		RepoRoot:  cfg.RepoRoot,
		Worktrees: make([]WorktreeInfo, 0, len(wts)),
	}

	for _, wt := range wts {
		sessionName := agent.SessionNameFor(cfg.RepoRoot, wt.Branch, wt.Path)
		status, active := agent.ProbeSession(sessionName)

		info := WorktreeInfo{
			Path:       wt.Path,
			Branch:     wt.Branch,
			IsMain:     wt.IsMain,
			BaseBranch: wt.BaseBranch,
			Agent: AgentInfo{
				Session: sessionName,
				Status:  status,
				Active:  active,
			},
		}
		out.Worktrees = append(out.Worktrees, info)

		out.Summary.Total++
		switch status {
		case "running":
			out.Summary.Running++
		case "waiting":
			out.Summary.Waiting++
		case "done":
			out.Summary.Done++
		case "error":
			out.Summary.Error++
		default:
			out.Summary.Idle++
		}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
