package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	gh "github.com/google/go-github/v58/github"
	"golang.org/x/oauth2"
)

// Client wraps the GitHub API client for both Cloud and Enterprise.
type Client struct {
	API  *gh.Client
	Host string
	ctx  context.Context
}

// NewClient creates a GitHub API client for the given host and token.
func NewClient(host, token string) *Client {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(ctx, ts)

	var client *gh.Client
	if host == "github.com" {
		client = gh.NewClient(httpClient)
	} else {
		baseURL := fmt.Sprintf("https://%s/api/v3/", host)
		uploadURL := fmt.Sprintf("https://%s/api/uploads/", host)
		var err error
		client, err = gh.NewClient(httpClient).WithEnterpriseURLs(baseURL, uploadURL)
		if err != nil {
			// Fall back to cloud client if enterprise URL fails
			client = gh.NewClient(httpClient)
		}
	}

	return &Client{
		API:  client,
		Host: host,
		ctx:  ctx,
	}
}

// RepoExists checks whether a repository exists and is accessible.
func (c *Client) RepoExists(owner, repo string) (bool, error) {
	_, resp, err := c.API.Repositories.Get(c.ctx, owner, repo)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// CreateRepo creates a repository in the specified organization or user account.
// It first tries the org endpoint; if the target is a personal account (404), it
// falls back to creating under the authenticated user.
func (c *Client) CreateRepo(org, name string, verbose bool) error {
	repo := &gh.Repository{
		Name:    gh.String(name),
		Private: gh.Bool(true),
	}

	_, resp, err := c.API.Repositories.Create(c.ctx, org, repo)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnprocessableEntity {
			if verbose {
				fmt.Printf("  Repository %s/%s already exists, continuing...\n", org, name)
			}
			return nil
		}
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			// org endpoint returned 404 — target is likely a personal account
			if verbose {
				fmt.Printf("  %s is not an org, creating under personal account...\n", org)
			}
			_, resp2, err2 := c.API.Repositories.Create(c.ctx, "", repo)
			if err2 != nil {
				if resp2 != nil && resp2.StatusCode == http.StatusUnprocessableEntity {
					if verbose {
						fmt.Printf("  Repository %s/%s already exists, continuing...\n", org, name)
					}
					return nil
				}
				return fmt.Errorf("failed to create repo %s/%s: %w", org, name, err2)
			}
			if verbose {
				fmt.Printf("  Created repository %s/%s\n", org, name)
			}
			return nil
		}
		return fmt.Errorf("failed to create repo %s/%s: %w", org, name, err)
	}

	if verbose {
		fmt.Printf("  Created repository %s/%s\n", org, name)
	}
	return nil
}

