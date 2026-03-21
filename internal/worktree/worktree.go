package worktree

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
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
			wt.BaseBranch = detectBase(wt.Branch)
			result = append(result, wt)
		}
	}

	// Prefer an explicit path match. If repoRoot is not canonical (or missing),
	// fall back to the first git worktree entry, which is the main worktree.
	mainIdx := -1
	cleanRoot := filepath.Clean(repoRoot)
	for i := range result {
		if filepath.Clean(result[i].Path) == cleanRoot {
			mainIdx = i
			break
		}
	}
	if mainIdx == -1 && len(result) > 0 {
		mainIdx = 0
	}
	if mainIdx >= 0 {
		result[mainIdx].IsMain = true
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

// DiffFile holds per-file diff information.
type DiffFile struct {
	Name     string // e.g. "internal/ui/model.go"
	OldName  string // non-empty for renames
	Status   string // "M", "A", "D", "R"
	Added    int
	Removed  int
	IsBinary bool
	Patch    string // raw diff chunk for this file
}

// DiffResult holds the parsed diff for all changed files.
type DiffResult struct {
	Files        []DiffFile
	TotalAdded   int
	TotalRemoved int
}

// DiffParsed returns structured per-file diff data.
func DiffParsed(path string) (DiffResult, error) {
	var result DiffResult

	// Get per-file stats
	numstatOut, err := gitOutput(path, "diff", "HEAD", "--numstat")
	if err != nil {
		return result, fmt.Errorf("git diff --numstat: %w", err)
	}

	// Get full patch
	patchOut, err := gitOutput(path, "diff", "HEAD")
	if err != nil {
		return result, fmt.Errorf("git diff: %w", err)
	}

	// Parse numstat into a map: filename -> (added, removed, isBinary)
	type stat struct {
		added, removed int
		isBinary       bool
	}
	stats := map[string]stat{}
	for _, line := range strings.Split(strings.TrimSpace(string(numstatOut)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		name := parts[2]
		// Handle renames: "old => new" or "{old => new}/path"
		if idx := strings.Index(name, " => "); idx >= 0 {
			// Use the new name as key
			name = resolveRenamePath(name)
		}
		if parts[0] == "-" && parts[1] == "-" {
			stats[name] = stat{isBinary: true}
		} else {
			a, _ := strconv.Atoi(parts[0])
			r, _ := strconv.Atoi(parts[1])
			stats[name] = stat{added: a, removed: r}
		}
	}

	// Split patch into per-file chunks
	fullPatch := string(patchOut)
	chunks := splitPatchByFile(fullPatch)

	// Build DiffFile entries from patch chunks
	for _, chunk := range chunks {
		df := parsePatchChunk(chunk)
		if s, ok := stats[df.Name]; ok {
			df.Added = s.added
			df.Removed = s.removed
			df.IsBinary = s.isBinary
		} else if df.OldName != "" {
			if s, ok := stats[df.OldName]; ok {
				df.Added = s.added
				df.Removed = s.removed
				df.IsBinary = s.isBinary
			}
		}
		result.TotalAdded += df.Added
		result.TotalRemoved += df.Removed
		result.Files = append(result.Files, df)
	}

	return result, nil
}

// splitPatchByFile splits a full git diff into per-file chunks.
func splitPatchByFile(patch string) []string {
	if patch == "" {
		return nil
	}
	const marker = "diff --git "
	var chunks []string
	rest := patch
	for {
		idx := strings.Index(rest, marker)
		if idx < 0 {
			break
		}
		// Find the next "diff --git" after the current one
		next := strings.Index(rest[idx+len(marker):], marker)
		if next < 0 {
			chunks = append(chunks, rest[idx:])
			break
		}
		chunks = append(chunks, rest[idx:idx+len(marker)+next])
		rest = rest[idx+len(marker)+next:]
	}
	return chunks
}

// parsePatchChunk extracts file metadata from a single diff chunk.
func parsePatchChunk(chunk string) DiffFile {
	df := DiffFile{Patch: chunk, Status: "M"}
	lines := strings.SplitN(chunk, "\n", 20) // only need headers

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git a/") {
			parts := strings.SplitN(line, " b/", 2)
			if len(parts) == 2 {
				df.Name = parts[1]
			}
		} else if strings.HasPrefix(line, "new file mode") {
			df.Status = "A"
		} else if strings.HasPrefix(line, "deleted file mode") {
			df.Status = "D"
		} else if strings.HasPrefix(line, "rename from ") {
			df.OldName = strings.TrimPrefix(line, "rename from ")
			df.Status = "R"
		} else if strings.HasPrefix(line, "rename to ") {
			df.Name = strings.TrimPrefix(line, "rename to ")
		} else if strings.HasPrefix(line, "Binary files") {
			df.IsBinary = true
		} else if strings.HasPrefix(line, "@@") {
			break // done with headers
		}
	}
	return df
}

// resolveRenamePath handles numstat rename formats like "{a => b}/c" or "a => b".
func resolveRenamePath(name string) string {
	// Handle brace format: "dir/{old.go => new.go}" or "{old => new}/file.go"
	if lbrace := strings.Index(name, "{"); lbrace >= 0 {
		if rbrace := strings.Index(name, "}"); rbrace > lbrace {
			prefix := name[:lbrace]
			suffix := name[rbrace+1:]
			inner := name[lbrace+1 : rbrace]
			if arrow := strings.Index(inner, " => "); arrow >= 0 {
				newPart := inner[arrow+4:]
				return filepath.Clean(prefix + newPart + suffix)
			}
		}
	}
	// Simple "old => new" format
	if arrow := strings.Index(name, " => "); arrow >= 0 {
		return name[arrow+4:]
	}
	return name
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
