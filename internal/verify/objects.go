package verify

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// VerifyObjects compares all git object hashes between source and target repos.
func VerifyObjects(srcHost, srcOwner, srcName, tgtHost, tgtOrg, tgtName, srcToken, tgtToken string, opts Options) (*CheckResult, error) {
	result := &CheckResult{
		Name:   "Git Object Hash Verification",
		Status: StatusPass,
	}

	if opts.Verbose {
		fmt.Println("  Cloning source repo for object enumeration...")
	}
	srcPath, srcCleanup, err := cloneBareTmp(srcHost, srcOwner, srcName, srcToken, "src-obj")
	if err != nil {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("Failed to clone source: %v", err)
		return result, nil
	}
	defer srcCleanup()

	// Remove excluded refs from source clone so objects only reachable through
	// rejected branches are not included in the comparison.
	if len(opts.ExcludedRefs) > 0 {
		if err := removeExcludedRefsFromClone(srcPath, opts.ExcludedRefs); err != nil {
			result.Status = StatusFail
			result.Details = fmt.Sprintf("Failed to remove excluded refs from source clone: %v", err)
			return result, nil
		}
	}

	if opts.Verbose {
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

	// Missing source objects in target = integrity problem (FAIL).
	// Extra target objects = expected in additive mode (prior runs, cleanup commits) → WARN.
	if len(missingSrc) > 0 {
		result.Status = StatusFail
		var details strings.Builder
		details.WriteString(fmt.Sprintf("%d objects in source missing from target", len(missingSrc)))
		if opts.Verbose {
			sort.Strings(missingSrc)
			for _, obj := range missingSrc[:min(10, len(missingSrc))] {
				fmt.Printf("  MISSING: %s\n", obj)
			}
		}
		if len(extraTgt) > 0 {
			details.WriteString(fmt.Sprintf("; %d extra objects in target (expected — prior runs or cleanup commits)", len(extraTgt)))
		}
		result.Details = details.String()
	} else if len(extraTgt) > 0 {
		result.Status = StatusWarn
		result.Details = fmt.Sprintf("All %d source objects present in target; %d extra objects in target (expected — prior runs or cleanup commits)", len(srcObjects), len(extraTgt))
	} else if len(opts.ExcludedRefs) > 0 {
		result.Status = StatusWarn
		result.Details = fmt.Sprintf("All %d objects match (%d refs excluded — rejected by remote)", len(srcObjects), len(opts.ExcludedRefs))
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

// VerifyObjectsSince compares objects created after a given SHA or date.
func VerifyObjectsSince(srcHost, srcOwner, srcName, tgtHost, tgtOrg, tgtName, srcToken, tgtToken, since string, opts Options) (*CheckResult, error) {
	result := &CheckResult{
		Name:   "Incremental Object Verification",
		Status: StatusPass,
	}

	srcPath, srcCleanup, err := cloneBareTmp(srcHost, srcOwner, srcName, srcToken, "src-inc")
	if err != nil {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("Failed to clone source: %v", err)
		return result, nil
	}
	defer srcCleanup()

	// Remove excluded refs so incremental comparison skips rejected branches
	if len(opts.ExcludedRefs) > 0 {
		if err := removeExcludedRefsFromClone(srcPath, opts.ExcludedRefs); err != nil {
			result.Status = StatusFail
			result.Details = fmt.Sprintf("Failed to remove excluded refs from source clone: %v", err)
			return result, nil
		}
	}

	tgtPath, tgtCleanup, err := cloneBareTmp(tgtHost, tgtOrg, tgtName, tgtToken, "tgt-inc")
	if err != nil {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("Failed to clone target: %v", err)
		return result, nil
	}
	defer tgtCleanup()

	srcObjects, err := listObjectsSince(srcPath, since)
	if err != nil {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("Failed to enumerate source objects: %v", err)
		return result, nil
	}

	tgtObjects, err := listObjectsSince(tgtPath, since)
	if err != nil {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("Failed to enumerate target objects: %v", err)
		return result, nil
	}

	srcSet := make(map[string]bool)
	for _, obj := range srcObjects {
		srcSet[obj] = true
	}
	tgtSet := make(map[string]bool)
	for _, obj := range tgtObjects {
		tgtSet[obj] = true
	}

	var missing int
	for obj := range srcSet {
		if !tgtSet[obj] {
			missing++
		}
	}

	if missing > 0 {
		result.Status = StatusFail
		result.Details = fmt.Sprintf("%d objects since %s in source missing from target", missing, since)
	} else if len(opts.ExcludedRefs) > 0 {
		result.Status = StatusWarn
		result.Details = fmt.Sprintf("All %d objects since %s match (%d refs excluded — rejected by remote)", len(srcObjects), since, len(opts.ExcludedRefs))
	} else {
		result.Details = fmt.Sprintf("All %d objects since %s match", len(srcObjects), since)
	}

	return result, nil
}

// validateSince ensures the --since value is a valid SHA or date, not a flag injection.
func validateSince(since string) error {
	if strings.HasPrefix(since, "-") {
		return fmt.Errorf("invalid --since value %q: must not start with '-'", since)
	}
	// Allow hex SHA (short or full)
	isHex := true
	for _, c := range since {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			isHex = false
			break
		}
	}
	if isHex && len(since) >= 4 {
		return nil
	}
	// Allow date-like patterns (digits, dashes, colons, T, Z, +, spaces)
	for _, c := range since {
		if !((c >= '0' && c <= '9') || c == '-' || c == ':' || c == 'T' || c == 'Z' || c == '+' || c == ' ' || c == '.') {
			return fmt.Errorf("invalid --since value %q: must be a git SHA or date", since)
		}
	}
	return nil
}

// listObjectsSince lists objects reachable from commits after the given reference.
func listObjectsSince(repoPath, since string) ([]string, error) {
	if err := validateSince(since); err != nil {
		return nil, err
	}
	// Try as date first (--after=date), fall back to SHA range (since..HEAD)
	cmd := exec.Command("git", "-C", repoPath, "rev-list", "--objects", "--all", "--after="+since)
	out, err := cmd.Output()
	if err != nil {
		// Try as SHA range
		cmd = exec.Command("git", "-C", repoPath, "rev-list", "--objects", "--all", since+"..HEAD")
		out, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("git rev-list --since failed: %w", err)
		}
	}

	var objects []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 1 {
			objects = append(objects, parts[0])
		}
	}
	return objects, nil
}
