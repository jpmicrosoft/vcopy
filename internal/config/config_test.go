package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Valid(t *testing.T) {
	content := `
source:
  repo: myorg/myrepo
  host: github.com
  public: true
target:
  org: target-org
  host: ghes.example.com
  name: mirror-repo
  visibility: internal
auth:
  method: pat
copy:
  issues: true
  wiki: true
  all_metadata: false
verify:
  quick: true
  skip: false
lfs: true
verbose: true
report:
  path: report.json
  sign_key: ABC123
`
	tmpDir, _ := os.MkdirTemp("", "vcopy-cfg-*")
	defer os.RemoveAll(tmpDir)
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(cfgPath, []byte(content), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Source.Repo != "myorg/myrepo" {
		t.Errorf("Source.Repo = %q", cfg.Source.Repo)
	}
	if !cfg.Source.Public {
		t.Error("Source.Public should be true")
	}
	if cfg.Target.Org != "target-org" {
		t.Errorf("Target.Org = %q", cfg.Target.Org)
	}
	if cfg.Target.Host != "ghes.example.com" {
		t.Errorf("Target.Host = %q", cfg.Target.Host)
	}
	if cfg.Target.Name != "mirror-repo" {
		t.Errorf("Target.Name = %q", cfg.Target.Name)
	}
	if cfg.Target.Visibility != "internal" {
		t.Errorf("Target.Visibility = %q, want %q", cfg.Target.Visibility, "internal")
	}
	if cfg.Auth.Method != "pat" {
		t.Errorf("Auth.Method = %q", cfg.Auth.Method)
	}
	if !cfg.Copy.Issues {
		t.Error("Copy.Issues should be true")
	}
	if !cfg.Copy.Wiki {
		t.Error("Copy.Wiki should be true")
	}
	if !cfg.LFS {
		t.Error("LFS should be true")
	}
	if !cfg.Verify.QuickMode {
		t.Error("Verify.QuickMode should be true")
	}
	if cfg.Report.SignKey != "ABC123" {
		t.Errorf("Report.SignKey = %q", cfg.Report.SignKey)
	}
}

func TestLoad_MissingRepo(t *testing.T) {
	content := `
target:
  org: my-org
`
	tmpDir, _ := os.MkdirTemp("", "vcopy-cfg-*")
	defer os.RemoveAll(tmpDir)
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(cfgPath, []byte(content), 0644)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for missing source.repo")
	}
}

func TestLoad_MissingOrg(t *testing.T) {
	content := `
source:
  repo: myorg/myrepo
`
	tmpDir, _ := os.MkdirTemp("", "vcopy-cfg-*")
	defer os.RemoveAll(tmpDir)
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(cfgPath, []byte(content), 0644)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for missing target.org")
	}
}

func TestLoad_AllMetadata(t *testing.T) {
	content := `
source:
  repo: a/b
target:
  org: c
copy:
  all_metadata: true
`
	tmpDir, _ := os.MkdirTemp("", "vcopy-cfg-*")
	defer os.RemoveAll(tmpDir)
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(cfgPath, []byte(content), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !cfg.Copy.Issues || !cfg.Copy.PullRequests || !cfg.Copy.Wiki {
		t.Error("all_metadata should set all copy flags to true")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_ExcludeConfig(t *testing.T) {
	content := `
source:
  repo: a/b
target:
  org: c
exclude:
  workflows: true
  copilot: true
  paths:
    - vendor
    - docs/internal
`
	tmpDir, _ := os.MkdirTemp("", "vcopy-cfg-*")
	defer os.RemoveAll(tmpDir)
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(cfgPath, []byte(content), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !cfg.Exclude.Workflows {
		t.Error("Exclude.Workflows should be true")
	}
	if !cfg.Exclude.Copilot {
		t.Error("Exclude.Copilot should be true")
	}
	if len(cfg.Exclude.Paths) != 2 {
		t.Errorf("expected 2 exclude paths, got %d", len(cfg.Exclude.Paths))
	}
}
