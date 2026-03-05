package copy

import (
	"testing"
)

func TestSanitizeRepoName(t *testing.T) {
	tests := []struct {
		name string
		input string
		want string
	}{
		{"normal name", "my-repo", "my-repo"},
		{"path traversal", "../../../etc/passwd", "passwd"},
		{"double dots only", "..", "repo"},
		{"dot only", ".", "repo"},
		{"empty string", "", "repo"},
		{"slashes stripped", "foo/bar/baz", "baz"},
		{"embedded double dots", "my..repo", "myrepo"},
		{"backslash path", `foo\bar\baz`, "baz"},
		{"name with extension", "repo.git", "repo.git"},
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

func TestIsPartialRemoteRejection(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
		want   bool
	}{
		{
			name:   "remote rejected branches",
			stderr: "To https://github.com/org/repo.git\n ! [remote rejected] main -> main (failed)\nerror: failed to push some refs",
			want:   true,
		},
		{
			name:   "remote rejected with lock error",
			stderr: " ! [remote rejected] main -> main (cannot lock ref 'refs/heads/main': reference already exists)\nerror: failed to push some refs",
			want:   true,
		},
		{
			name:   "fatal auth error",
			stderr: "fatal: Authentication failed for 'https://github.com/org/repo.git'",
			want:   false,
		},
		{
			name:   "remote rejected plus fatal",
			stderr: " ! [remote rejected] main -> main (failed)\nfatal: something went wrong",
			want:   false,
		},
		{
			name:   "fast-forward rejection not remote rejected",
			stderr: " ! [rejected] main -> main (fetch first)\nerror: failed to push some refs",
			want:   false,
		},
		{
			name:   "empty stderr",
			stderr: "",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPartialRemoteRejection(tt.stderr)
			if got != tt.want {
				t.Errorf("isPartialRemoteRejection() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractRejectedRefs(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
		want   []string
	}{
		{
			name: "multiple rejected refs",
			stderr: `To https://github.com/org/repo.git
 ! [remote rejected] feat-branch -> feat-branch (failed)
 ! [remote rejected] main -> main (cannot lock ref)
error: failed to push some refs`,
			want: []string{"feat-branch", "main"},
		},
		{
			name:   "no rejected refs",
			stderr: "Everything up-to-date",
			want:   nil,
		},
		{
			name:   "single rejected ref",
			stderr: " ! [remote rejected] develop -> develop (protected branch hook declined)",
			want:   []string{"develop"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRejectedRefs(tt.stderr)
			if len(got) != len(tt.want) {
				t.Fatalf("extractRejectedRefs() got %d refs, want %d: %v", len(got), len(tt.want), got)
			}
			for i, ref := range got {
				if ref != tt.want[i] {
					t.Errorf("ref[%d] = %q, want %q", i, ref, tt.want[i])
				}
			}
		})
	}
}
