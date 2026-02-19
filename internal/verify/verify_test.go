package verify

import (
	"testing"
	"time"
)

func TestVerificationReport_AllPassed(t *testing.T) {
	report := &VerificationReport{
		SourceRepo: "owner/repo",
		TargetRepo: "org/repo",
		SourceHost: "github.com",
		TargetHost: "ghes.example.com",
		Timestamp:  time.Now(),
		Checks: []CheckResult{
			{Name: "Check 1", Status: StatusPass, Details: "ok"},
			{Name: "Check 2", Status: StatusPass, Details: "ok"},
			{Name: "Check 3", Status: StatusWarn, Details: "warning"},
		},
	}
	if !report.AllPassed() {
		t.Error("expected AllPassed to return true when all checks are PASS or WARN")
	}
}

func TestVerificationReport_NotAllPassed(t *testing.T) {
	report := &VerificationReport{
		Checks: []CheckResult{
			{Name: "Check 1", Status: StatusPass},
			{Name: "Check 2", Status: StatusFail},
		},
	}
	if report.AllPassed() {
		t.Error("expected AllPassed to return false when a check is FAIL")
	}
}

func TestVerificationReport_Empty(t *testing.T) {
	report := &VerificationReport{}
	if !report.AllPassed() {
		t.Error("expected AllPassed to return true for empty checks")
	}
}

func TestStatusConstants(t *testing.T) {
	if StatusPass != "PASS" {
		t.Errorf("StatusPass = %q", StatusPass)
	}
	if StatusFail != "FAIL" {
		t.Errorf("StatusFail = %q", StatusFail)
	}
	if StatusWarn != "WARN" {
		t.Errorf("StatusWarn = %q", StatusWarn)
	}
	if StatusSkip != "SKIP" {
		t.Errorf("StatusSkip = %q", StatusSkip)
	}
}
