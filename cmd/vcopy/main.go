package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jpmicrosoft/vcopy/internal/auth"
	"github.com/jpmicrosoft/vcopy/internal/config"
	vcopy "github.com/jpmicrosoft/vcopy/internal/copy"
	ghclient "github.com/jpmicrosoft/vcopy/internal/github"
	"github.com/jpmicrosoft/vcopy/internal/report"
	"github.com/jpmicrosoft/vcopy/internal/verify"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// version is set at build time via -ldflags "-X main.version=..."
var version = "dev"

var (
	sourceHost       string
	targetHost       string
	targetName       string
	targetVisibility string
	authMethod       string
	sourceToken      string
	targetToken      string
	publicSource     bool
	lfs              bool
	force            bool
	codeOnly         bool
	copyIssues       bool
	copyPRs          bool
	copyWiki         bool
	allMetadata      bool
	verifyOnly       bool
	skipVerify       bool
	quickVerify      bool
	since            string
	reportPath       string
	signKey          string
	configPath       string
	noWorkflows      bool
	noCopilot        bool
	noActions        bool
	noGitHub         bool
	excludePaths     []string
	// batch subcommand flags
	batchSearch        string
	batchPrefix        string
	batchSuffix        string
	batchSkipExist     bool
	batchSync          bool
	batchPerRepoReport bool
	batchDelay         time.Duration
	verbose            bool
	dryRun             bool
	nonInteractive     bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "vcopy <source-repo> <target-org>",
		Version: version,
		Short:   "Verified GitHub repository copy tool",
		Long: `vcopy copies a GitHub repository to a target organization (Cloud or Enterprise)
with comprehensive integrity verification to ensure the copy has not been tampered with.

It performs a bare mirror clone of all branches, tags, and history, then runs a
5-layer verification suite: object hashes, ref comparison, tree hashes, commit
signatures, and git bundle integrity.`,
		Args: cobra.RangeArgs(0, 2),
		RunE: run,
	}

	f := rootCmd.Flags()
	f.StringVar(&configPath, "config", "", "Path to YAML config file")
	f.StringVar(&sourceHost, "source-host", "github.com", "Source GitHub host")
	f.StringVar(&targetHost, "target-host", "github.com", "Target GitHub host")
	f.StringVar(&targetName, "target-name", "", "Target repo name (default: same as source)")
	f.StringVar(&authMethod, "auth-method", "auto", "Auth method: auto, gh, pat")
	f.StringVar(&sourceToken, "source-token", "", "PAT for source (if auth-method=pat)")
	f.StringVar(&targetToken, "target-token", "", "PAT for target (if auth-method=pat)")
	f.BoolVar(&publicSource, "public-source", false, "Source repo is public (skip source authentication)")
	f.BoolVar(&publicSource, "public", false, "Source repo is public (skip source authentication)")
	_ = f.MarkDeprecated("public", "use --public-source instead")
	f.StringVar(&targetVisibility, "visibility", "private", "Target repo visibility: private, public, or internal")
	f.BoolVar(&lfs, "lfs", false, "Include Git LFS objects in copy")
	f.BoolVar(&force, "force", false, "Force push to existing target repo (WARNING: overwrites all branches/tags)")
	f.BoolVar(&codeOnly, "code-only", false, "Copy source code only (branches/commits, no tags, releases, or metadata)")
	f.BoolVar(&copyIssues, "issues", false, "Copy issues")
	f.BoolVar(&copyPRs, "pull-requests", false, "Copy pull requests")
	f.BoolVar(&copyWiki, "wiki", false, "Copy wiki")
	f.BoolVar(&allMetadata, "all-metadata", false, "Copy all metadata (issues, PRs, wiki)")
	f.BoolVar(&verifyOnly, "verify-only", false, "Skip copy, only verify existing source vs target")
	f.BoolVar(&skipVerify, "skip-verify", false, "Skip verification (copy only)")
	f.BoolVar(&quickVerify, "quick-verify", false, "Quick verification (refs + tree hashes only)")
	f.StringVar(&since, "since", "", "Incremental verify: only check objects after this SHA or date")
	f.StringVar(&reportPath, "report", "", "Path to write JSON verification report")
	f.StringVar(&signKey, "sign", "", "GPG key ID to sign the verification report (Attestation Signature)")
	f.BoolVar(&verbose, "verbose", false, "Verbose output")
	f.BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")
	f.BoolVar(&nonInteractive, "non-interactive", false, "Skip confirmation prompts (for CI/CD and automation)")
	f.BoolVar(&noWorkflows, "no-workflows", false, "Exclude GitHub Actions workflows (.github/workflows/) from the target")
	f.BoolVar(&noActions, "no-actions", false, "Exclude GitHub Actions custom actions (.github/actions/) from the target")
	f.BoolVar(&noCopilot, "no-copilot", false, "Exclude Copilot instructions and skills (.github/copilot-instructions.md, .github/copilot/) from the target")
	f.BoolVar(&noGitHub, "no-github", false, "Exclude entire .github/ directory from the target (supersedes --no-workflows, --no-actions, --no-copilot)")
	f.StringSliceVar(&excludePaths, "exclude", nil, "Additional paths to exclude from the target (comma-separated or repeated)")

	// Batch subcommand
	batchCmd := &cobra.Command{
		Use:   "batch <source-org> <target-org>",
		Short: "Batch copy repos matching a search filter from source org to target org",
		Long: `Discover repositories in the source organization matching a name filter,
then copy each one to the target organization. Supports prefix/suffix renaming,
skip-existing for resumable runs, and all standard vcopy flags.

Example:
  vcopy batch Azure jpmicrosoft --search "terraform-azurerm-avm-" --public-source --no-github --dry-run`,
		Args: cobra.ExactArgs(2),
		RunE: batchRun,
	}
	bf := batchCmd.Flags()
	bf.StringVar(&batchSearch, "search", "", "Repository name filter (required, matched against repo name)")
	bf.StringVar(&batchPrefix, "prefix", "", "Prefix to prepend to each target repo name")
	bf.StringVar(&batchSuffix, "suffix", "", "Suffix to append to each target repo name")
	bf.BoolVar(&batchSkipExist, "skip-existing", false, "Skip repos that already exist in the target org (useful for resuming)")
	bf.BoolVar(&batchSync, "sync", false, "Update existing repos (additive push + incremental release sync) instead of skipping them")
	bf.StringVar(&reportPath, "report", "", "Path to write combined JSON batch report")
	bf.BoolVar(&batchPerRepoReport, "per-repo-report", false, "Also write individual JSON reports per repo (e.g., report-reponame.json)")
	bf.DurationVar(&batchDelay, "batch-delay", 3*time.Second, "Delay between repos to avoid secondary rate limits (e.g., 3s, 5s, 0 to disable)")
	batchCmd.MarkFlagRequired("search")
	// Shared flags inherited by batch
	bf.StringVar(&sourceHost, "source-host", "github.com", "Source GitHub host")
	bf.StringVar(&targetHost, "target-host", "github.com", "Target GitHub host")
	bf.StringVar(&authMethod, "auth-method", "auto", "Auth method: auto, gh, pat")
	bf.StringVar(&sourceToken, "source-token", "", "PAT for source (if auth-method=pat)")
	bf.StringVar(&targetToken, "target-token", "", "PAT for target (if auth-method=pat)")
	bf.BoolVar(&publicSource, "public-source", false, "Source repos are public (skip source authentication)")
	bf.BoolVar(&publicSource, "public", false, "Source repos are public (skip source authentication)")
	_ = bf.MarkDeprecated("public", "use --public-source instead")
	bf.StringVar(&targetVisibility, "visibility", "private", "Target repo visibility: private, public, or internal")
	bf.BoolVar(&lfs, "lfs", false, "Include Git LFS objects in copy")
	bf.BoolVar(&codeOnly, "code-only", false, "Copy source code only (branches/commits, no tags, releases, or metadata)")
	bf.BoolVar(&skipVerify, "skip-verify", false, "Skip verification (copy only)")
	bf.BoolVar(&quickVerify, "quick-verify", false, "Quick verification (refs + tree hashes only)")
	bf.BoolVar(&verbose, "verbose", false, "Verbose output")
	bf.BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")
	bf.BoolVar(&nonInteractive, "non-interactive", false, "Skip confirmation prompts")
	bf.BoolVar(&noWorkflows, "no-workflows", false, "Exclude GitHub Actions workflows (.github/workflows/) from target")
	bf.BoolVar(&noActions, "no-actions", false, "Exclude GitHub Actions custom actions (.github/actions/) from target")
	bf.BoolVar(&noCopilot, "no-copilot", false, "Exclude Copilot instructions/skills from target")
	bf.BoolVar(&noGitHub, "no-github", false, "Exclude entire .github/ directory from target")
	bf.StringSliceVar(&excludePaths, "exclude", nil, "Additional paths to exclude from target")
	rootCmd.AddCommand(batchCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	sourceRepo, targetOrg, repoName, err := resolveRunArgs(cmd, args)
	if err != nil {
		return err
	}

	if dryRun {
		printRunDryRun(sourceRepo, targetOrg, repoName)
		return nil
	}

	srcToken, tgtToken, err := authenticate()
	if err != nil {
		return err
	}

	srcClient, tgtClient, srcOwner, srcName, err := buildRunClients(srcToken, tgtToken, sourceRepo)
	if err != nil {
		return err
	}

	var rejectedRefs []string
	if !verifyOnly {
		rejectedRefs, err = performRepoCopy(srcClient, tgtClient, srcOwner, srcName, targetOrg, repoName, srcToken, tgtToken)
		if err != nil {
			return err
		}
	}

	if !skipVerify {
		if err := runAndReportVerification(srcOwner, srcName, targetOrg, repoName, srcToken, tgtToken, rejectedRefs); err != nil {
			return err
		}
	} else {
		fmt.Println("\nCopy complete (verification skipped).")
	}

	return handleExcludePaths(targetOrg, repoName, tgtToken)
}

