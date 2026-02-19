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

func TestValidateSince(t *testing.T) {
	valid := []string{
		"2025-06-01",
		"2025-06-01T12:00:00Z",
		"abc123def456",
		"a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
	}
	for _, v := range valid {
		if err := validateSince(v); err != nil {
			t.Errorf("validateSince(%q) unexpected error: %v", v, err)
		}
	}

	invalid := []string{
		"--upload-pack=evil",
		"-x",
		"foo;bar",
		"abc$(cmd)",
	}
	for _, v := range invalid {
		if err := validateSince(v); err == nil {
			t.Errorf("validateSince(%q) expected error, got nil", v)
		}
	}
}
