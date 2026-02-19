package verify

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	ghclient "github.com/jpmicrosoft/vcopy/internal/github"
)

// RefResult holds comparison results for a single ref.
type RefResult struct {
	Name      string `json:"name"`
	Type      string `json:"type"` // "branch" or "tag"
	SourceSHA string `json:"source_sha"`
	TargetSHA string `json:"target_sha"`
	Match     bool   `json:"match"`
}

// VerifyRefs compares all branches and tags between source and target.
func VerifyRefs(srcHost, srcOwner, srcName, tgtHost, tgtOrg, tgtName, srcToken, tgtToken string, verbose bool) (*CheckResult, error) {
	result := &CheckResult{
		Name:   "Branch/Tag Ref Comparison",
		Status: StatusPass,
	}

	srcRefs, err := listRemoteRefs(srcHost, srcOwner, srcName, srcToken)
	if err != nil {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("Failed to list source refs: %v", err)
		return result, nil
	}

	tgtRefs, err := listRemoteRefs(tgtHost, tgtOrg, tgtName, tgtToken)
	if err != nil {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("Failed to list target refs: %v", err)
		return result, nil
	}

	var refResults []RefResult
	var mismatches int

	// Check all source refs exist in target with same SHA
	for ref, srcSHA := range srcRefs {
		tgtSHA, exists := tgtRefs[ref]
		refType := "branch"
		if strings.HasPrefix(ref, "refs/tags/") {
			refType = "tag"
		}

		rr := RefResult{
			Name:      ref,
			Type:      refType,
			SourceSHA: srcSHA,
			TargetSHA: tgtSHA,
			Match:     exists && srcSHA == tgtSHA,
		}

		if !rr.Match {
			mismatches++
			if verbose {
				if !exists {
					fmt.Printf("  MISSING in target: %s (%s)\n", ref, srcSHA)
				} else {
					fmt.Printf("  MISMATCH: %s source=%s target=%s\n", ref, srcSHA, tgtSHA)
				}
			}
		}
		refResults = append(refResults, rr)
	}

	// Check for extra refs in target
	for ref, tgtSHA := range tgtRefs {
		if _, exists := srcRefs[ref]; !exists {
			refResults = append(refResults, RefResult{
				Name:      ref,
				Type:      "extra",
				TargetSHA: tgtSHA,
				Match:     false,
			})
			mismatches++
			if verbose {
				fmt.Printf("  EXTRA in target: %s (%s)\n", ref, tgtSHA)
			}
		}
	}

	if mismatches > 0 {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("%d ref mismatches out of %d total refs", mismatches, len(srcRefs))
	} else {
		result.Details = fmt.Sprintf("All %d refs match", len(srcRefs))
	}

	return result, nil
}

// listRemoteRefs uses git ls-remote to list all refs on a remote.
func listRemoteRefs(host, owner, repo, token string) (map[string]string, error) {
	url := ghclient.CloneURL(host, owner, repo, token)

	tmpDir, err := os.MkdirTemp("", "vcopy-refs-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.Command("git", "ls-remote", url)
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-remote failed: %w", err)
	}

	refs := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			sha := parts[0]
			ref := parts[1]
			// Skip HEAD (symbolic ref) and hidden GitHub refs (refs/pull/*)
			if ref == "HEAD" || strings.HasPrefix(ref, "refs/pull/") {
				continue
			}
			refs[ref] = sha
		}
	}
	return refs, nil
}

// cloneBareTmp clones a repo bare into a temp directory and returns the path.
func cloneBareTmp(host, owner, repo, token, prefix string) (string, func(), error) {
	url := ghclient.CloneURL(host, owner, repo, token)

	tmpDir, err := os.MkdirTemp("", "vcopy-"+prefix+"-*")
	if err != nil {
		return "", nil, err
	}

	repoPath := filepath.Join(tmpDir, repo+".git")
	cmd := exec.Command("git", "clone", "--bare", url, repoPath)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		return "", nil, fmt.Errorf("bare clone failed: %w", err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }
	return repoPath, cleanup, nil
}
