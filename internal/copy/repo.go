package copy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	ghclient "github.com/jpmicrosoft/vcopy/internal/github"
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
	cloneCmd := exec.Command("git", "clone", "--bare", "--mirror", srcURL, mirrorPath)
	cloneCmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if verbose {
		cloneCmd.Stdout = os.Stdout
		cloneCmd.Stderr = os.Stderr
	}
	if err := cloneCmd.Run(); err != nil {
		return fmt.Errorf("bare clone failed: %w", err)
	}

	// Mirror push to target
	if verbose {
		fmt.Printf("  Mirror pushing to %s/%s/%s...\n", tgtHost, tgtOrg, tgtName)
	}
	pushCmd := exec.Command("git", "-C", mirrorPath, "push", "--mirror", tgtURL)
	pushCmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if verbose {
		pushCmd.Stdout = os.Stdout
		pushCmd.Stderr = os.Stderr
	}
	if err := pushCmd.Run(); err != nil {
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

	cloneCmd := exec.Command("git", "clone", "--bare", "--mirror", srcURL, wikiPath)
	cloneCmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if err := cloneCmd.Run(); err != nil {
		return fmt.Errorf("wiki clone failed (wiki may not exist): %w", err)
	}

	pushCmd := exec.Command("git", "-C", wikiPath, "push", "--mirror", tgtURL)
	pushCmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if err := pushCmd.Run(); err != nil {
		return fmt.Errorf("wiki push failed: %w", err)
	}

	return nil
}
