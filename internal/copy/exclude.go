package copy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	ghclient "github.com/jaiperez/vcopy/internal/github"
	"github.com/jaiperez/vcopy/internal/progress"
)

// Well-known path sets for preset exclusion flags.
var (
	WorkflowPaths = []string{".github/workflows", ".github/actions"}
	CopilotPaths  = []string{
		".github/copilot-instructions.md",
		".github/copilot",
	}
	// CodeOwnersPaths are always removed because CODEOWNERS references
	// teams/users that typically don't exist in the target org.
	CodeOwnersPaths = []string{
		"CODEOWNERS",
		".github/CODEOWNERS",
		"docs/CODEOWNERS",
	}
)

// BuildExcludePaths merges preset flags, user-supplied --exclude paths, and
// always-excluded paths (CODEOWNERS) into a single deduplicated list.
func BuildExcludePaths(noWorkflows, noCopilot bool, extraPaths []string) ([]string, error) {
	seen := make(map[string]bool)
	var result []string

	add := func(paths []string) error {
		for _, p := range paths {
			clean, err := sanitizeExcludePath(p)
			if err != nil {
				return err
			}
			if !seen[clean] {
				seen[clean] = true
				result = append(result, clean)
			}
		}
		return nil
	}

	if noWorkflows {
		if err := add(WorkflowPaths); err != nil {
			return nil, err
		}
	}
	if noCopilot {
		if err := add(CopilotPaths); err != nil {
			return nil, err
		}
	}
	if err := add(extraPaths); err != nil {
		return nil, err
	}

	// Always remove CODEOWNERS — references source org teams/users
	// that won't exist in the target
	if err := add(CodeOwnersPaths); err != nil {
		return nil, err
	}

	return result, nil
}

// sanitizeExcludePath validates and normalises an exclude path.
// Rejects absolute paths, traversal attempts, and empty strings.
func sanitizeExcludePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	p = filepath.ToSlash(p) // normalise to forward slashes for git
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimSuffix(p, "/")

	if p == "" {
		return "", fmt.Errorf("empty exclude path")
	}
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "\\") {
		return "", fmt.Errorf("exclude path must be relative: %q", p)
	}
	if strings.Contains(p, "..") {
		return "", fmt.Errorf("exclude path must not contain '..': %q", p)
	}
	if strings.HasPrefix(p, "-") {
		return "", fmt.Errorf("exclude path must not start with '-': %q", p)
	}
	return p, nil
}

// CleanupExcludedPaths clones the target repo, removes the specified paths,
// and pushes a cleanup commit. This is a post-push operation that preserves
// full history — the cleanup appears as a single new commit.
func CleanupExcludedPaths(host, owner, repo, token string, paths []string, verbose bool) error {
	if len(paths) == 0 {
		return nil
	}

	sp := progress.Start("Removing excluded paths from target...")

	cloneURL := ghclient.CloneURL(host, owner, repo, token)

	tmpDir, err := os.MkdirTemp("", "vcopy-exclude-*")
	if err != nil {
		sp.StopFail()
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	repoPath := filepath.Join(tmpDir, sanitizeRepoName(repo))

	// Shallow clone (depth 1) — we only need the latest commit
	cmd := exec.Command("git", "clone", "--depth", "1", cloneURL, repoPath)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		sp.StopFail()
		return fmt.Errorf("clone failed: %w: %s", err, sanitizeOutput(string(out), token, ""))
	}

	// Check which paths actually exist before running git rm
	var toRemove []string
	for _, p := range paths {
		full := filepath.Join(repoPath, filepath.FromSlash(p))
		if _, err := os.Stat(full); err == nil {
			toRemove = append(toRemove, p)
		} else if verbose {
			fmt.Printf("  Skipping (not found): %s\n", p)
		}
	}

	if len(toRemove) == 0 {
		sp.Stop()
		if verbose {
			fmt.Println("  No excluded paths found in target — nothing to remove")
		}
		return nil
	}

	// git rm -rf <paths>
	rmArgs := append([]string{"-C", repoPath, "rm", "-rf", "--"}, toRemove...)
	rmCmd := exec.Command("git", rmArgs...)
	rmCmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := rmCmd.CombinedOutput(); err != nil {
		sp.StopFail()
		return fmt.Errorf("git rm failed: %w: %s", err, sanitizeOutput(string(out), token, ""))
	}

	// Configure git author for the cleanup commit
	configCmds := [][]string{
		{"-C", repoPath, "config", "user.email", "vcopy@automated"},
		{"-C", repoPath, "config", "user.name", "vcopy"},
	}
	for _, args := range configCmds {
		exec.Command("git", args...).Run()
	}

	// Commit
	msg := fmt.Sprintf("vcopy: remove excluded paths\n\nRemoved: %s", strings.Join(toRemove, ", "))
	commitCmd := exec.Command("git", "-C", repoPath, "commit", "-m", msg)
	commitCmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := commitCmd.CombinedOutput(); err != nil {
		sp.StopFail()
		return fmt.Errorf("commit failed: %w: %s", err, sanitizeOutput(string(out), token, ""))
	}

	// Push
	pushCmd := exec.Command("git", "-C", repoPath, "push", "origin", "HEAD")
	pushCmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := pushCmd.CombinedOutput(); err != nil {
		sp.StopFail()
		return fmt.Errorf("push failed: %w: %s", err, sanitizeOutput(string(out), token, ""))
	}

	sp.Stop()

	if verbose {
		fmt.Printf("  Removed %d path(s): %s\n", len(toRemove), strings.Join(toRemove, ", "))
	} else {
		fmt.Printf("  Removed %d excluded path(s) from target\n", len(toRemove))
	}

	return nil
}
