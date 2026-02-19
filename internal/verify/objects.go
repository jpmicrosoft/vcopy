package verify

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// VerifyObjects compares all git object hashes between source and target repos.
func VerifyObjects(srcHost, srcOwner, srcName, tgtHost, tgtOrg, tgtName, srcToken, tgtToken string, verbose bool) (*CheckResult, error) {
	result := &CheckResult{
		Name:   "Git Object Hash Verification",
		Status: StatusPass,
	}

	if verbose {
		fmt.Println("  Cloning source repo for object enumeration...")
	}
	srcPath, srcCleanup, err := cloneBareTmp(srcHost, srcOwner, srcName, srcToken, "src-obj")
	if err != nil {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("Failed to clone source: %v", err)
		return result, nil
	}
	defer srcCleanup()

	if verbose {
		fmt.Println("  Cloning target repo for object enumeration...")
	}
	tgtPath, tgtCleanup, err := cloneBareTmp(tgtHost, tgtOrg, tgtName, tgtToken, "tgt-obj")
	if err != nil {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("Failed to clone target: %v", err)
		return result, nil
	}
	defer tgtCleanup()

	srcObjects, err := listAllObjects(srcPath)
	if err != nil {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("Failed to enumerate source objects: %v", err)
		return result, nil
	}

	tgtObjects, err := listAllObjects(tgtPath)
	if err != nil {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("Failed to enumerate target objects: %v", err)
		return result, nil
	}

	// Compare
	srcSet := make(map[string]bool)
	for _, obj := range srcObjects {
		srcSet[obj] = true
	}
	tgtSet := make(map[string]bool)
	for _, obj := range tgtObjects {
		tgtSet[obj] = true
	}

	var missingSrc []string // in source but not target
	var extraTgt []string   // in target but not source

	for obj := range srcSet {
		if !tgtSet[obj] {
			missingSrc = append(missingSrc, obj)
		}
	}
	for obj := range tgtSet {
		if !srcSet[obj] {
			extraTgt = append(extraTgt, obj)
		}
	}

	if len(missingSrc) > 0 || len(extraTgt) > 0 {
		result.Status = StatusFail
		var details strings.Builder
		if len(missingSrc) > 0 {
			details.WriteString(fmt.Sprintf("%d objects in source missing from target", len(missingSrc)))
			if verbose {
				sort.Strings(missingSrc)
				for _, obj := range missingSrc[:min(10, len(missingSrc))] {
					fmt.Printf("  MISSING: %s\n", obj)
				}
			}
		}
		if len(extraTgt) > 0 {
			if details.Len() > 0 {
				details.WriteString("; ")
			}
			details.WriteString(fmt.Sprintf("%d extra objects in target", len(extraTgt)))
		}
		result.Details = details.String()
	} else {
		result.Details = fmt.Sprintf("All %d objects match", len(srcObjects))
	}

	return result, nil
}

// listAllObjects uses git rev-list --objects --all to enumerate all objects.
func listAllObjects(repoPath string) ([]string, error) {
	cmd := exec.Command("git", "-C", repoPath, "rev-list", "--objects", "--all")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git rev-list failed: %w", err)
	}

	var objects []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		// Each line is "sha [path]" — we only need the SHA
		parts := strings.Fields(line)
		if len(parts) >= 1 {
			objects = append(objects, parts[0])
		}
	}
	return objects, nil
}
