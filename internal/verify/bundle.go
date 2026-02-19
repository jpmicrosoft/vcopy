package verify

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	ghclient "github.com/jaiperez/vcopy/internal/github"
)

// VerifyBundle creates git bundles from both source and target, computes SHA-256 checksums, and compares them.
func VerifyBundle(srcHost, srcOwner, srcName, tgtHost, tgtOrg, tgtName, srcToken, tgtToken string, verbose bool) (*CheckResult, error) {
	result := &CheckResult{
		Name:   "Bundle Checksum Verification",
		Status: StatusPass,
	}

	srcChecksum, err := createBundleChecksum(srcHost, srcOwner, srcName, srcToken, "src", verbose)
	if err != nil {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("Failed to create source bundle: %v", err)
		return result, nil
	}

	tgtChecksum, err := createBundleChecksum(tgtHost, tgtOrg, tgtName, tgtToken, "tgt", verbose)
	if err != nil {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("Failed to create target bundle: %v", err)
		return result, nil
	}

	if srcChecksum != tgtChecksum {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("Bundle checksums differ: source=%s target=%s", srcChecksum, tgtChecksum)
		if verbose {
			fmt.Printf("  Source bundle SHA-256: %s\n", srcChecksum)
			fmt.Printf("  Target bundle SHA-256: %s\n", tgtChecksum)
		}
	} else {
		result.Details = fmt.Sprintf("Bundle checksums match: %s", srcChecksum)
		if verbose {
			fmt.Printf("  SHA-256: %s\n", srcChecksum)
		}
	}

	return result, nil
}

// createBundleChecksum clones a repo, creates a git bundle, and returns its SHA-256 checksum.
func createBundleChecksum(host, owner, repo, token, prefix string, verbose bool) (string, error) {
	url := ghclient.CloneURL(host, owner, repo, token)

	tmpDir, err := os.MkdirTemp("", "vcopy-bundle-"+prefix+"-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	repoPath := filepath.Join(tmpDir, repo+".git")
	bundlePath := filepath.Join(tmpDir, repo+".bundle")

	// Bare clone
	if verbose {
		fmt.Printf("  Cloning %s/%s/%s for bundle creation...\n", host, owner, repo)
	}
	cloneCmd := exec.Command("git", "clone", "--bare", url, repoPath)
	cloneCmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if err := cloneCmd.Run(); err != nil {
		return "", fmt.Errorf("clone failed: %w", err)
	}

	// Remove hidden refs (refs/pull/*) so bundles match between source and target
	removeHiddenRefsFromClone(repoPath)

	// Create bundle
	bundleCmd := exec.Command("git", "-C", repoPath, "bundle", "create", bundlePath, "--all")
	if err := bundleCmd.Run(); err != nil {
		return "", fmt.Errorf("bundle create failed: %w", err)
	}

	// Compute SHA-256
	checksum, err := fileSHA256(bundlePath)
	if err != nil {
		return "", fmt.Errorf("checksum computation failed: %w", err)
	}

	return checksum, nil
}

// fileSHA256 computes the SHA-256 hash of a file.
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// removeHiddenRefsFromClone removes refs/pull/* from a bare clone so that
// verification bundles and comparisons are not affected by GitHub's hidden refs.
func removeHiddenRefsFromClone(repoPath string) {
	cmd := exec.Command("git", "-C", repoPath, "for-each-ref", "--format=%(refname)", "refs/pull/")
	out, err := cmd.Output()
	if err != nil {
		return
	}
	refs := strings.TrimSpace(string(out))
	if refs == "" {
		return
	}
	var input strings.Builder
	for _, ref := range strings.Split(refs, "\n") {
		ref = strings.TrimSpace(ref)
		if ref != "" {
			input.WriteString(fmt.Sprintf("delete %s\n", ref))
		}
	}
	delCmd := exec.Command("git", "-C", repoPath, "update-ref", "--stdin")
	delCmd.Stdin = strings.NewReader(input.String())
	delCmd.Run()
}
