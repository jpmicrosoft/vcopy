package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the YAML configuration file.
type Config struct {
	Source   SourceConfig   `yaml:"source"`
	Target   TargetConfig   `yaml:"target"`
	Auth     AuthConfig     `yaml:"auth"`
	Copy     CopyConfig     `yaml:"copy"`
	Verify   VerifyConfig   `yaml:"verify"`
	Report   ReportConfig   `yaml:"report"`
	Exclude  ExcludeConfig  `yaml:"exclude"`
	LFS            bool           `yaml:"lfs"`
	Force          bool           `yaml:"force"`
	CodeOnly       bool           `yaml:"code_only"`
	NonInteractive bool           `yaml:"non_interactive"`
	Verbose        bool           `yaml:"verbose"`
}

// ExcludeConfig holds path exclusion settings.
type ExcludeConfig struct {
	Workflows bool     `yaml:"workflows"` // exclude .github/workflows
	Actions   bool     `yaml:"actions"`   // exclude .github/actions
	Copilot   bool     `yaml:"copilot"`   // exclude .github/copilot-instructions.md, .github/copilot/
	GitHub    bool     `yaml:"github"`    // exclude entire .github directory
	Paths     []string `yaml:"paths"`     // additional paths to exclude
}

// SourceConfig holds source repository settings.
type SourceConfig struct {
	Repo   string `yaml:"repo"`
	Host   string `yaml:"host"`
	Public bool   `yaml:"public"`
}

// TargetConfig holds target repository settings.
type TargetConfig struct {
	Org        string `yaml:"org"`
	Host       string `yaml:"host"`
	Name       string `yaml:"name"`
	Visibility string `yaml:"visibility"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	Method      string `yaml:"method"`
	SourceToken string `yaml:"source_token"`
	TargetToken string `yaml:"target_token"`
}

// CopyConfig holds copy settings.
type CopyConfig struct {
	Issues       bool `yaml:"issues"`
	PullRequests bool `yaml:"pull_requests"`
	Wiki         bool `yaml:"wiki"`
	AllMetadata  bool `yaml:"all_metadata"`
}

// VerifyConfig holds verification settings.
type VerifyConfig struct {
	Skip       bool   `yaml:"skip"`
	QuickMode  bool   `yaml:"quick"`
	VerifyOnly bool   `yaml:"verify_only"`
	Since      string `yaml:"since"`
}

// ReportConfig holds report settings.
type ReportConfig struct {
	Path   string `yaml:"path"`
	SignKey string `yaml:"sign_key"`
}

// Load reads and parses a YAML config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := &Config{
		Source: SourceConfig{Host: "github.com"},
		Target: TargetConfig{Host: "github.com"},
		Auth:   AuthConfig{Method: "auto"},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if cfg.Source.Repo == "" {
		return nil, fmt.Errorf("config: source.repo is required")
	}
	if cfg.Target.Org == "" {
		return nil, fmt.Errorf("config: target.org is required")
	}

	if cfg.Copy.AllMetadata {
		cfg.Copy.Issues = true
		cfg.Copy.PullRequests = true
		cfg.Copy.Wiki = true
	}

	return cfg, nil
}
