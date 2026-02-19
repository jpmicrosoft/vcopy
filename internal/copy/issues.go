package copy

import (
	"fmt"
	"strings"

	gh "github.com/google/go-github/v58/github"
	ghclient "github.com/jaiperez/vcopy/internal/github"
)

// CopyIssues migrates all issues from source to target repository.
func CopyIssues(src, tgt *ghclient.Client, srcOwner, srcRepo, tgtOwner, tgtRepo string, verbose bool) error {
	// Copy labels first
	labels, err := src.ListLabels(srcOwner, srcRepo)
	if err != nil {
		return fmt.Errorf("failed to list source labels: %w", err)
	}
	for _, label := range labels {
		tgtLabel := &gh.Label{
			Name:        label.Name,
			Color:       label.Color,
			Description: label.Description,
		}
		if err := tgt.CreateLabel(tgtOwner, tgtRepo, tgtLabel); err != nil {
			if verbose {
				fmt.Printf("  Warning: failed to create label %s: %v\n", label.GetName(), err)
			}
		}
	}

	issues, err := src.ListAllIssues(srcOwner, srcRepo)
	if err != nil {
		return fmt.Errorf("failed to list source issues: %w", err)
	}

	for _, issue := range issues {
		if verbose {
			fmt.Printf("  Copying issue #%d: %s\n", issue.GetNumber(), issue.GetTitle())
		}

		// Build label names
		var labelNames []string
		for _, l := range issue.Labels {
			labelNames = append(labelNames, l.GetName())
		}

		body := formatMigratedIssueBody(issue, srcOwner, srcRepo)

		req := &gh.IssueRequest{
			Title:  issue.Title,
			Body:   &body,
			Labels: &labelNames,
		}

		newIssue, err := tgt.CreateIssue(tgtOwner, tgtRepo, req)
		if err != nil {
			return fmt.Errorf("failed to create issue #%d: %w", issue.GetNumber(), err)
		}

		// Copy comments
		comments, err := src.ListIssueComments(srcOwner, srcRepo, issue.GetNumber())
		if err != nil {
			if verbose {
				fmt.Printf("  Warning: failed to list comments for issue #%d: %v\n", issue.GetNumber(), err)
			}
			continue
		}
		for _, comment := range comments {
			commentBody := formatMigratedComment(comment)
			if err := tgt.CreateIssueComment(tgtOwner, tgtRepo, newIssue.GetNumber(), commentBody); err != nil {
				if verbose {
					fmt.Printf("  Warning: failed to copy comment on issue #%d: %v\n", issue.GetNumber(), err)
				}
			}
		}

		// Close issue if it was closed in source
		if issue.GetState() == "closed" {
			closedState := "closed"
			closeReq := &gh.IssueRequest{State: &closedState}
			if _, err := tgt.CreateIssue(tgtOwner, tgtRepo, closeReq); err != nil {
				if verbose {
					fmt.Printf("  Warning: failed to close issue #%d: %v\n", newIssue.GetNumber(), err)
				}
			}
		}
	}

	fmt.Printf("  Copied %d issues\n", len(issues))
	return nil
}

func formatMigratedIssueBody(issue *gh.Issue, srcOwner, srcRepo string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("> *Migrated from %s/%s#%d*\n", srcOwner, srcRepo, issue.GetNumber()))
	sb.WriteString(fmt.Sprintf("> *Original author: @%s*\n", issue.GetUser().GetLogin()))
	sb.WriteString(fmt.Sprintf("> *Created: %s*\n\n", issue.GetCreatedAt().Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString(issue.GetBody())
	return sb.String()
}

func formatMigratedComment(comment *gh.IssueComment) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("> *Comment by @%s on %s*\n\n",
		comment.GetUser().GetLogin(),
		comment.GetCreatedAt().Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString(comment.GetBody())
	return sb.String()
}
