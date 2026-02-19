package copy

import (
	"fmt"
	"strings"

	gh "github.com/google/go-github/v58/github"
	ghclient "github.com/jaiperez/vcopy/internal/github"
)

// CopyPullRequests migrates PRs as issues (PRs cannot be recreated via API).
func CopyPullRequests(src, tgt *ghclient.Client, srcOwner, srcRepo, tgtOwner, tgtRepo string, verbose bool) error {
	prs, err := src.ListPullRequests(srcOwner, srcRepo)
	if err != nil {
		return fmt.Errorf("failed to list source PRs: %w", err)
	}

	for _, pr := range prs {
		if verbose {
			fmt.Printf("  Copying PR #%d: %s\n", pr.GetNumber(), pr.GetTitle())
		}

		body := formatMigratedPRBody(pr, srcOwner, srcRepo)
		labelNames := []string{"migrated-pr"}

		req := &gh.IssueRequest{
			Title:  gh.String(fmt.Sprintf("[PR #%d] %s", pr.GetNumber(), pr.GetTitle())),
			Body:   &body,
			Labels: &labelNames,
		}

		_, err := tgt.CreateIssue(tgtOwner, tgtRepo, req)
		if err != nil {
			return fmt.Errorf("failed to create issue for PR #%d: %w", pr.GetNumber(), err)
		}
	}

	fmt.Printf("  Copied %d pull requests (as issues)\n", len(prs))
	return nil
}

func formatMigratedPRBody(pr *gh.PullRequest, srcOwner, srcRepo string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("> *Migrated PR from %s/%s#%d*\n", srcOwner, srcRepo, pr.GetNumber()))
	sb.WriteString(fmt.Sprintf("> *Original author: @%s*\n", pr.GetUser().GetLogin()))
	sb.WriteString(fmt.Sprintf("> *State: %s*\n", pr.GetState()))
	sb.WriteString(fmt.Sprintf("> *Base: %s ← Head: %s*\n", pr.GetBase().GetRef(), pr.GetHead().GetRef()))
	sb.WriteString(fmt.Sprintf("> *Created: %s*\n\n", pr.GetCreatedAt().Format("2006-01-02 15:04:05 UTC")))
	if pr.GetMerged() {
		mergedBy := "unknown"
		if pr.GetMergedBy() != nil {
			mergedBy = pr.GetMergedBy().GetLogin()
		}
		sb.WriteString(fmt.Sprintf("> *Merged at: %s by @%s*\n\n", pr.GetMergedAt().Format("2006-01-02 15:04:05 UTC"), mergedBy))
	}
	sb.WriteString(pr.GetBody())
	return sb.String()
}
