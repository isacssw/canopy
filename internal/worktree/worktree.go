package worktree

import (
	"fmt"
	"os/exec"
	"strings"
)

type Worktree struct {
	Path       string
	Branch     string
	BaseBranch string // detected from branch name convention (e.g. feat/x → main)
	IsMain     bool
}

// List returns all git worktrees for the given repo root.
func List(repoRoot string) ([]Worktree, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}
	return parse(string(out)), nil
}

func parse(raw string) []Worktree {
	var result []Worktree
	blocks := strings.Split(strings.TrimSpace(raw), "\n\n")
	for _, block := range blocks {
		lines := strings.Split(strings.TrimSpace(block), "\n")
		wt := Worktree{}
		for _, line := range lines {
			if strings.HasPrefix(line, "worktree ") {
				wt.Path = strings.TrimPrefix(line, "worktree ")
			} else if strings.HasPrefix(line, "branch ") {
				ref := strings.TrimPrefix(line, "branch ")
				// refs/heads/feat/my-feature → feat/my-feature
				wt.Branch = strings.TrimPrefix(ref, "refs/heads/")
			} else if line == "bare" {
				wt.IsMain = true
			}
		}
		if wt.Path != "" {
			// The main worktree is always listed first by git
			if len(result) == 0 {
				wt.IsMain = true
			}
			wt.BaseBranch = detectBase(wt.Branch)
			result = append(result, wt)
		}
	}
	return result
}

// detectBase infers the base branch from naming conventions.
// feat/foo, fix/foo, chore/foo → main
// release/x → main
// hotfix/x → main
func detectBase(branch string) string {
	prefixes := []string{"feat/", "fix/", "chore/", "release/", "hotfix/", "refactor/", "test/"}
	for _, p := range prefixes {
		if strings.HasPrefix(branch, p) {
			return "main"
		}
	}
	return ""
}

// Create creates a new git worktree with a new branch.
func Create(repoRoot, path, branch, baseBranch string) error {
	cmd := exec.Command("git", "worktree", "add", "-b", branch, path, baseBranch)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree add: %w\n%s", err, string(out))
	}
	return nil
}

// Delete removes a git worktree and its branch.
func Delete(repoRoot, path, branch string) error {
	cmd := exec.Command("git", "worktree", "remove", "--force", path)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree remove: %w\n%s", err, string(out))
	}
	if branch != "" {
		exec.Command("git", "-C", repoRoot, "branch", "-D", branch).Run() //nolint
	}
	return nil
}

// Diff returns the git diff for the given worktree path.
func Diff(path string) (string, error) {
	cmd := exec.Command("git", "diff", "HEAD")
	cmd.Dir = path
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}
