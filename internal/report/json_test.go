package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jpmicrosoft/vcopy/internal/verify"
)

func TestWriteJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vcopy-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	reportPath := filepath.Join(tmpDir, "report.json")

	report := &verify.VerificationReport{
		SourceRepo: "owner/repo",
		TargetRepo: "org/repo",
		SourceHost: "github.com",
		TargetHost: "ghes.example.com",
		Timestamp:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Checks: []verify.CheckResult{
			{Name: "Refs", Status: "PASS", Details: "All 5 refs match"},
			{Name: "Objects", Status: "FAIL", Details: "2 missing"},
		},
	}

	if err := WriteJSON(report, reportPath); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	// Read back and verify
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("failed to read report: %v", err)
	}

	var parsed verify.VerificationReport
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if parsed.SourceRepo != "owner/repo" {
		t.Errorf("SourceRepo = %q, want %q", parsed.SourceRepo, "owner/repo")
	}
	if parsed.TargetHost != "ghes.example.com" {
		t.Errorf("TargetHost = %q", parsed.TargetHost)
	}
	if len(parsed.Checks) != 2 {
		t.Errorf("got %d checks, want 2", len(parsed.Checks))
	}
	if parsed.Checks[0].Status != "PASS" {
		t.Errorf("check 0 status = %q", parsed.Checks[0].Status)
	}
	if parsed.Checks[1].Status != "FAIL" {
		t.Errorf("check 1 status = %q", parsed.Checks[1].Status)
	}
}

func TestWriteJSON_BadPath(t *testing.T) {
	report := &verify.VerificationReport{}
	err := WriteJSON(report, "/nonexistent/dir/report.json")
	if err == nil {
		t.Error("expected error for bad path")
	}
}
