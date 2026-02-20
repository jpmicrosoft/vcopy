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

// VerifyBundle creates git bundles from both source and target, verifies they are valid,
// and compares the refs they contain. Raw checksums are not compared because git bundles
// are non-deterministic (packfile compression/ordering varies between clones).
func VerifyBundle(srcHost, srcOwner, srcName, tgtHost, tgtOrg, tgtName, srcToken, tgtToken string, verbose bool) (*CheckResult, error) {
	result := &CheckResult{
		Name:   "Bundle Integrity Verification",
		Status: StatusPass,
	}

	srcRefs, srcChecksum, err := createAndVerifyBundle(srcHost, srcOwner, srcName, srcToken, "src", verbose)
	if err != nil {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("Source bundle failed: %v", err)
		return result, nil
	}

	tgtRefs, tgtChecksum, err := createAndVerifyBundle(tgtHost, tgtOrg, tgtName, tgtToken, "tgt", verbose)
	if err != nil {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("Target bundle failed: %v", err)
		return result, nil
	}

	// Compare bundle refs (deterministic) rather than raw checksums (non-deterministic)
	var mismatches int
	for ref, srcSHA := range srcRefs {
		if tgtSHA, ok := tgtRefs[ref]; !ok {
			mismatches++
			if verbose {
				fmt.Printf("  MISSING in target bundle: %s\n", ref)
			}
		} else if srcSHA != tgtSHA {
			mismatches++
			if verbose {
				fmt.Printf("  MISMATCH: %s source=%s target=%s\n", ref, srcSHA, tgtSHA)
			}
		}
	}
	for ref := range tgtRefs {
		if _, ok := srcRefs[ref]; !ok {
			mismatches++
			if verbose {
				fmt.Printf("  EXTRA in target bundle: %s\n", ref)
			}
		}
	}

	if mismatches > 0 {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("Bundle ref mismatches: %d (source SHA-256: %s, target SHA-256: %s)", mismatches, srcChecksum, tgtChecksum)
	} else {
		result.Details = fmt.Sprintf("Both bundles valid, all %d refs match (source: %s, target: %s)", len(srcRefs), srcChecksum[:12], tgtChecksum[:12])
	}

	return result, nil
}

// createAndVerifyBundle clones a repo, creates a git bundle, verifies it, and returns
// the bundle's ref list and SHA-256 checksum.
func createAndVerifyBundle(host, owner, repo, token, prefix string, verbose bool) (map[string]string, string, error) {
	url := ghclient.CloneURL(host, owner, repo, token)

	tmpDir, err := os.MkdirTemp("", "vcopy-bundle-"+prefix+"-*")
	if err != nil {
		return nil, "", err
	}
	defer os.RemoveAll(tmpDir)

	safeName := sanitizeRepoName(repo)
	repoPath := filepath.Join(tmpDir, safeName+".git")
	bundlePath := filepath.Join(tmpDir, safeName+".bundle")

	// Bare clone
	if verbose {
		fmt.Printf("  Cloning %s/%s/%s for bundle verification...\n", host, owner, repo)
	}
	cloneCmd := exec.Command("git", "clone", "--bare", url, repoPath)
	cloneCmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if err := cloneCmd.Run(); err != nil {
		return nil, "", fmt.Errorf("clone failed: %w", err)
	}

	// Remove hidden refs (refs/pull/*) so bundles match between source and target
	removeHiddenRefsFromClone(repoPath)

	// Create bundle
	bundleCmd := exec.Command("git", "-C", repoPath, "bundle", "create", bundlePath, "--all")
	if err := bundleCmd.Run(); err != nil {
		return nil, "", fmt.Errorf("bundle create failed: %w", err)
	}

	// Verify bundle is valid
	verifyCmd := exec.Command("git", "bundle", "verify", bundlePath)
	verifyCmd.Dir = repoPath
	if err := verifyCmd.Run(); err != nil {
		return nil, "", fmt.Errorf("bundle verify failed: %w", err)
	}

	// List refs in bundle
	listCmd := exec.Command("git", "bundle", "list-heads", bundlePath)
	out, err := listCmd.Output()
	if err != nil {
		return nil, "", fmt.Errorf("bundle list-heads failed: %w", err)
	}

	refs := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			refs[parts[1]] = parts[0] // ref -> SHA
		}
	}

	// Compute SHA-256 for informational purposes
	checksum, err := fileSHA256(bundlePath)
	if err != nil {
		return nil, "", fmt.Errorf("checksum failed: %w", err)
	}

	return refs, checksum, nil
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
