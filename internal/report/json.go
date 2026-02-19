package report

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/jaiperez/vcopy/internal/verify"
)

// WriteJSON writes the verification report to a JSON file.
func WriteJSON(report *verify.VerificationReport, path string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write report file: %w", err)
	}

	return nil
}
