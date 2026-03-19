package worktree

import (
	"fmt"
	"os/exec"
	"path/filepath"
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
	out, err := gitOutput(repoRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}
	return parse(string(out), repoRoot), nil
}

func parse(raw, repoRoot string) []Worktree {
	var result []Worktree
	cleanRoot := filepath.Clean(repoRoot)
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
			}
		}
		if wt.Path != "" {
			wt.IsMain = filepath.Clean(wt.Path) == cleanRoot
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
	out, err := gitCombinedOutput(repoRoot, "worktree", "add", "-b", branch, path, baseBranch)
	if err != nil {
		return fmt.Errorf("git worktree add: %w\n%s", err, string(out))
	}
	return nil
}

// Delete removes a git worktree and its branch.
func Delete(repoRoot, path, branch string) error {
	out, err := gitCombinedOutput(repoRoot, "worktree", "remove", "--force", path)
	if err != nil {
		return fmt.Errorf("git worktree remove: %w\n%s", err, string(out))
	}
	if branch != "" {
		delOut, delErr := gitCombinedOutput(repoRoot, "branch", "-D", branch)
		if delErr != nil {
			return fmt.Errorf("worktree removed but failed to delete branch %q: %w\n%s", branch, delErr, strings.TrimSpace(string(delOut)))
		}
	}
	return nil
}

// Diff returns the git diff for the given worktree path.
func Diff(path string) (string, error) {
	out, err := gitOutput(path, "diff", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}

func gitOutput(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Output()
}

func gitCombinedOutput(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}
