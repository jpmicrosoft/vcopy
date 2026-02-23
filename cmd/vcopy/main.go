package main

import (
"bufio"
"fmt"
"os"
"strings"

"github.com/jpmicrosoft/vcopy/internal/auth"
"github.com/jpmicrosoft/vcopy/internal/config"
vcopy "github.com/jpmicrosoft/vcopy/internal/copy"
ghclient "github.com/jpmicrosoft/vcopy/internal/github"
"github.com/jpmicrosoft/vcopy/internal/report"
"github.com/jpmicrosoft/vcopy/internal/verify"
"github.com/spf13/cobra"
)

var (
sourceHost   string
targetHost   string
targetName   string
authMethod   string
sourceToken  string
targetToken  string
publicSource bool
lfs          bool
force        bool
codeOnly     bool
copyIssues   bool
copyPRs      bool
copyWiki     bool
allMetadata  bool
verifyOnly   bool
skipVerify   bool
quickVerify  bool
since        string
reportPath   string
signKey      string
configPath   string
verbose        bool
dryRun         bool
nonInteractive bool
)

func main() {
rootCmd := &cobra.Command{
Use:   "vcopy <source-repo> <target-org>",
Short: "Verified GitHub repository copy tool",
Long: `vcopy copies a GitHub repository to a target organization (Cloud or Enterprise)
with comprehensive integrity verification to ensure the copy has not been tampered with.

It performs a bare mirror clone of all branches, tags, and history, then runs a
5-layer verification suite: object hashes, ref comparison, tree hashes, commit
signatures, and bundle SHA-256 checksums.`,
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
f.BoolVar(&publicSource, "public", false, "Source repo is public (skip source authentication)")
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

if err := rootCmd.Execute(); err != nil {
os.Exit(1)
}
}

func run(cmd *cobra.Command, args []string) error {
// Load config file if provided
if configPath != "" {
cfg, err := config.Load(configPath)
if err != nil {
return fmt.Errorf("config error: %w", err)
}
applyConfig(cfg)
}

// Config file may supply args, CLI args override
var sourceRepo, targetOrg string
if len(args) >= 2 {
sourceRepo = args[0]
targetOrg = args[1]
} else if configPath != "" {
cfgArgs, err := config.Load(configPath)
if err != nil {
return fmt.Errorf("config error on re-read: %w", err)
}
sourceRepo = cfgArgs.Source.Repo
targetOrg = cfgArgs.Target.Org
} else {
return fmt.Errorf("requires 2 arguments: <source-repo> <target-org> (or use --config)")
}

if allMetadata {
copyIssues = true
copyPRs = true
copyWiki = true
}

// Resolve target repo name
repoName := targetName
if repoName == "" {
_, name, err := parseRepo(sourceRepo)
if err != nil {
return err
}
repoName = name
}

if dryRun {
fmt.Println("=== DRY RUN ===")
fmt.Printf("Source:      %s/%s (public: %v)\n", sourceHost, sourceRepo, publicSource)
fmt.Printf("Target:      %s/%s/%s\n", targetHost, targetOrg, repoName)
fmt.Printf("LFS:         %v\n", lfs)
fmt.Printf("Metadata:    issues=%v, PRs=%v, wiki=%v\n", copyIssues, copyPRs, copyWiki)
		fmt.Printf("Code only:   %v\n", codeOnly)
fmt.Printf("Verify:      skip=%v, quick=%v, only=%v, since=%q\n", skipVerify, quickVerify, verifyOnly, since)
fmt.Printf("Report:      %s\n", reportPath)
fmt.Printf("Attestation: %s\n", signKey)
return nil
}

// Authenticate
var srcToken, tgtToken string
var err error
if publicSource {
fmt.Println("Public source mode: skipping source authentication")
if copyIssues || copyPRs {
fmt.Println("Note: Metadata copy from public repos uses unauthenticated API access (60 req/hr rate limit).")
fmt.Println("      For repos with many issues/PRs/releases, consider providing a source token for higher limits.")
}
tgtToken, err = auth.AuthenticateTarget(authMethod, targetHost, targetToken)
if err != nil {
return fmt.Errorf("target authentication failed: %w", err)
}
} else {
srcToken, tgtToken, err = auth.Authenticate(authMethod, sourceHost, targetHost, sourceToken, targetToken)
if err != nil {
return fmt.Errorf("authentication failed: %w", err)
}
}

srcClient := ghclient.NewClient(sourceHost, srcToken)
tgtClient := ghclient.NewClient(targetHost, tgtToken)

srcOwner, srcName, err := parseRepo(sourceRepo)
if err != nil {
return err
}

if !verifyOnly {
fmt.Printf("Creating target repository %s/%s on %s...\n", targetOrg, repoName, targetHost)

// Check if target repo already exists
exists, existErr := tgtClient.RepoExists(targetOrg, repoName)
if existErr != nil {
if !force {
return fmt.Errorf("cannot verify if target repo exists: %w\n  Use --force to bypass this check", existErr)
}
fmt.Printf("  Warning: could not check if target repo exists: %v\n", existErr)
fmt.Println("  Proceeding because --force was specified.")
}

forceOverwrite := false
if exists {
if force {
// --force: destructive mirror push (overwrites everything)
fmt.Println()
fmt.Printf("  ⚠️  WARNING: Target repository %s/%s already exists on %s.\n", targetOrg, repoName, targetHost)
fmt.Println("  --force mode: a mirror push will OVERWRITE all branches, tags, and history.")
fmt.Println("  Any content in the target that does not exist in the source WILL BE PERMANENTLY LOST.")
fmt.Println()
if !nonInteractive && !confirmAction("Do you want to continue and overwrite the existing repository?") {
fmt.Println("Aborted.")
return nil
}
fmt.Println()
forceOverwrite = true
} else {
// Default: additive push (preserves existing tags and releases)
fmt.Printf("  Repository %s/%s already exists. Using additive mode (existing tags and releases preserved).\n", targetOrg, repoName)
}
} else {
if err := tgtClient.CreateRepo(targetOrg, repoName, verbose); err != nil {
return fmt.Errorf("failed to create target repo: %w", err)
}
}

fmt.Printf("Mirroring %s/%s from %s to %s/%s on %s...\n", srcOwner, srcName, sourceHost, targetOrg, repoName, targetHost)
if err := vcopy.MirrorRepo(sourceHost, srcOwner, srcName, targetHost, targetOrg, repoName, srcToken, tgtToken, lfs, forceOverwrite, codeOnly, verbose); err != nil {
return fmt.Errorf("mirror failed: %w", err)
}

// Auto-sync releases and tags (unless --code-only)
if !codeOnly {
if exists && !forceOverwrite {
fmt.Println("Syncing new releases to target (existing releases preserved)...")
if err := vcopy.SyncReleases(srcClient, tgtClient, srcOwner, srcName, targetOrg, repoName, verbose); err != nil {
fmt.Printf("Warning: release sync failed: %v\n", err)
}
		} else if exists && forceOverwrite {
			// --force on existing repo: clean orphaned releases, then copy all from source
			fmt.Println("Cleaning orphaned releases from target...")
			if err := vcopy.CleanTargetReleases(srcClient, tgtClient, srcOwner, srcName, targetOrg, repoName, verbose); err != nil {
				fmt.Printf("Warning: release cleanup failed: %v\n", err)
			}
			fmt.Println("Copying releases...")
			if err := vcopy.CopyReleases(srcClient, tgtClient, srcOwner, srcName, targetOrg, repoName, verbose); err != nil {
				fmt.Printf("Warning: release copy failed: %v\n", err)
			}
		} else {
			// New repo: copy all releases from source
			fmt.Println("Copying releases...")
			if err := vcopy.CopyReleases(srcClient, tgtClient, srcOwner, srcName, targetOrg, repoName, verbose); err != nil {
				fmt.Printf("Warning: release copy failed: %v\n", err)
			}
}
} else if verbose {
fmt.Println("  Skipping tags and releases (--code-only mode)")
}

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
}

if !skipVerify {
fmt.Println("\n=== Running Integrity Verification ===")

var results *verify.VerificationReport
if since != "" {
results, err = verify.RunIncremental(sourceHost, srcOwner, srcName, targetHost, targetOrg, repoName, srcToken, tgtToken, since, verbose)
} else {
opts := verify.Options{
QuickMode: quickVerify,
CodeOnly:  codeOnly,
Verbose:   verbose,
}
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
} else {
fmt.Println("\nCopy complete (verification skipped).")
}

return nil
}

func applyConfig(cfg *config.Config) {
if sourceHost == "github.com" && cfg.Source.Host != "" {
sourceHost = cfg.Source.Host
}
if targetHost == "github.com" && cfg.Target.Host != "" {
targetHost = cfg.Target.Host
}
if targetName == "" && cfg.Target.Name != "" {
targetName = cfg.Target.Name
}
if authMethod == "auto" && cfg.Auth.Method != "" {
authMethod = cfg.Auth.Method
}
if sourceToken == "" && cfg.Auth.SourceToken != "" {
sourceToken = cfg.Auth.SourceToken
}
if targetToken == "" && cfg.Auth.TargetToken != "" {
targetToken = cfg.Auth.TargetToken
}
if !publicSource { publicSource = cfg.Source.Public }
if !lfs { lfs = cfg.LFS }
if !force { force = cfg.Force }
if !codeOnly { codeOnly = cfg.CodeOnly }
if !copyIssues { copyIssues = cfg.Copy.Issues }
if !copyPRs { copyPRs = cfg.Copy.PullRequests }
if !copyWiki { copyWiki = cfg.Copy.Wiki }
if !allMetadata { allMetadata = cfg.Copy.AllMetadata }
if !skipVerify { skipVerify = cfg.Verify.Skip }
if !quickVerify { quickVerify = cfg.Verify.QuickMode }
if !verifyOnly { verifyOnly = cfg.Verify.VerifyOnly }
if since == "" { since = cfg.Verify.Since }
if reportPath == "" && cfg.Report.Path != "" { reportPath = cfg.Report.Path }
if signKey == "" && cfg.Report.SignKey != "" { signKey = cfg.Report.SignKey }
if !verbose { verbose = cfg.Verbose }
	if !nonInteractive { nonInteractive = cfg.NonInteractive }
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
