package copy

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	ghclient "github.com/jaiperez/vcopy/internal/github"
	"github.com/jaiperez/vcopy/internal/progress"
	"github.com/jaiperez/vcopy/internal/retry"
)

// MirrorRepo performs a bare clone from source and mirror push to target.
func MirrorRepo(srcHost, srcOwner, srcName, tgtHost, tgtOrg, tgtName, srcToken, tgtToken string, lfs, verbose bool) error {
	srcURL := ghclient.CloneURL(srcHost, srcOwner, srcName, srcToken)
	tgtURL := ghclient.CloneURL(tgtHost, tgtOrg, tgtName, tgtToken)

	tmpDir, err := os.MkdirTemp("", "vcopy-mirror-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	mirrorPath := filepath.Join(tmpDir, srcName+".git")

	// Bare clone from source
	sp := progress.Start(fmt.Sprintf("Cloning %s/%s/%s...", srcHost, srcOwner, srcName))
	cloneErr := retry.Do(retry.Default(), "git clone", func() error {
		return runGitCmd(verbose, srcToken, tgtToken, nil, "clone", "--bare", "--mirror", srcURL, mirrorPath)
	})
	if cloneErr != nil {
		sp.StopFail()
		return fmt.Errorf("bare clone failed: %w", cloneErr)
	}
	sp.Stop()

	// Remove hidden refs (refs/pull/*) that GitHub rejects on push.
	// These are read-only refs created by GitHub for PR head/merge commits
	// and cannot be pushed to another repo.
	if err := removeHiddenRefs(mirrorPath, verbose); err != nil && verbose {
		fmt.Printf("  Warning: failed to clean hidden refs: %v\n", err)
	}

	// LFS: fetch all LFS objects from source
	if lfs {
		sp = progress.Start("Fetching LFS objects...")
		lfsErr := retry.Do(retry.Default(), "git lfs fetch", func() error {
			return runGitCmd(verbose, srcToken, tgtToken, &mirrorPath, "lfs", "fetch", "--all", srcURL)
		})
		if lfsErr != nil {
			sp.StopFail()
			return fmt.Errorf("LFS fetch failed: %w", lfsErr)
		}
		sp.Stop()
	} else {
		// Detect LFS usage and warn
		if hasLFSObjects(mirrorPath) {
			fmt.Println("  ⚠ Repository uses Git LFS but --lfs was not specified.")
			fmt.Println("    LFS objects will NOT be copied. Use --lfs to include them.")
		}
	}

	// Mirror push to target
	sp = progress.Start(fmt.Sprintf("Pushing to %s/%s/%s...", tgtHost, tgtOrg, tgtName))
	pushErr := retry.Do(retry.Default(), "git push", func() error {
		return runGitCmd(verbose, srcToken, tgtToken, &mirrorPath, "push", "--mirror", tgtURL)
	})
	if pushErr != nil {
		sp.StopFail()
		return fmt.Errorf("mirror push failed: %w", pushErr)
	}
	sp.Stop()

	// LFS: push all LFS objects to target
	if lfs {
		sp = progress.Start("Pushing LFS objects...")
		lfsPushErr := retry.Do(retry.Default(), "git lfs push", func() error {
			return runGitCmd(verbose, srcToken, tgtToken, &mirrorPath, "lfs", "push", "--all", tgtURL)
		})
		if lfsPushErr != nil {
			sp.StopFail()
			return fmt.Errorf("LFS push failed: %w", lfsPushErr)
		}
		sp.Stop()
	}

	return nil
}

// CopyWiki clones and pushes the wiki repository.
func CopyWiki(srcHost, srcOwner, srcName, tgtHost, tgtOrg, tgtName, srcToken, tgtToken string, verbose bool) error {
	srcURL := ghclient.CloneURL(srcHost, srcOwner, srcName+".wiki", srcToken)
	tgtURL := ghclient.CloneURL(tgtHost, tgtOrg, tgtName+".wiki", tgtToken)

	tmpDir, err := os.MkdirTemp("", "vcopy-wiki-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	wikiPath := filepath.Join(tmpDir, "wiki.git")

	if err := runGitCmd(false, srcToken, tgtToken, nil, "clone", "--bare", "--mirror", srcURL, wikiPath); err != nil {
		return fmt.Errorf("wiki clone failed (wiki may not exist): %w", err)
	}

	if err := runGitCmd(false, srcToken, tgtToken, &wikiPath, "push", "--mirror", tgtURL); err != nil {
		return fmt.Errorf("wiki push failed: %w", err)
	}

	return nil
}

// hasLFSObjects checks if a repo has .gitattributes with LFS filter entries.
func hasLFSObjects(repoPath string) bool {
	cmd := exec.Command("git", "-C", repoPath, "show", "HEAD:.gitattributes")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "filter=lfs")
}

// removeHiddenRefs deletes refs/pull/* and other hidden GitHub refs from a bare clone
// so that mirror push does not fail with "deny updating a hidden ref".
func removeHiddenRefs(repoPath string, verbose bool) error {
	cmd := exec.Command("git", "-C", repoPath, "for-each-ref", "--format=%(refname)", "refs/pull/")
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	refs := strings.TrimSpace(string(out))
	if refs == "" {
		return nil
	}

	// Build update-ref --stdin input to delete all PR refs
	var input strings.Builder
	count := 0
	for _, ref := range strings.Split(refs, "\n") {
		ref = strings.TrimSpace(ref)
		if ref != "" {
			input.WriteString(fmt.Sprintf("delete %s\n", ref))
			count++
		}
	}

	if count > 0 {
		delCmd := exec.Command("git", "-C", repoPath, "update-ref", "--stdin")
		delCmd.Stdin = strings.NewReader(input.String())
		if delErr := delCmd.Run(); delErr != nil {
			return fmt.Errorf("update-ref failed: %w", delErr)
		}
		if verbose {
			fmt.Printf("  Removed %d hidden refs (refs/pull/*) from bare clone\n", count)
		}
	}
	return nil
}

// runGitCmd executes a git command, sanitizing any token from output to prevent leakage.
func runGitCmd(verbose bool, srcToken, tgtToken string, dir *string, args ...string) error {
	gitArgs := args
	if dir != nil {
		gitArgs = append([]string{"-C", *dir}, args...)
	}

	cmd := exec.Command("git", gitArgs...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()

	if verbose {
		sanitized := sanitizeOutput(stdoutBuf.String(), srcToken, tgtToken)
		if sanitized != "" {
			fmt.Print(sanitized)
		}
		sanitized = sanitizeOutput(stderrBuf.String(), srcToken, tgtToken)
		if sanitized != "" {
			fmt.Print(sanitized)
		}
	}

	if err != nil {
		sanitizedErr := sanitizeOutput(stderrBuf.String(), srcToken, tgtToken)
		return fmt.Errorf("%w: %s", err, sanitizedErr)
	}
	return nil
}

// sanitizeOutput replaces tokens in output with [REDACTED] to prevent credential leakage.
func sanitizeOutput(output, srcToken, tgtToken string) string {
	if srcToken != "" {
		output = strings.ReplaceAll(output, srcToken, "[REDACTED]")
	}
	if tgtToken != "" {
		output = strings.ReplaceAll(output, tgtToken, "[REDACTED]")
	}
	return output
}