// resolveRunArgs loads config and resolves source repo, target org, and repo name.
func resolveRunArgs(cmd *cobra.Command, args []string) (sourceRepo, targetOrg, repoName string, err error) {
	var cfg *config.Config
	if configPath != "" {
		cfg, err = config.Load(configPath)
		if err != nil {
			return "", "", "", fmt.Errorf("config error: %w", err)
		}
		applyConfig(cfg, cmd.Flags())
	}

	if len(args) >= 2 {
		sourceRepo = args[0]
		targetOrg = args[1]
	} else if cfg != nil {
		sourceRepo = cfg.Source.Repo
		targetOrg = cfg.Target.Org
	} else {
		return "", "", "", fmt.Errorf("requires 2 arguments: <source-repo> <target-org> (or use --config)")
	}

	if allMetadata {
		copyIssues = true
		copyPRs = true
		copyWiki = true
	}

	if verifyOnly && skipVerify {
		return "", "", "", fmt.Errorf("cannot use --verify-only and --skip-verify together")
	}

	if err := validateVisibility(targetVisibility); err != nil {
		return "", "", "", err
	}

	repoName = targetName
	if repoName == "" {
		_, name, parseErr := parseRepo(sourceRepo)
		if parseErr != nil {
			return "", "", "", parseErr
		}
		repoName = name
	}

	return sourceRepo, targetOrg, repoName, nil
}

