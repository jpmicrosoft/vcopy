package copy

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	ghclient "github.com/jaiperez/vcopy/internal/github"
)

// MirrorRepo performs a bare clone from source and mirror push to target.
func MirrorRepo(srcHost, srcOwner, srcName, tgtHost, tgtOrg, tgtName, srcToken, tgtToken string, verbose bool) error {
	srcURL := ghclient.CloneURL(srcHost, srcOwner, srcName, srcToken)
	tgtURL := ghclient.CloneURL(tgtHost, tgtOrg, tgtName, tgtToken)

	tmpDir, err := os.MkdirTemp("", "vcopy-mirror-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	mirrorPath := filepath.Join(tmpDir, srcName+".git")

	// Bare clone from source
	if verbose {
		fmt.Printf("  Bare cloning from %s/%s/%s...\n", srcHost, srcOwner, srcName)
	}
	if err := runGitCmd(verbose, srcToken, tgtToken, nil, "clone", "--bare", "--mirror", srcURL, mirrorPath); err != nil {
		return fmt.Errorf("bare clone failed: %w", err)
	}

	// Mirror push to target
	if verbose {
		fmt.Printf("  Mirror pushing to %s/%s/%s...\n", tgtHost, tgtOrg, tgtName)
	}
	if err := runGitCmd(verbose, srcToken, tgtToken, &mirrorPath, "push", "--mirror", tgtURL); err != nil {
		return fmt.Errorf("mirror push failed: %w", err)
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
