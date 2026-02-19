package report

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/jaiperez/vcopy/internal/verify"
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

	// Verify with GPG
	cmd := exec.Command("gpg", "--verify", "/dev/stdin")
	cmd.Stdin = strings.NewReader(att.Signature + "\n" + hash)
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("attestation verification failed: %w", err)
	}

	return true, nil
}