// printRunDryRun prints the single-repo dry run plan.
func printRunDryRun(sourceRepo, targetOrg, repoName string) {
	fmt.Println("=== DRY RUN ===")
	fmt.Printf("Source:      %s/%s (public: %v)\n", sourceHost, sourceRepo, publicSource)
	fmt.Printf("Target:      %s/%s/%s (visibility: %s)\n", targetHost, targetOrg, repoName, targetVisibility)
	fmt.Printf("LFS:         %v\n", lfs)
	fmt.Printf("Metadata:    issues=%v, PRs=%v, wiki=%v\n", copyIssues, copyPRs, copyWiki)
	fmt.Printf("Code only:   %v\n", codeOnly)
	fmt.Printf("Exclude:     no-workflows=%v, no-actions=%v, no-copilot=%v, no-github=%v, paths=%v\n", noWorkflows, noActions, noCopilot, noGitHub, excludePaths)
	fmt.Printf("Verify:      skip=%v, quick=%v, only=%v, since=%q\n", skipVerify, quickVerify, verifyOnly, since)
	fmt.Printf("Report:      %s\n", reportPath)
	fmt.Printf("Attestation: %s\n", signKey)
}

// buildRunClients creates source and target API clients and parses the source repo.
func buildRunClients(srcToken, tgtToken, sourceRepo string) (*ghclient.Client, *ghclient.Client, string, string, error) {
	srcClient, err := ghclient.NewClient(sourceHost, srcToken)
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("failed to create source client: %w", err)
	}
	tgtClient, err := ghclient.NewClient(targetHost, tgtToken)
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("failed to create target client: %w", err)
	}
	srcOwner, srcName, err := parseRepo(sourceRepo)
	if err != nil {
		return nil, nil, "", "", err
	}
	return srcClient, tgtClient, srcOwner, srcName, nil
}

// performRepoCopy handles repo creation, mirroring, releases, and metadata for single-repo mode.
func performRepoCopy(srcClient, tgtClient *ghclient.Client, srcOwner, srcName, targetOrg, repoName, srcToken, tgtToken string) ([]string, error) {
	fmt.Printf("Creating target repository %s/%s on %s (visibility: %s)...\n", targetOrg, repoName, targetHost, targetVisibility)

	exists, existErr := tgtClient.RepoExists(targetOrg, repoName)
	if existErr != nil {
		if !force {
			return nil, fmt.Errorf("cannot verify if target repo exists: %w\n  Use --force to bypass this check", existErr)
		}
		fmt.Printf("  Warning: could not check if target repo exists: %v\n", existErr)
		fmt.Println("  Proceeding because --force was specified.")
	}

	forceOverwrite := false
	if exists {
		if force {
			fmt.Println()
			fmt.Printf("  ⚠️  WARNING: Target repository %s/%s already exists on %s.\n", targetOrg, repoName, targetHost)
			fmt.Println("  --force mode: a mirror push will OVERWRITE all branches, tags, and history.")
			fmt.Println("  Any content in the target that does not exist in the source WILL BE PERMANENTLY LOST.")
			fmt.Println()
			if !nonInteractive && !confirmAction("Do you want to continue and overwrite the existing repository?") {
				fmt.Println("Aborted.")
				return nil, nil
			}
			fmt.Println()
			forceOverwrite = true
		} else {
			fmt.Printf("  Repository %s/%s already exists. Using additive mode (existing tags and releases preserved).\n", targetOrg, repoName)
		}
	} else {
		if err := tgtClient.CreateRepo(targetOrg, repoName, targetVisibility, verbose); err != nil {
			return nil, fmt.Errorf("failed to create target repo: %w", err)
		}
	}

	fmt.Printf("Mirroring %s/%s from %s to %s/%s on %s...\n", srcOwner, srcName, sourceHost, targetOrg, repoName, targetHost)
	rejectedRefs, err := vcopy.MirrorRepo(sourceHost, srcOwner, srcName, targetHost, targetOrg, repoName, srcToken, tgtToken, lfs, forceOverwrite, codeOnly, verbose)
	if err != nil {
		return nil, fmt.Errorf("mirror failed: %w", err)
	}

	if !codeOnly {
		syncReleasesToTarget(srcClient, tgtClient, srcOwner, srcName, targetOrg, repoName, exists, forceOverwrite)
	} else if verbose {
		fmt.Println("  Skipping tags and releases (--code-only mode)")
	}

	if err := copyMetadata(srcClient, tgtClient, srcOwner, srcName, targetOrg, repoName, srcToken, tgtToken); err != nil {
		return rejectedRefs, err
	}

	return rejectedRefs, nil
}

