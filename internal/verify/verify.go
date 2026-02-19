package verify

import (
	"fmt"
	"time"
)

// Status constants for verification checks.
const (
	StatusPass = "PASS"
	StatusFail = "FAIL"
	StatusWarn = "WARN"
	StatusSkip = "SKIP"
)

// CheckResult holds the result of a single verification check.
type CheckResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Details string `json:"details"`
}

// VerificationReport holds all verification results.
type VerificationReport struct {
	SourceRepo string        `json:"source_repo"`
	TargetRepo string        `json:"target_repo"`
	SourceHost string        `json:"source_host"`
	TargetHost string        `json:"target_host"`
	Timestamp  time.Time     `json:"timestamp"`
	Checks     []CheckResult `json:"checks"`
}

// AllPassed returns true if all checks passed (PASS or WARN).
func (r *VerificationReport) AllPassed() bool {
	for _, c := range r.Checks {
		if c.Status == StatusFail {
			return false
		}
	}
	return true
}

// RunAll executes all verification checks and returns a consolidated report.
func RunAll(srcHost, srcOwner, srcName, tgtHost, tgtOrg, tgtName, srcToken, tgtToken string, verbose bool) (*VerificationReport, error) {
	report := &VerificationReport{
		SourceRepo: fmt.Sprintf("%s/%s", srcOwner, srcName),
		TargetRepo: fmt.Sprintf("%s/%s", tgtOrg, tgtName),
		SourceHost: srcHost,
		TargetHost: tgtHost,
		Timestamp:  time.Now().UTC(),
	}

	type checkFunc func(string, string, string, string, string, string, string, string, bool) (*CheckResult, error)
	checks := []struct {
		name string
		fn   checkFunc
	}{
		{"Ref Comparison", VerifyRefs},
		{"Object Hashes", VerifyObjects},
		{"Tree Hashes", VerifyTrees},
		{"Commit Signatures", VerifySignatures},
		{"Bundle Checksum", VerifyBundle},
	}

	for _, check := range checks {
		if verbose {
			fmt.Printf("\nRunning: %s...\n", check.name)
		}
		result, err := check.fn(srcHost, srcOwner, srcName, tgtHost, tgtOrg, tgtName, srcToken, tgtToken, verbose)
		if err != nil {
			report.Checks = append(report.Checks, CheckResult{
				Name:    check.name,
				Status:  StatusFail,
				Details: fmt.Sprintf("Check error: %v", err),
			})
		} else {
			report.Checks = append(report.Checks, *result)
		}
	}

	return report, nil
}
