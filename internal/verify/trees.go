package verify

import (
	"fmt"
	"os/exec"
	"strings"
)

// VerifyTrees compares root tree hashes for each branch between source and target.
func VerifyTrees(srcHost, srcOwner, srcName, tgtHost, tgtOrg, tgtName, srcToken, tgtToken string, opts Options) (*CheckResult, error) {
	result := &CheckResult{
		Name:   "Tree Hash Comparison",
		Status: StatusPass,
	}

	srcPath, srcCleanup, err := cloneBareTmp(srcHost, srcOwner, srcName, srcToken, "src-tree")
	if err != nil {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("Failed to clone source: %v", err)
		return result, nil
	}
	defer srcCleanup()

	tgtPath, tgtCleanup, err := cloneBareTmp(tgtHost, tgtOrg, tgtName, tgtToken, "tgt-tree")
	if err != nil {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("Failed to clone target: %v", err)
		return result, nil
	}
	defer tgtCleanup()

	// Get branch names from source, filtering out excluded refs
	branches, err := listBranches(srcPath)
	if err != nil {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("Failed to list branches: %v", err)
		return result, nil
	}

	excluded := buildExcludedSet(opts.ExcludedRefs)
	var filteredBranches []string
	for _, branch := range branches {
		if !excluded["refs/heads/"+branch] {
			filteredBranches = append(filteredBranches, branch)
		}
	}

	var mismatches int
	total := len(filteredBranches)

	for _, branch := range filteredBranches {
		srcTree, err := getTreeHash(srcPath, branch)
		if err != nil {
			if opts.Verbose {
				fmt.Printf("  Warning: could not get tree hash for source branch %s: %v\n", branch, err)
			}
			continue
		}

		tgtTree, err := getTreeHash(tgtPath, branch)
		if err != nil {
			mismatches++
			if opts.Verbose {
				fmt.Printf("  MISSING: branch %s not found in target\n", branch)
			}
			continue
		}

		if srcTree != tgtTree {
			mismatches++
			if opts.Verbose {
				fmt.Printf("  MISMATCH: branch %s source_tree=%s target_tree=%s\n", branch, srcTree, tgtTree)
			}
		}
	}

	excludedCount := len(branches) - len(filteredBranches)
	if mismatches > 0 {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("%d tree hash mismatches out of %d branches", mismatches, total)
	} else if excludedCount > 0 {
		result.Status = StatusWarn
		result.Details = fmt.Sprintf("All %d checked branch tree hashes match (%d branches excluded — rejected by remote)", total, excludedCount)
	} else {
		result.Details = fmt.Sprintf("All %d branch tree hashes match", total)
	}

	return result, nil
}

// listBranches lists all branches in a bare repo.
func listBranches(repoPath string) ([]string, error) {
	cmd := exec.Command("git", "-C", repoPath, "branch", "--list", "--format=%(refname:short)")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

// getTreeHash gets the root tree hash for a given branch.
func getTreeHash(repoPath, branch string) (string, error) {
	// Pass ref^{tree} as a single argument to avoid injection via branch names.
	// exec.Command does not invoke a shell, and git treats each arg separately,
	// but concatenating user input is fragile — use explicit ref syntax.
	ref := branch + "^{tree}"
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", ref)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