// copyMetadata copies issues, PRs, and wiki as requested by flags.
func copyMetadata(srcClient, tgtClient *ghclient.Client, srcOwner, srcName, targetOrg, repoName, srcToken, tgtToken string) error {
	if copyIssues {
		fmt.Println("Copying issues...")
		if err := vcopy.CopyIssues(srcClient, tgtClient, srcOwner, srcName, targetOrg, repoName, verbose); err != nil {
			return fmt.Errorf("issue copy failed: %w", err)
		}
	}
	if copyPRs {
		fmt.Println("Copying pull requests...")
		if err := vcopy.CopyPullRequests(srcClient, tgtClient, srcOwner, srcName, targetOrg, repoName, verbose); err != nil {
			return fmt.Errorf("PR copy failed: %w", err)
		}
	}
	if copyWiki {
		fmt.Println("Copying wiki...")
		if err := vcopy.CopyWiki(sourceHost, srcOwner, srcName, targetHost, targetOrg, repoName, srcToken, tgtToken, verbose); err != nil {
			fmt.Printf("Warning: wiki copy failed (wiki may not exist): %v\n", err)
		}
	}
	return nil
}

// handleExcludePaths removes excluded paths from the target repo.
func handleExcludePaths(targetOrg, repoName, tgtToken string) error {
	excludeList, err := vcopy.BuildExcludePaths(noWorkflows, noActions, noCopilot, noGitHub, excludePaths)
	if err != nil {
		return fmt.Errorf("invalid exclude paths: %w", err)
	}
	if len(excludeList) > 0 && !verifyOnly {
		if err := vcopy.CleanupExcludedPaths(targetHost, targetOrg, repoName, tgtToken, excludeList, verbose); err != nil {
			return fmt.Errorf("exclude cleanup failed: %w", err)
		}
	}
	return nil
}

// applyConfig applies all sections of a parsed YAML config to the global flags.
// CLI flags take precedence — config values are only used when the flag was not set.
func applyConfig(cfg *config.Config, flags *pflag.FlagSet) {
	applyHostConfig(cfg, flags)
	applyAuthConfig(cfg)
	applyCopyConfig(cfg)
	applyVerifyConfig(cfg)
	applyExcludeConfig(cfg)
}

// applyHostConfig sets source/target host, target name, visibility, and public-source
// from config when the corresponding CLI flags are still at their defaults.
func applyHostConfig(cfg *config.Config, flags *pflag.FlagSet) {
	if sourceHost == "github.com" && cfg.Source.Host != "" {
		sourceHost = cfg.Source.Host
	}
	if targetHost == "github.com" && cfg.Target.Host != "" {
		targetHost = cfg.Target.Host
	}
	if targetName == "" && cfg.Target.Name != "" {
		targetName = cfg.Target.Name
	}
	if (flags == nil || !flags.Changed("visibility")) && cfg.Target.Visibility != "" {
		targetVisibility = cfg.Target.Visibility
	}
	if !publicSource {
		publicSource = cfg.Source.Public
	}
}

// applyAuthConfig sets auth method and tokens from config when not already provided via CLI flags.
func applyAuthConfig(cfg *config.Config) {
	if authMethod == "auto" && cfg.Auth.Method != "" {
		authMethod = cfg.Auth.Method
	}
	if sourceToken == "" && cfg.Auth.SourceToken != "" {
		sourceToken = cfg.Auth.SourceToken
	}
	if targetToken == "" && cfg.Auth.TargetToken != "" {
		targetToken = cfg.Auth.TargetToken
	}
}

// applyCopyConfig sets LFS, force, code-only, metadata, verbose, and non-interactive
// flags from config when the corresponding CLI flags are still at their defaults.
func applyCopyConfig(cfg *config.Config) {
	if !lfs {
		lfs = cfg.LFS
	}
	if !force {
		force = cfg.Force
	}
	if !codeOnly {
		codeOnly = cfg.CodeOnly
	}
	if !copyIssues {
		copyIssues = cfg.Copy.Issues
	}
	if !copyPRs {
		copyPRs = cfg.Copy.PullRequests
	}
	if !copyWiki {
		copyWiki = cfg.Copy.Wiki
	}
	if !allMetadata {
		allMetadata = cfg.Copy.AllMetadata
	}
	if !verbose {
		verbose = cfg.Verbose
	}
	if !nonInteractive {
		nonInteractive = cfg.NonInteractive
	}
}

// applyVerifyConfig sets skip-verify, quick-verify, verify-only, since, report path,
// and sign key from config when the corresponding CLI flags are still at their defaults.
func applyVerifyConfig(cfg *config.Config) {
	if !skipVerify {
		skipVerify = cfg.Verify.Skip
	}
	if !quickVerify {
		quickVerify = cfg.Verify.QuickMode
	}
	if !verifyOnly {
		verifyOnly = cfg.Verify.VerifyOnly
	}
	if since == "" {
		since = cfg.Verify.Since
	}
	if reportPath == "" && cfg.Report.Path != "" {
		reportPath = cfg.Report.Path
	}
	if signKey == "" && cfg.Report.SignKey != "" {
		signKey = cfg.Report.SignKey
	}
}

