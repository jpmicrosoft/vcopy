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

func TestSanitizeRepoName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"normal name", "my-repo", "my-repo"},
		{"path traversal", "../../../etc/passwd", "passwd"},
		{"double dots only", "..", "repo"},
		{"dot only", ".", "repo"},
		{"empty string", "", "repo"},
		{"slashes stripped", "foo/bar/baz", "baz"},
		{"embedded double dots", "my..repo", "myrepo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeRepoName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeRepoName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildExcludedSet(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		check string
		want  bool
	}{
		{"nil input", nil, "refs/heads/main", false},
		{"empty input", []string{}, "refs/heads/main", false},
		{"present ref", []string{"refs/heads/main", "refs/heads/dev"}, "refs/heads/main", true},
		{"absent ref", []string{"refs/heads/main"}, "refs/heads/dev", false},
		{"duplicate refs", []string{"refs/heads/main", "refs/heads/main"}, "refs/heads/main", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := buildExcludedSet(tt.input)
			got := set[tt.check]
			if got != tt.want {
				t.Errorf("buildExcludedSet(%v)[%q] = %v, want %v", tt.input, tt.check, got, tt.want)
			}
		})
	}
}

func TestRemoveExcludedRefsFromClone_Validation(t *testing.T) {
	tests := []struct {
		name    string
		refs    []string
		wantErr bool
	}{
		{"empty refs", []string{}, false},
		{"invalid ref no prefix", []string{"main"}, true},
		{"invalid ref with newline", []string{"refs/heads/main\nevil"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := removeExcludedRefsFromClone("/nonexistent", tt.refs)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}