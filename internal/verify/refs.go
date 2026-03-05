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
func VerifyRefs(srcHost, srcOwner, srcName, tgtHost, tgtOrg, tgtName, srcToken, tgtToken string, opts Options) (*CheckResult, error) {
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

	excluded := buildExcludedSet(opts.ExcludedRefs)
	var refResults []RefResult
	var mismatches, excludedCount int

	// Check all source refs exist in target with same SHA
	for ref, srcSHA := range srcRefs {
		if excluded[ref] {
			excludedCount++
			continue
		}

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
			if opts.Verbose {
				if !exists {
					fmt.Printf("  MISSING in target: %s (%s)\n", ref, srcSHA)
				} else {
					fmt.Printf("  MISMATCH: %s source=%s target=%s\n", ref, srcSHA, tgtSHA)
				}
			}
		}
		refResults = append(refResults, rr)
	}

	// Check for extra refs in target. Excluded refs still exist in srcRefs, so
	// they will not be incorrectly flagged as "extra".
	// Extra target refs are expected in additive mode (prior runs, cleanup commits).
	var extraTgt int
	for ref, tgtSHA := range tgtRefs {
		if _, exists := srcRefs[ref]; !exists {
			refResults = append(refResults, RefResult{
				Name:      ref,
				Type:      "extra",
				TargetSHA: tgtSHA,
				Match:     false,
			})
			extraTgt++
			if opts.Verbose {
				fmt.Printf("  EXTRA in target: %s (%s)\n", ref, tgtSHA)
			}
		}
	}

	if mismatches > 0 {
		result.Status = StatusFail
		var details strings.Builder
		details.WriteString(fmt.Sprintf("%d source ref mismatches out of %d checked refs", mismatches, len(srcRefs)-excludedCount))
		if extraTgt > 0 {
			details.WriteString(fmt.Sprintf("; %d extra refs in target (expected — prior runs or cleanup commits)", extraTgt))
		}
		result.Details = details.String()
	} else if extraTgt > 0 || excludedCount > 0 {
		result.Status = StatusWarn
		var parts []string
		parts = append(parts, fmt.Sprintf("All %d source refs match", len(srcRefs)-excludedCount))
		if extraTgt > 0 {
			parts = append(parts, fmt.Sprintf("%d extra refs in target (expected — prior runs or cleanup commits)", extraTgt))
		}
		if excludedCount > 0 {
			parts = append(parts, fmt.Sprintf("%d refs excluded — rejected by remote", excludedCount))
		}
		result.Details = strings.Join(parts, "; ")
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
		return nil, fmt.Errorf("git ls-remote failed: %w", sanitizeError(err, token))
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

	repoPath := filepath.Join(tmpDir, sanitizeRepoName(repo)+".git")
	cmd := exec.Command("git", "clone", "--bare", url, repoPath)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		return "", nil, fmt.Errorf("bare clone failed: %w", sanitizeError(err, token))
	}

	// Remove hidden refs (refs/pull/*) so verification comparisons are accurate
	if err := removeHiddenRefsFromClone(repoPath); err != nil {
		os.RemoveAll(tmpDir)
		return "", nil, fmt.Errorf("hidden ref cleanup failed: %w", err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }
	return repoPath, cleanup, nil
}
