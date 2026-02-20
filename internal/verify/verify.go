package verify

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/jaiperez/vcopy/internal/progress"
)

// sanitizeRepoName strips path separators and traversal sequences from a repo name
// to prevent path traversal when used in temp file paths.
func sanitizeRepoName(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "..", "")
	if name == "" || name == "." {
		name = "repo"
	}
	return name
}

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
	Attestation *Attestation `json:"attestation,omitempty"`
}

// Attestation holds the GPG signature of the report.
type Attestation struct {
	SignedBy  string `json:"signed_by"`
	Signature string `json:"signature"`
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

// Options controls which verification checks to run.
type Options struct {
	QuickMode bool
	CodeOnly  bool   // skip tag-dependent checks (ref comparison, bundle)
	Since     string // SHA or date for incremental verification
	Verbose   bool
}

// RunAll executes all verification checks and returns a consolidated report.
func RunAll(srcHost, srcOwner, srcName, tgtHost, tgtOrg, tgtName, srcToken, tgtToken string, opts Options) (*VerificationReport, error) {
	report := &VerificationReport{
		SourceRepo: fmt.Sprintf("%s/%s", srcOwner, srcName),
		TargetRepo: fmt.Sprintf("%s/%s", tgtOrg, tgtName),
		SourceHost: srcHost,
		TargetHost: tgtHost,
		Timestamp:  time.Now().UTC(),
	}

	type checkFunc func(string, string, string, string, string, string, string, string, bool) (*CheckResult, error)

	// Quick mode: only refs + trees
	// Full mode: refs, objects, trees, signatures, bundle
	type checkDef struct {
		name         string
		fn           checkFunc
		quickSkip    bool
		codeOnlySkip bool
	}

	checks := []checkDef{
		{"Ref Comparison", VerifyRefs, false, true},
		{"Object Hashes", VerifyObjects, true, false},
		{"Tree Hashes", VerifyTrees, false, false},
		{"Commit Signatures", VerifySignatures, true, false},
		{"Bundle Integrity", VerifyBundle, true, true},
	}

	for _, check := range checks {
		if opts.QuickMode && check.quickSkip {
			report.Checks = append(report.Checks, CheckResult{
				Name:    check.name,
				Status:  StatusSkip,
				Details: "Skipped (quick mode)",
			})
			continue
		}
		if opts.CodeOnly && check.codeOnlySkip {
			report.Checks = append(report.Checks, CheckResult{
				Name:    check.name,
				Status:  StatusSkip,
				Details: "Skipped (code-only mode, tags not copied)",
			})
			continue
		}

		sp := progress.Start(fmt.Sprintf("Verifying: %s", check.name))
		result, err := check.fn(srcHost, srcOwner, srcName, tgtHost, tgtOrg, tgtName, srcToken, tgtToken, opts.Verbose)
		if err != nil {
			sp.StopFail()
			report.Checks = append(report.Checks, CheckResult{
				Name:    check.name,
				Status:  StatusFail,
				Details: fmt.Sprintf("Check error: %v", err),
			})
		} else {
			if result.Status == StatusFail {
				sp.StopFail()
			} else {
				sp.Stop()
			}
			report.Checks = append(report.Checks, *result)
		}
	}

	return report, nil
}

// RunIncremental runs verification only on objects newer than the given reference.
func RunIncremental(srcHost, srcOwner, srcName, tgtHost, tgtOrg, tgtName, srcToken, tgtToken, since string, verbose bool) (*VerificationReport, error) {
	report := &VerificationReport{
		SourceRepo: fmt.Sprintf("%s/%s", srcOwner, srcName),
		TargetRepo: fmt.Sprintf("%s/%s", tgtOrg, tgtName),
		SourceHost: srcHost,
		TargetHost: tgtHost,
		Timestamp:  time.Now().UTC(),
	}

	// Always do ref comparison (fast)
	sp := progress.Start("Verifying: Ref Comparison")
	refsResult, err := VerifyRefs(srcHost, srcOwner, srcName, tgtHost, tgtOrg, tgtName, srcToken, tgtToken, verbose)
	if err != nil {
		sp.StopFail()
		report.Checks = append(report.Checks, CheckResult{Name: "Ref Comparison", Status: StatusFail, Details: err.Error()})
	} else {
		if refsResult.Status == StatusFail {
			sp.StopFail()
		} else {
			sp.Stop()
		}
		report.Checks = append(report.Checks, *refsResult)
	}

	// Incremental object verification
	sp = progress.Start(fmt.Sprintf("Verifying: Objects since %s", since))
	objResult, err := VerifyObjectsSince(srcHost, srcOwner, srcName, tgtHost, tgtOrg, tgtName, srcToken, tgtToken, since, verbose)
	if err != nil {
		sp.StopFail()
		report.Checks = append(report.Checks, CheckResult{Name: "Incremental Objects", Status: StatusFail, Details: err.Error()})
	} else {
		if objResult.Status == StatusFail {
			sp.StopFail()
		} else {
			sp.Stop()
		}
		report.Checks = append(report.Checks, *objResult)
	}

	// Tree comparison (always)
	sp = progress.Start("Verifying: Tree Hashes")
	treeResult, err := VerifyTrees(srcHost, srcOwner, srcName, tgtHost, tgtOrg, tgtName, srcToken, tgtToken, verbose)
	if err != nil {
		sp.StopFail()
		report.Checks = append(report.Checks, CheckResult{Name: "Tree Hashes", Status: StatusFail, Details: err.Error()})
	} else {
		if treeResult.Status == StatusFail {
			sp.StopFail()
		} else {
			sp.Stop()
		}
		report.Checks = append(report.Checks, *treeResult)
	}

	return report, nil
}
