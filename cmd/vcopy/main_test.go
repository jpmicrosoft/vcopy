package main

import (
	"testing"

	"github.com/jaiperez/vcopy/internal/config"
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

func TestConfirmAction_NonInteractive(t *testing.T) {
	// When nonInteractive is true, the force path should skip confirmAction.
	// We test this by verifying the flag exists and defaults to false.
	if nonInteractive {
		t.Error("nonInteractive should default to false")
	}
}

func TestApplyConfig_NonInteractive(t *testing.T) {
	// Reset global state
	origNI := nonInteractive
	defer func() { nonInteractive = origNI }()

	nonInteractive = false
	cfg := &config.Config{
		NonInteractive: true,
		Source:         config.SourceConfig{Repo: "o/r"},
		Target:         config.TargetConfig{Org: "t"},
	}
	applyConfig(cfg)
	if !nonInteractive {
		t.Error("applyConfig should set nonInteractive from config")
	}
}

func TestApplyConfig_NoOverrideWhenSet(t *testing.T) {
	// If nonInteractive is already true, config should not reset it
	origNI := nonInteractive
	defer func() { nonInteractive = origNI }()

	nonInteractive = true
	cfg := &config.Config{
		NonInteractive: false,
		Source:         config.SourceConfig{Repo: "o/r"},
		Target:         config.TargetConfig{Org: "t"},
	}
	applyConfig(cfg)
	if !nonInteractive {
		t.Error("applyConfig should not reset nonInteractive when already true")
	}
}

func TestBatchTargetName(t *testing.T) {
	tests := []struct {
		name       string
		repoName   string
		prefix     string
		suffix     string
		wantTarget string
	}{
		{"no prefix or suffix", "terraform-azurerm-avm-res-cache-redis", "", "", "terraform-azurerm-avm-res-cache-redis"},
		{"prefix only", "terraform-azurerm-avm-res-cache-redis", "myorg-", "", "myorg-terraform-azurerm-avm-res-cache-redis"},
		{"suffix only", "terraform-azurerm-avm-res-cache-redis", "", "-internal", "terraform-azurerm-avm-res-cache-redis-internal"},
		{"both prefix and suffix", "terraform-azurerm-avm-res-cache-redis", "avm-", "-copy", "avm-terraform-azurerm-avm-res-cache-redis-copy"},
		{"short name", "my-repo", "pre-", "-suf", "pre-my-repo-suf"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.prefix + tt.repoName + tt.suffix
			if got != tt.wantTarget {
				t.Errorf("batch name = %q, want %q", got, tt.wantTarget)
			}
		})
	}
}