// applyExcludeConfig sets no-workflows, no-actions, no-copilot, no-github, and
// custom exclude paths from config when the corresponding CLI flags are still at their defaults.
func applyExcludeConfig(cfg *config.Config) {
	if !noWorkflows {
		noWorkflows = cfg.Exclude.Workflows
	}
	if !noActions {
		noActions = cfg.Exclude.Actions
	}
	if !noCopilot {
		noCopilot = cfg.Exclude.Copilot
	}
	if !noGitHub {
		noGitHub = cfg.Exclude.GitHub
	}
	if len(excludePaths) == 0 && len(cfg.Exclude.Paths) > 0 {
		excludePaths = cfg.Exclude.Paths
	}
}

func parseRepo(repo string) (owner, name string, err error) {
	parts := splitRepo(repo)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repo format %q: expected owner/repo", repo)
	}
	return parts[0], parts[1], nil
}

func splitRepo(repo string) []string {
	var parts []string
	current := ""
	for _, c := range repo {
		if c == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// confirmAction prompts the user with a yes/no question and returns true only if they type "yes" or "y".
func confirmAction(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("  %s [yes/no]: ", prompt)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "yes" || answer == "y"
}

// authenticate handles source and target authentication for both public and private modes.
func authenticate() (string, string, error) {
	if publicSource {
		fmt.Println("Public source mode: skipping source authentication")
		var srcTok string
		if sourceToken != "" {
			srcTok = sourceToken
			fmt.Println("  Using --source-token for authenticated access (5,000 req/hr)")
		}
		if (copyIssues || copyPRs) && srcTok == "" {
			fmt.Println("  Warning: --source-token recommended for metadata copy (unauthenticated rate limit is 60 req/hr)")
		}
		tgtTok, err := auth.AuthenticateTarget(authMethod, targetHost, targetToken)
		if err != nil {
			return "", "", fmt.Errorf("target authentication failed: %w", err)
		}
		return srcTok, tgtTok, nil
	}
	srcTok, tgtTok, err := auth.Authenticate(authMethod, sourceHost, targetHost, sourceToken, targetToken)
	if err != nil {
		return "", "", fmt.Errorf("authentication failed: %w", err)
	}
	return srcTok, tgtTok, nil
}

// syncReleasesToTarget handles the three release-copy strategies for single-repo mode.
func syncReleasesToTarget(srcClient, tgtClient *ghclient.Client, srcOwner, srcName, targetOrg, repoName string, exists, forceOverwrite bool) {
	if exists && !forceOverwrite {
		fmt.Printf("Syncing releases (additive — new releases only)...\n")
		if err := vcopy.SyncReleases(srcClient, tgtClient, srcOwner, srcName, targetOrg, repoName, verbose); err != nil {
			fmt.Printf("Warning: release sync failed: %v\n", err)
		}
	} else if exists && forceOverwrite {
		fmt.Println("Cleaning target releases (force mode)...")
		if err := vcopy.CleanTargetReleases(srcClient, tgtClient, srcOwner, srcName, targetOrg, repoName, verbose); err != nil {
			fmt.Printf("Warning: failed to clean target releases: %v\n", err)
		}
		fmt.Println("Copying releases...")
		if err := vcopy.CopyReleases(srcClient, tgtClient, srcOwner, srcName, targetOrg, repoName, verbose); err != nil {
			fmt.Printf("Warning: release copy failed: %v\n", err)
		}
	} else {
		fmt.Println("Copying releases...")
		if err := vcopy.CopyReleases(srcClient, tgtClient, srcOwner, srcName, targetOrg, repoName, verbose); err != nil {
			fmt.Printf("Warning: release copy failed: %v\n", err)
		}
	}
}

// runAndReportVerification runs verification, optionally signs and writes the report.
func runAndReportVerification(srcOwner, srcName, targetOrg, repoName, srcToken, tgtToken string, rejectedRefs []string) error {
	fmt.Println("\n=== Running Integrity Verification ===")

	opts := verify.Options{
		QuickMode:    quickVerify,
		CodeOnly:     codeOnly,
		Verbose:      verbose,
		ExcludedRefs: rejectedRefs,
	}

	var results *verify.VerificationReport
	var err error
	if since != "" {
		results, err = verify.RunIncremental(sourceHost, srcOwner, srcName, targetHost, targetOrg, repoName, srcToken, tgtToken, since, opts)
	} else {
		results, err = verify.RunAll(sourceHost, srcOwner, srcName, targetHost, targetOrg, repoName, srcToken, tgtToken, opts)
	}
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	if signKey != "" {
		fmt.Printf("Signing verification report with key %s...\n", signKey)
		if err := report.SignReport(results, signKey); err != nil {
			return fmt.Errorf("attestation signature failed: %w", err)
		}
		fmt.Println("Attestation Signature applied")
	}

	report.PrintTerminal(results)

	if reportPath != "" {
		if err := report.WriteJSON(results, reportPath); err != nil {
			return fmt.Errorf("failed to write report: %w", err)
		}
		fmt.Printf("\nJSON report written to: %s\n", reportPath)
	}

	if !results.AllPassed() {
		return fmt.Errorf("VERIFICATION FAILED: one or more checks did not pass")
	}

	fmt.Println("\nAll verification checks passed. Repository copy is verified.")
	return nil
}

// printBatchDryRun prints the batch plan without executing.
func printBatchDryRun(sourceOrg, targetOrg string, repos []string) {
	mode := "Copy"
	verb := "copied"
	if batchSync {
		mode = "Sync"
		verb = "synced"
	}
	fmt.Printf("=== DRY RUN — Batch %s Plan ===\n", mode)
	fmt.Printf("Source org:  %s (%s)\n", sourceOrg, sourceHost)
	fmt.Printf("Target org:  %s (%s)\n", targetOrg, targetHost)
	fmt.Printf("Name format: %s{name}%s\n", batchPrefix, batchSuffix)
	fmt.Printf("Flags:       no-github=%v, no-workflows=%v, no-actions=%v, no-copilot=%v, skip-verify=%v, code-only=%v, sync=%v\n",
		noGitHub, noWorkflows, noActions, noCopilot, skipVerify, codeOnly, batchSync)
	fmt.Println()
	for i, name := range repos {
		targetRepoName := batchPrefix + name + batchSuffix
		fmt.Printf("  [%d/%d] %s/%s → %s/%s\n", i+1, len(repos), sourceOrg, name, targetOrg, targetRepoName)
	}
	fmt.Printf("\nTotal: %d repositories would be %s.\n", len(repos), verb)
}

// batchState tracks counters and results across the batch loop.
type batchState struct {
	succeeded, failed, skipped, releasesSkipped int
	failures                                    []string
	repoResults                                 []report.BatchRepoResult
	rateLimitHit                                bool
}

// batchRun implements the `vcopy batch` subcommand.
func batchRun(cmd *cobra.Command, args []string) error {
	sourceOrg := args[0]
	targetOrg := args[1]

	if batchSkipExist && batchSync {
		return fmt.Errorf("--skip-existing and --sync are mutually exclusive")
	}

	if err := validateVisibility(targetVisibility); err != nil {
		return err
	}

	srcToken, tgtToken, err := authenticate()
	if err != nil {
		return err
	}

	repos, err := discoverBatchRepos(srcToken, sourceOrg)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		fmt.Println("No repositories found matching the search filter.")
		return nil
	}
	fmt.Printf("Found %d repositories\n\n", len(repos))

	if dryRun {
		printBatchDryRun(sourceOrg, targetOrg, repos)
		return nil
	}

	srcClient, tgtClient, excludeList, err := initBatchClients(srcToken, tgtToken)
	if err != nil {
		return err
	}

	state := &batchState{}
	for i, name := range repos {
		if i > 0 && batchDelay > 0 {
			time.Sleep(batchDelay)
		}
		processBatchRepo(srcClient, tgtClient, sourceOrg, name, targetOrg, srcToken, tgtToken, excludeList, repos, i, state)
	}

	return writeBatchReportAndSummary(sourceOrg, targetOrg, repos, state)
}

// discoverBatchRepos creates a search client and finds matching repos.
func discoverBatchRepos(srcToken, sourceOrg string) ([]string, error) {
	searchClient, err := ghclient.NewClient(sourceHost, srcToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create source client: %w", err)
	}
	fmt.Printf("Searching for repos matching %q in %s...\n", batchSearch, sourceOrg)
	repos, err := searchClient.SearchRepos(sourceOrg, batchSearch)
	if err != nil {
		return nil, fmt.Errorf("repo search failed: %w", err)
	}
	return repos, nil
}

// initBatchClients creates source/target API clients and the exclude list for batch operations.
func initBatchClients(srcToken, tgtToken string) (*ghclient.Client, *ghclient.Client, []string, error) {
	tgtClient, err := ghclient.NewClient(targetHost, tgtToken)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create target client: %w", err)
	}
	excludeList, err := vcopy.BuildExcludePaths(noWorkflows, noActions, noCopilot, noGitHub, excludePaths)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid exclude paths: %w", err)
	}
	srcClient, err := ghclient.NewClient(sourceHost, srcToken)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create source client: %w", err)
	}
	return srcClient, tgtClient, excludeList, nil
}