// ListAllIssues returns all issues (not PRs) for a repository.
func (c *Client) ListAllIssues(owner, repo string) ([]*gh.Issue, error) {
	var allIssues []*gh.Issue
	opts := &gh.IssueListByRepoOptions{
		State:       "all",
		Sort:        "created",
		Direction:   "asc",
		ListOptions: gh.ListOptions{PerPage: 100},
	}

	for {
		issues, resp, err := c.API.Issues.ListByRepo(c.ctx, owner, repo, opts)
		if err != nil {
			return nil, err
		}
		for _, issue := range issues {
			if !issue.IsPullRequest() {
				allIssues = append(allIssues, issue)
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allIssues, nil
}

// CreateIssue creates an issue in the target repository.
func (c *Client) CreateIssue(owner, repo string, req *gh.IssueRequest) (*gh.Issue, error) {
	issue, _, err := c.API.Issues.Create(c.ctx, owner, repo, req)
	return issue, err
}

// ListIssueComments returns all comments on an issue.
func (c *Client) ListIssueComments(owner, repo string, number int) ([]*gh.IssueComment, error) {
	var allComments []*gh.IssueComment
	opts := &gh.IssueListCommentsOptions{
		ListOptions: gh.ListOptions{PerPage: 100},
	}

	for {
		comments, resp, err := c.API.Issues.ListComments(c.ctx, owner, repo, number, opts)
		if err != nil {
			return nil, err
		}
		allComments = append(allComments, comments...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allComments, nil
}

// CreateIssueComment creates a comment on an issue.
func (c *Client) CreateIssueComment(owner, repo string, number int, body string) error {
	_, _, err := c.API.Issues.CreateComment(c.ctx, owner, repo, number, &gh.IssueComment{Body: &body})
	return err
}

// ListPullRequests returns all pull requests for a repository.
func (c *Client) ListPullRequests(owner, repo string) ([]*gh.PullRequest, error) {
	var allPRs []*gh.PullRequest
	opts := &gh.PullRequestListOptions{
		State:       "all",
		Sort:        "created",
		Direction:   "asc",
		ListOptions: gh.ListOptions{PerPage: 100},
	}

	for {
		prs, resp, err := c.API.PullRequests.List(c.ctx, owner, repo, opts)
		if err != nil {
			return nil, err
		}
		allPRs = append(allPRs, prs...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allPRs, nil
}

// ListReleases returns all releases for a repository.
func (c *Client) ListReleases(owner, repo string) ([]*gh.RepositoryRelease, error) {
	var allReleases []*gh.RepositoryRelease
	opts := &gh.ListOptions{PerPage: 100}

	for {
		releases, resp, err := c.API.Repositories.ListReleases(c.ctx, owner, repo, opts)
		if err != nil {
			return nil, err
		}
		allReleases = append(allReleases, releases...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allReleases, nil
}

// CreateRelease creates a release in the target repository.
func (c *Client) CreateRelease(owner, repo string, release *gh.RepositoryRelease) (*gh.RepositoryRelease, error) {
	r, _, err := c.API.Repositories.CreateRelease(c.ctx, owner, repo, release)
	return r, err
}

// DeleteRelease deletes a release by its ID. Does not delete the associated tag.
func (c *Client) DeleteRelease(owner, repo string, releaseID int64) error {
	_, err := c.API.Repositories.DeleteRelease(c.ctx, owner, repo, releaseID)
	return err
}

// DownloadReleaseAsset downloads a release asset and returns the HTTP response body.
// Caller is responsible for closing the response body.
func (c *Client) DownloadReleaseAsset(owner, repo string, assetID int64) (*http.Response, error) {
	_, redirectURL, err := c.API.Repositories.DownloadReleaseAsset(c.ctx, owner, repo, assetID, http.DefaultClient)
	if err != nil {
		return nil, err
	}
	if redirectURL != "" {
		// Validate redirect URL to prevent SSRF
		parsedURL, err := url.Parse(redirectURL)
		if err != nil || (parsedURL.Scheme != "https" && parsedURL.Scheme != "http") {
			return nil, fmt.Errorf("invalid redirect URL for asset %d", assetID)
		}
		client := &http.Client{Timeout: 10 * time.Minute}
		resp, err := client.Get(redirectURL)
		if err != nil {
			return nil, fmt.Errorf("download request failed for asset %d: %w", assetID, err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("download failed for asset %d: HTTP %d", assetID, resp.StatusCode)
		}
		return resp, nil
	}
	return nil, fmt.Errorf("no download URL returned for asset %d", assetID)
}

// EditIssue updates an existing issue (e.g. to close it).
func (c *Client) EditIssue(owner, repo string, number int, req *gh.IssueRequest) error {
	_, _, err := c.API.Issues.Edit(c.ctx, owner, repo, number, req)
	return err
}

// UploadReleaseAsset uploads an asset to a release.
func (c *Client) UploadReleaseAsset(owner, repo string, releaseID int64, name string, file *UploadFile) error {
	opts := &gh.UploadOptions{Name: name}
	_, _, err := c.API.Repositories.UploadReleaseAsset(c.ctx, owner, repo, releaseID, opts, file.File)
	return err
}

// CloneURL returns the HTTPS clone URL for a repository.
func CloneURL(host, owner, repo, token string) string {
	if token != "" {
		return fmt.Sprintf("https://x-access-token:%s@%s/%s/%s.git", token, host, owner, repo)
	}
	return fmt.Sprintf("https://%s/%s/%s.git", host, owner, repo)
}

// ListLabels returns all labels for a repository.
func (c *Client) ListLabels(owner, repo string) ([]*gh.Label, error) {
	var allLabels []*gh.Label
	opts := &gh.ListOptions{PerPage: 100}

	for {
		labels, resp, err := c.API.Issues.ListLabels(c.ctx, owner, repo, opts)
		if err != nil {
			return nil, err
		}
		allLabels = append(allLabels, labels...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allLabels, nil
}

// CreateLabel creates a label in the target repository.
func (c *Client) CreateLabel(owner, repo string, label *gh.Label) error {
	_, _, err := c.API.Issues.CreateLabel(c.ctx, owner, repo, label)
	if err != nil && strings.Contains(err.Error(), "already_exists") {
		return nil
	}
	return err
}

// ListReleaseAssets returns all assets for a release.
func (c *Client) ListReleaseAssets(owner, repo string, releaseID int64) ([]*gh.ReleaseAsset, error) {
	var allAssets []*gh.ReleaseAsset
	opts := &gh.ListOptions{PerPage: 100}

	for {
		assets, resp, err := c.API.Repositories.ListReleaseAssets(c.ctx, owner, repo, releaseID, opts)
		if err != nil {
			return nil, err
		}
		allAssets = append(allAssets, assets...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allAssets, nil
}
