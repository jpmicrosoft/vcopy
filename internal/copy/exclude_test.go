package copy

import (
	"testing"
)

func TestBuildExcludePaths_Presets(t *testing.T) {
	// Both workflow + copilot presets — workflows(1) + copilot(2) + codeowners(3 always)
	paths, err := BuildExcludePaths(true, false, true, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := map[string]bool{
		".github/workflows":              true,
		".github/copilot-instructions.md": true,
		".github/copilot":                true,
		"CODEOWNERS":                     true,
		".github/CODEOWNERS":             true,
		"docs/CODEOWNERS":                true,
	}
	if len(paths) != len(expected) {
		t.Fatalf("expected %d paths, got %d: %v", len(expected), len(paths), paths)
	}
	for _, p := range paths {
		if !expected[p] {
			t.Errorf("unexpected path: %q", p)
		}
	}
}

func TestBuildExcludePaths_NoPresets(t *testing.T) {
	// No presets — still gets CODEOWNERS (3 always-excluded paths)
	paths, err := BuildExcludePaths(false, false, false, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 3 {
		t.Errorf("expected 3 paths (CODEOWNERS always), got %d: %v", len(paths), paths)
	}
}

func TestBuildExcludePaths_Custom(t *testing.T) {
	// 2 custom + 3 CODEOWNERS = 5
	paths, err := BuildExcludePaths(false, false, false, false, []string{"docs/internal", "scripts/deploy.sh"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 5 {
		t.Fatalf("expected 5 paths, got %d: %v", len(paths), paths)
	}
}

func TestBuildExcludePaths_Dedup(t *testing.T) {
	// Preset + custom that overlaps — workflows(1) + 1 overlap + CODEOWNERS(3) = 4
	paths, err := BuildExcludePaths(true, false, false, false, []string{".github/workflows"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 4 {
		t.Errorf("expected 4 paths (deduped), got %d: %v", len(paths), paths)
	}
}

func TestBuildExcludePaths_Mixed(t *testing.T) {
	// workflows(1) + actions(1) + copilot(2) + custom(2) + CODEOWNERS(3) = 9
	paths, err := BuildExcludePaths(true, true, true, false, []string{"vendor", "docs/secret"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 9 {
		t.Fatalf("expected 9 paths, got %d: %v", len(paths), paths)
	}
}

func TestBuildExcludePaths_NoGitHub(t *testing.T) {
	// --no-github supersedes individual flags — .github(1) + CODEOWNERS(3) = 4
	paths, err := BuildExcludePaths(true, true, true, true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := map[string]bool{
		".github":            true,
		"CODEOWNERS":         true,
		".github/CODEOWNERS": true,
		"docs/CODEOWNERS":    true,
	}
	if len(paths) != len(expected) {
		t.Fatalf("expected %d paths, got %d: %v", len(expected), len(paths), paths)
	}
	for _, p := range paths {
		if !expected[p] {
			t.Errorf("unexpected path: %q", p)
		}
	}
}

func TestBuildExcludePaths_NoActions(t *testing.T) {
	// --no-actions only — actions(1) + CODEOWNERS(3) = 4
	paths, err := BuildExcludePaths(false, true, false, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 4 {
		t.Fatalf("expected 4 paths, got %d: %v", len(paths), paths)
	}
}

func TestSanitizeExcludePath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"normal path", "docs/internal", "docs/internal", false},
		{"dotfile", ".github/workflows", ".github/workflows", false},
		{"trailing slash stripped", "vendor/", "vendor", false},
		{"leading dot-slash stripped", "./src/test", "src/test", false},
		{"backslash normalised", `docs\internal`, "docs/internal", false},
		{"empty", "", "", true},
		{"spaces only", "   ", "", true},
		{"absolute path", "/etc/passwd", "", true},
		{"traversal", "../../../etc", "", true},
		{"embedded traversal", "foo/../bar", "", true},
		{"flag injection", "--upload-pack=evil", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sanitizeExcludePath(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("sanitizeExcludePath(%q) expected error, got %q", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("sanitizeExcludePath(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got != tt.want {
				t.Errorf("sanitizeExcludePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildExcludePaths_InvalidPath(t *testing.T) {
	_, err := BuildExcludePaths(false, false, false, false, []string{"valid/path", "../traversal"})
	if err == nil {
		t.Error("expected error for traversal path, got nil")
	}
}
