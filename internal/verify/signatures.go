package verify

import (
	"fmt"
	"os/exec"
	"strings"
)

// SignatureResult holds the verification result for a signed commit.
type SignatureResult struct {
	CommitSHA string `json:"commit_sha"`
	InSource  bool   `json:"in_source"`
	InTarget  bool   `json:"in_target"`
	Match     bool   `json:"match"`
}

// VerifySignatures checks that GPG/SSH commit signatures are preserved after copy.
func VerifySignatures(srcHost, srcOwner, srcName, tgtHost, tgtOrg, tgtName, srcToken, tgtToken string, opts Options) (*CheckResult, error) {
	result := &CheckResult{
		Name:   "Commit Signature Verification",
		Status: StatusPass,
	}

	srcPath, srcCleanup, err := cloneBareTmp(srcHost, srcOwner, srcName, srcToken, "src-sig")
	if err != nil {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("Failed to clone source: %v", err)
		return result, nil
	}
	defer srcCleanup()

	// Remove excluded refs from source clone to avoid counting signatures
	// only reachable through rejected branches.
	if len(opts.ExcludedRefs) > 0 {
		if err := removeExcludedRefsFromClone(srcPath, opts.ExcludedRefs); err != nil {
			result.Status = StatusFail
			result.Details = fmt.Sprintf("Failed to remove excluded refs from source clone: %v", err)
			return result, nil
		}
	}

	tgtPath, tgtCleanup, err := cloneBareTmp(tgtHost, tgtOrg, tgtName, tgtToken, "tgt-sig")
	if err != nil {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("Failed to clone target: %v", err)
		return result, nil
	}
	defer tgtCleanup()

	// Find signed commits in source
	srcSigned, err := listSignedCommits(srcPath)
	if err != nil {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("Failed to list signed commits in source: %v", err)
		return result, nil
	}

	if len(srcSigned) == 0 {
		result.Status = StatusPass
		result.Details = "No signed commits found in source repository"
		return result, nil
	}

	// Check each signed commit exists with signature in target
	tgtSigned, err := listSignedCommits(tgtPath)
	if err != nil {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("Failed to list signed commits in target: %v", err)
		return result, nil
	}

	tgtSet := make(map[string]bool)
	for _, sha := range tgtSigned {
		tgtSet[sha] = true
	}

	var lost int
	for _, sha := range srcSigned {
		if !tgtSet[sha] {
			lost++
			if opts.Verbose {
				fmt.Printf("  SIGNATURE LOST: commit %s has signature in source but not target\n", sha)
			}
		}
	}

	if lost > 0 {
		result.Status = StatusWarn
		result.Details = fmt.Sprintf("%d of %d signed commits lost signatures after copy", lost, len(srcSigned))
	} else {
		result.Details = fmt.Sprintf("All %d signed commits preserved signatures", len(srcSigned))
	}

	return result, nil
}

// listSignedCommits returns SHAs of all commits with GPG/SSH signatures.
func listSignedCommits(repoPath string) ([]string, error) {
	// Use log format to show commit SHA and signature status
	cmd := exec.Command("git", "-C", repoPath, "log", "--all", "--format=%H %G?")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var signed []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			sha := parts[0]
			sigStatus := parts[1]
			// G = good, B = bad, U = untrusted, X = expired, Y = expired key, R = revoked
			// E = cannot check, N = no signature
			if sigStatus != "N" {
				signed = append(signed, sha)
			}
		}
	}
	return signed, nil
}
