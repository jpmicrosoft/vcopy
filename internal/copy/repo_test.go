package copy

import (
	"testing"
)

func TestSanitizeOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		srcToken string
		tgtToken string
		want     string
	}{
		{
			name:     "replaces source token",
			output:   "fatal: could not read from https://x-access-token:ghp_abc123@github.com/o/r.git",
			srcToken: "ghp_abc123",
			tgtToken: "ghp_xyz789",
			want:     "fatal: could not read from https://x-access-token:[REDACTED]@github.com/o/r.git",
		},
		{
			name:     "replaces target token",
			output:   "pushing to https://x-access-token:ghp_xyz789@ghes.example.com/o/r.git",
			srcToken: "ghp_abc123",
			tgtToken: "ghp_xyz789",
			want:     "pushing to https://x-access-token:[REDACTED]@ghes.example.com/o/r.git",
		},
		{
			name:     "replaces both tokens",
			output:   "src=ghp_abc123 tgt=ghp_xyz789",
			srcToken: "ghp_abc123",
			tgtToken: "ghp_xyz789",
			want:     "src=[REDACTED] tgt=[REDACTED]",
		},
		{
			name:     "no tokens in output",
			output:   "Cloning into bare repository...",
			srcToken: "ghp_abc123",
			tgtToken: "ghp_xyz789",
			want:     "Cloning into bare repository...",
		},
		{
			name:     "empty tokens",
			output:   "some output",
			srcToken: "",
			tgtToken: "",
			want:     "some output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeOutput(tt.output, tt.srcToken, tt.tgtToken)
			if got != tt.want {
				t.Errorf("sanitizeOutput() = %q, want %q", got, tt.want)
			}
		})
	}
}
