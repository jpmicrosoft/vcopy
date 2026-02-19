package main

import (
	"fmt"
	"os"

	"github.com/jpmicrosoft/vcopy/internal/auth"
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
	copyIssues   bool
	copyPRs      bool
	copyWiki     bool
	copyReleases bool
	allMetadata  bool
	verifyOnly   bool
	reportPath   string
	verbose      bool
	dryRun       bool
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
		Args: cobra.ExactArgs(2),
		RunE: run,
	}

	f := rootCmd.Flags()
	f.StringVar(&sourceHost, "source-host", "github.com", "Source GitHub host")
	f.StringVar(&targetHost, "target-host", "github.com", "Target GitHub host")
	f.StringVar(&targetName, "target-name", "", "Target repo name (default: same as source)")
	f.StringVar(&authMethod, "auth-method", "auto", "Auth method: auto, gh, pat")
	f.StringVar(&sourceToken, "source-token", "", "PAT for source (if auth-method=pat)")
	f.StringVar(&targetToken, "target-token", "", "PAT for target (if auth-method=pat)")
	f.BoolVar(&copyIssues, "issues", false, "Copy issues")
	f.BoolVar(&copyPRs, "pull-requests", false, "Copy pull requests")
	f.BoolVar(&copyWiki, "wiki", false, "Copy wiki")
	f.BoolVar(&copyReleases, "releases", false, "Copy releases and artifacts")
	f.BoolVar(&allMetadata, "all-metadata", false, "Copy all metadata (issues, PRs, wiki, releases)")
	f.BoolVar(&verifyOnly, "verify-only", false, "Skip copy, only verify existing source vs target")
	f.StringVar(&reportPath, "report", "", "Path to write JSON verification report")
	f.BoolVar(&verbose, "verbose", false, "Verbose output")
	f.BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	sourceRepo := args[0]
	targetOrg := args[1]

	if allMetadata {
		copyIssues = true
		copyPRs = true
		copyWiki = true
		copyReleases = true
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
		fmt.Printf("Source:  %s/%s\n", sourceHost, sourceRepo)
		fmt.Printf("Target:  %s/%s/%s\n", targetHost, targetOrg, repoName)
		fmt.Printf("Copy Issues: %v, PRs: %v, Wiki: %v, Releases: %v\n", copyIssues, copyPRs, copyWiki, copyReleases)
		fmt.Printf("Verify Only: %v\n", verifyOnly)
		return nil
	}

	// Authenticate
	srcToken, tgtToken, err := auth.Authenticate(authMethod, sourceHost, targetHost, sourceToken, targetToken)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Create GitHub API clients
	srcClient := ghclient.NewClient(sourceHost, srcToken)
	tgtClient := ghclient.NewClient(targetHost, tgtToken)

	srcOwner, srcName, err := parseRepo(sourceRepo)
	if err != nil {
		return err
	}

	if !verifyOnly {
		// Create target repo if needed
		fmt.Printf("Creating target repository %s/%s on %s...\n", targetOrg, repoName, targetHost)
		if err := tgtClient.CreateRepo(targetOrg, repoName, verbose); err != nil {
			return fmt.Errorf("failed to create target repo: %w", err)
		}

		// Mirror the git repository
		fmt.Printf("Mirroring %s/%s from %s to %s/%s on %s...\n", srcOwner, srcName, sourceHost, targetOrg, repoName, targetHost)
		if err := vcopy.MirrorRepo(sourceHost, srcOwner, srcName, targetHost, targetOrg, repoName, srcToken, tgtToken, verbose); err != nil {
			return fmt.Errorf("mirror failed: %w", err)
		}

		// Copy metadata if requested
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
		if copyReleases {
			fmt.Println("Copying releases...")
			if err := vcopy.CopyReleases(srcClient, tgtClient, srcOwner, srcName, targetOrg, repoName, verbose); err != nil {
				return fmt.Errorf("release copy failed: %w", err)
			}
		}
	}

	// Run verification
	fmt.Println("\n=== Running Integrity Verification ===")
	results, err := verify.RunAll(sourceHost, srcOwner, srcName, targetHost, targetOrg, repoName, srcToken, tgtToken, verbose)
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	// Output report
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

	fmt.Println("\n✓ All verification checks passed. Repository copy is verified.")
	return nil
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
