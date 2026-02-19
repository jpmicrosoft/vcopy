package main

import (
	"testing"
)

func TestParseRepo(t *testing.T) {
	tests := []struct {
		input     string
		wantOwner string
		wantName  string
		wantErr   bool
	}{
		{"myorg/myrepo", "myorg", "myrepo", false},
		{"owner/repo-name", "owner", "repo-name", false},
		{"a/b", "a", "b", false},
		{"noslash", "", "", true},
		{"too/many/parts", "", "", true},
		{"", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			owner, name, err := parseRepo(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseRepo(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("parseRepo(%q) unexpected error: %v", tt.input, err)
				return
			}
			if owner != tt.wantOwner {
				t.Errorf("parseRepo(%q) owner = %q, want %q", tt.input, owner, tt.wantOwner)
			}
			if name != tt.wantName {
				t.Errorf("parseRepo(%q) name = %q, want %q", tt.input, name, tt.wantName)
			}
		})
	}
}

func TestSplitRepo(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"a/b", 2},
		{"a/b/c", 3},
		{"noslash", 1},
		{"", 0},
		{"a//b", 2}, // double slash
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			parts := splitRepo(tt.input)
			if len(parts) != tt.want {
				t.Errorf("splitRepo(%q) = %d parts, want %d", tt.input, len(parts), tt.want)
			}
		})
	}
}