// processBatchRepo handles a single repo in the batch loop: skip-existing, copy with retry, and result tracking.
func processBatchRepo(srcClient, tgtClient *ghclient.Client, sourceOrg, name, targetOrg, srcToken, tgtToken string, excludeList []string, repos []string, i int, state *batchState) {
	targetRepoName := batchPrefix + name + batchSuffix
	sourceRepo := sourceOrg + "/" + name
	prefix := fmt.Sprintf("[%d/%d]", i+1, len(repos))

	if batchSkipExist {
		exists, existErr := tgtClient.RepoExists(targetOrg, targetRepoName)
		if existErr != nil {
			fmt.Printf("%s ⚠ Warning: could not check if %s/%s exists: %v\n", prefix, targetOrg, targetRepoName, existErr)
		}
		if exists {
			fmt.Printf("%s Skipping %s (target %s/%s already exists)\n", prefix, name, targetOrg, targetRepoName)
			state.skipped++
			state.repoResults = append(state.repoResults, report.BatchRepoResult{
				SourceRepo: sourceRepo,
				TargetRepo: targetOrg + "/" + targetRepoName,
				Status:     "skipped",
			})
			return
		}
	}

	if batchSync {
		fmt.Printf("\n%s Syncing %s → %s/%s\n", prefix, sourceRepo, targetOrg, targetRepoName)
	} else {
		fmt.Printf("\n%s Copying %s → %s/%s\n", prefix, sourceRepo, targetOrg, targetRepoName)
	}

	verifyResult, hitRL, err := batchCopyWithRetry(srcClient, tgtClient, sourceOrg, name, targetOrg, targetRepoName, srcToken, tgtToken, excludeList, state.rateLimitHit, prefix)
	updateRateLimitState(state, hitRL, len(repos)-i-1)

	if err != nil {
		fmt.Printf("%s ✗ FAILED: %v\n", prefix, err)
		state.failed++
		state.failures = append(state.failures, fmt.Sprintf("%s: %v", sourceRepo, err))
		entry := report.BatchRepoResult{
			SourceRepo: sourceRepo,
			TargetRepo: targetOrg + "/" + targetRepoName,
			Status:     "failed",
			Error:      err.Error(),
		}
		if verifyResult != nil {
			entry.Checks = verifyResult.Checks
		}
		state.repoResults = append(state.repoResults, entry)
		return
	}

	fmt.Printf("%s ✓ Done\n", prefix)
	state.succeeded++
	entry := report.BatchRepoResult{
		SourceRepo: sourceRepo,
		TargetRepo: targetOrg + "/" + targetRepoName,
		Status:     "succeeded",
	}
	if verifyResult != nil {
		entry.Checks = verifyResult.Checks
	}
	state.repoResults = append(state.repoResults, entry)

	if batchPerRepoReport && reportPath != "" && verifyResult != nil {
		perRepoPath := strings.TrimSuffix(reportPath, filepath.Ext(reportPath)) + "-" + name + ".json"
		if err := report.WriteJSON(verifyResult, perRepoPath); err != nil {
			fmt.Printf("  Warning: failed to write per-repo report: %v\n", err)
		} else {
			fmt.Printf("  Per-repo report: %s\n", perRepoPath)
		}
	}
}

