package report

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jpmicrosoft/vcopy/internal/verify"
)

// BatchRepoResult holds the outcome of a single repo within a batch operation.
type BatchRepoResult struct {
	SourceRepo string                       `json:"source_repo"`
	TargetRepo string                       `json:"target_repo"`
	Status     string                       `json:"status"` // succeeded, failed, skipped
	Error      string                       `json:"error,omitempty"`
	Checks     []verify.CheckResult         `json:"checks,omitempty"`
}

// BatchReport is the combined verification report for a batch copy operation.
type BatchReport struct {
	SourceOrg  string            `json:"source_org"`
	TargetOrg  string            `json:"target_org"`
	SourceHost string            `json:"source_host"`
	TargetHost string            `json:"target_host"`
	SearchFilter string          `json:"search_filter"`
	Timestamp  time.Time         `json:"timestamp"`
	Summary    BatchSummary      `json:"summary"`
	Repos      []BatchRepoResult `json:"repos"`
}

// BatchSummary holds counts for the batch operation.
type BatchSummary struct {
	Total           int `json:"total"`
	Succeeded       int `json:"succeeded"`
	Failed          int `json:"failed"`
	Skipped         int `json:"skipped"`
	ReleasesSkipped int `json:"releases_skipped,omitempty"`
}

// WriteBatchJSON writes the batch verification report to a JSON file.
func WriteBatchJSON(report *BatchReport, path string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal batch report: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write batch report file: %w", err)
	}
	return nil
}
