package report

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jpmicrosoft/vcopy/internal/verify"
)

// SignReport adds a GPG attestation signature to the verification report.
// The signature covers the SHA-256 hash of the report data (excluding the attestation field).
func SignReport(report *verify.VerificationReport, keyID string) error {
	// Compute hash of report data without attestation
	report.Attestation = nil
	data, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to marshal report for signing: %w", err)
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(data))

	// Sign the hash with GPG
	cmd := exec.Command("gpg", "--armor", "--detach-sign", "--default-key", keyID)
	cmd.Stdin = strings.NewReader(hash)
	sigBytes, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("GPG signing failed (is key %s available?): %w", keyID, err)
	}

	report.Attestation = &verify.Attestation{
		SignedBy:  keyID,
		Signature: string(sigBytes),
	}

	return nil
}

// VerifyAttestation verifies the GPG signature on a report.
func VerifyAttestation(report *verify.VerificationReport) (bool, error) {
	if report.Attestation == nil {
		return false, fmt.Errorf("report has no attestation signature")
	}

	// Recompute hash without attestation
	att := report.Attestation
	report.Attestation = nil
	data, err := json.Marshal(report)
	report.Attestation = att // restore
	if err != nil {
		return false, fmt.Errorf("failed to marshal report: %w", err)
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(data))

	// Write signature to temp file for gpg --verify
	sigFile, err := os.CreateTemp("", "vcopy-sig-*.asc")
	if err != nil {
		return false, fmt.Errorf("failed to create temp sig file: %w", err)
	}
	defer os.Remove(sigFile.Name())
	if _, err := sigFile.WriteString(att.Signature); err != nil {
		sigFile.Close()
		return false, fmt.Errorf("failed to write sig file: %w", err)
	}
	sigFile.Close()

	// Write hash to temp file as signed data
	dataFile, err := os.CreateTemp("", "vcopy-data-*.txt")
	if err != nil {
		return false, fmt.Errorf("failed to create temp data file: %w", err)
	}
	defer os.Remove(dataFile.Name())
	if _, err := dataFile.WriteString(hash); err != nil {
		dataFile.Close()
		return false, fmt.Errorf("failed to write data file: %w", err)
	}
	dataFile.Close()

	// gpg --verify <signature-file> <signed-data-file>
	cmd := exec.Command("gpg", "--verify", sigFile.Name(), dataFile.Name())
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("attestation verification failed: %w", err)
	}

	return true, nil
}