// batchCopyWithRetry calls copyOneRepo and retries once if a secondary rate limit is hit.
func batchCopyWithRetry(srcClient, tgtClient *ghclient.Client, srcOwner, srcName, targetOrg, targetRepoName, srcToken, tgtToken string, excludeList []string, skipReleases bool, prefix string) (*verify.VerificationReport, bool, error) {
	verifyResult, hitRL, err := copyOneRepo(srcClient, tgtClient, srcOwner, srcName, targetOrg, targetRepoName, srcToken, tgtToken, excludeList, skipReleases, batchSync)

	if err != nil && ghclient.IsRateLimitError(err) {
		retryAfter := ghclient.RetryAfterFromError(err)
		if retryAfter <= 0 {
			retryAfter = 60 * time.Second
		}
		if retryAfter <= 10*time.Minute {
			fmt.Printf("%s ⏳ Secondary rate limit hit — waiting %v before retrying...\n", prefix, retryAfter.Truncate(time.Second))
			time.Sleep(retryAfter + 5*time.Second)
			verifyResult, hitRL, err = copyOneRepo(srcClient, tgtClient, srcOwner, srcName, targetOrg, targetRepoName, srcToken, tgtToken, excludeList, skipReleases, batchSync)
		} else {
			fmt.Printf("%s ⚠ Secondary rate limit hit but retry-after too long (%v) — skipping retry\n", prefix, retryAfter.Truncate(time.Second))
		}
	}

	return verifyResult, hitRL, err
}

// updateRateLimitState updates the batch state when a rate limit is first encountered.
func updateRateLimitState(state *batchState, hitRL bool, remaining int) {
	if hitRL && !state.rateLimitHit {
		state.rateLimitHit = true
		if remaining > 0 {
			fmt.Printf("\n  ⚠ API rate limit exhausted — release copying will be skipped for the remaining %d repos.\n", remaining)
			fmt.Println("    Git clone, push, and verification will continue normally.")
			fmt.Println("    Re-run with a shorter batch or authenticated source to copy releases.")
		}
	} else if state.rateLimitHit && !codeOnly {
		state.releasesSkipped++
	}
}

