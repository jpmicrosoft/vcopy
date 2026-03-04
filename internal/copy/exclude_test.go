package copy

import (
	"testing"
)

func TestBuildExcludePaths_Presets(t *testing.T) {
	// Both presets enabled
	paths, err := BuildExcludePaths(true, true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should include workflow + copilot paths
	if len(paths) != 3 {
		t.Fatalf("expected 3 paths, got %d: %v", len(paths), paths)
	}
	expected := map[string]bool{
		".github/workflows":              true,
		".github/copilot-instructions.md": true,
		".github/copilot":                true,
	}
	for _, p := range paths {
		if !expected[p] {
			t.Errorf("unexpected path: %q", p)
		}
	}
}

func TestBuildExcludePaths_NoPresets(t *testing.T) {
	paths, err := BuildExcludePaths(false, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected 0 paths, got %d", len(paths))
	}
}

func TestBuildExcludePaths_Custom(t *testing.T) {
	paths, err := BuildExcludePaths(false, false, []string{"docs/internal", "scripts/deploy.sh"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d: %v", len(paths), paths)
	}
}

func TestBuildExcludePaths_Dedup(t *testing.T) {
	// Preset + custom that overlaps
	paths, err := BuildExcludePaths(true, false, []string{".github/workflows"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 1 {
		t.Errorf("expected 1 path (deduped), got %d: %v", len(paths), paths)
	}
}

func TestBuildExcludePaths_Mixed(t *testing.T) {
	paths, err := BuildExcludePaths(true, true, []string{"vendor", "docs/secret"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 5 {
		t.Fatalf("expected 5 paths, got %d: %v", len(paths), paths)
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
	_, err := BuildExcludePaths(false, false, []string{"valid/path", "../traversal"})
	if err == nil {
		t.Error("expected error for traversal path, got nil")
	}
}