// writeBatchReportAndSummary writes the batch JSON report and prints the summary.
func writeBatchReportAndSummary(sourceOrg, targetOrg string, repos []string, state *batchState) error {
	if reportPath != "" {
		batchReport := &report.BatchReport{
			SourceOrg:    sourceOrg,
			TargetOrg:    targetOrg,
			SourceHost:   sourceHost,
			TargetHost:   targetHost,
			SearchFilter: batchSearch,
			Timestamp:    time.Now().UTC(),
			Summary: report.BatchSummary{
				Total:           len(repos),
				Succeeded:       state.succeeded,
				Failed:          state.failed,
				Skipped:         state.skipped,
				ReleasesSkipped: state.releasesSkipped,
			},
			Repos: state.repoResults,
		}
		if err := report.WriteBatchJSON(batchReport, reportPath); err != nil {
			fmt.Printf("Warning: failed to write batch report: %v\n", err)
		} else {
			fmt.Printf("\nBatch report written to: %s\n", reportPath)
		}
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════╗")
	fmt.Println("║            VCOPY BATCH SUMMARY                     ║")
	fmt.Println("╚══════════════════════════════════════════════════════╝")
	fmt.Printf("  Total:     %d repositories\n", len(repos))
	fmt.Printf("  Succeeded: %d\n", state.succeeded)
	fmt.Printf("  Failed:    %d\n", state.failed)
	fmt.Printf("  Skipped:   %d\n", state.skipped)
	if state.releasesSkipped > 0 {
		fmt.Printf("  Releases skipped (rate limit): %d\n", state.releasesSkipped)
	}
	if len(state.failures) > 0 {
		fmt.Println("\n  Failures:")
		for _, f := range state.failures {
			fmt.Printf("    - %s\n", f)
		}
	}
	fmt.Println()

	if state.failed > 0 {
		return fmt.Errorf("%d of %d repos failed to copy", state.failed, len(repos))
	}
	return nil
}

// copyOneRepo copies a single repo from source to target, handling creation,
// mirroring, releases, verification, and path exclusion.
// If skipReleases is true, release copying is skipped entirely (used when rate limited).
// Returns the verification report (nil if verification was skipped), whether a rate
// limit error was encountered during release copy, and any fatal error.
func copyOneRepo(srcClient, tgtClient *ghclient.Client, srcOwner, srcName, targetOrg, targetRepoName, srcToken, tgtToken string, excludeList []string, skipReleases, syncMode bool) (*verify.VerificationReport, bool, error) {
	// Determine if target already exists
	repoExists := false
	if syncMode {
		exists, err := tgtClient.RepoExists(targetOrg, targetRepoName)
		if err != nil {
			fmt.Printf("  Warning: could not check if target exists: %v\n", err)
		}
		repoExists = exists
		if exists && verbose {
			fmt.Printf("  Target exists — syncing (additive push + incremental releases)\n")
		}
	}

	// Create target repo (skip if sync mode confirmed it exists)
	if !repoExists {
		if err := tgtClient.CreateRepo(targetOrg, targetRepoName, targetVisibility, verbose); err != nil {
			return nil, false, fmt.Errorf("create repo: %w", err)
		}
	}

	// Mirror (always additive: forceOverwrite=false)
	rejectedRefs, err := vcopy.MirrorRepo(sourceHost, srcOwner, srcName, targetHost, targetOrg, targetRepoName, srcToken, tgtToken, lfs, false, codeOnly, verbose)
	if err != nil {
		return nil, false, fmt.Errorf("mirror: %w", err)
	}

	// Copy/sync releases (unless code-only or rate-limited)
	hitRateLimit := handleBatchReleases(srcClient, tgtClient, srcOwner, srcName, targetOrg, targetRepoName, skipReleases, syncMode, repoExists)

	// Verification
	var results *verify.VerificationReport
	if !skipVerify {
		opts := verify.Options{
			QuickMode:    quickVerify,
			CodeOnly:     codeOnly,
			Verbose:      verbose,
			ExcludedRefs: rejectedRefs,
		}
		var err error
		results, err = verify.RunAll(sourceHost, srcOwner, srcName, targetHost, targetOrg, targetRepoName, srcToken, tgtToken, opts)
		if err != nil {
			return nil, hitRateLimit, fmt.Errorf("verification: %w", err)
		}
		if !results.AllPassed() {
			return results, hitRateLimit, fmt.Errorf("verification FAILED")
		}
	}

	// Exclude paths (post-verification)
	if len(excludeList) > 0 {
		if err := vcopy.CleanupExcludedPaths(targetHost, targetOrg, targetRepoName, tgtToken, excludeList, verbose); err != nil {
			fmt.Printf("  Warning: exclude cleanup failed: %v\n", err)
		}
	}

	return results, hitRateLimit, nil
}

// handleBatchReleases handles the release copy/sync decision for a single repo in batch mode.
func handleBatchReleases(srcClient, tgtClient *ghclient.Client, srcOwner, srcName, targetOrg, targetRepoName string, skipReleases, syncMode, repoExists bool) bool {
	if !codeOnly && !skipReleases {
		var releaseErr error
		if syncMode && repoExists {
			releaseErr = vcopy.SyncReleases(srcClient, tgtClient, srcOwner, srcName, targetOrg, targetRepoName, verbose)
		} else {
			releaseErr = vcopy.CopyReleases(srcClient, tgtClient, srcOwner, srcName, targetOrg, targetRepoName, verbose)
		}
		if releaseErr != nil {
			if ghclient.IsRateLimitError(releaseErr) {
				fmt.Printf("  ⚠ Release copy skipped: API rate limit exhausted\n")
				return true
			}
			fmt.Printf("  Warning: release copy failed: %v\n", releaseErr)
		}
	} else if skipReleases && !codeOnly {
		fmt.Printf("  ⚠ Release copy skipped (rate limit previously exhausted)\n")
	}
	return false
}

func validateVisibility(v string) error {
	switch v {
	case "private", "public", "internal":
		return nil
	default:
		return fmt.Errorf("invalid visibility value %q: must be private, public, or internal", v)
	}
}
