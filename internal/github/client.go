package github

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	gh "github.com/google/go-github/v58/github"
	"golang.org/x/oauth2"
)

// IsRateLimitError returns true if the error is a GitHub rate limit error
// (primary or secondary/abuse). Callers can use this to skip operations
// rather than retrying when rate limits are exhausted.
func IsRateLimitError(err error) bool {
	var rle *gh.RateLimitError
	if errors.As(err, &rle) {
		return true
	}
	var arle *gh.AbuseRateLimitError
	return errors.As(err, &arle)
}

// Client wraps the GitHub API client for both Cloud and Enterprise.
type Client struct {
	API  *gh.Client
	Host string
	ctx  context.Context
}

// NewClient creates a GitHub API client for the given host and token.
// If token is empty, an unauthenticated client is created (suitable for public repos).
// All clients automatically handle rate limiting by sleeping until reset.
func NewClient(host, token string) (*Client, error) {
	ctx := context.Background()

	var httpClient *http.Client
	if token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		httpClient = oauth2.NewClient(ctx, ts)
		httpClient.Timeout = 5 * time.Minute
		// Wrap the oauth2 transport with rate limit awareness
		httpClient.Transport = &rateLimitTransport{base: httpClient.Transport}
	} else {
		httpClient = &http.Client{
			Timeout:   5 * time.Minute,
			Transport: &rateLimitTransport{base: http.DefaultTransport},
		}
	}

	var client *gh.Client
	if host == "github.com" {
		client = gh.NewClient(httpClient)
	} else {
		baseURL := fmt.Sprintf("https://%s/api/v3/", host)
		uploadURL := fmt.Sprintf("https://%s/api/uploads/", host)
		var err error
		client, err = gh.NewClient(httpClient).WithEnterpriseURLs(baseURL, uploadURL)
		if err != nil {
			return nil, fmt.Errorf("invalid Enterprise URL for host %s: %w", host, err)
		}
	}

	return &Client{
		API:  client,
		Host: host,
		ctx:  ctx,
	}, nil
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
func (c *Client) CreateRepo(org, name, visibility string, verbose bool) error {
	switch visibility {
	case "private", "public", "internal":
	default:
		return fmt.Errorf("invalid visibility %q: must be private, public, or internal", visibility)
	}

	repo := &gh.Repository{
		Name:       gh.String(name),
		Visibility: gh.String(visibility),
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
	// Pass nil to get the redirect URL without following it, so we can apply
	// SSRF validation before making the actual download request.
	rc, redirectURL, err := c.API.Repositories.DownloadReleaseAsset(c.ctx, owner, repo, assetID, nil)
	if err != nil {
		return nil, err
	}
	// If no redirect (content returned directly), wrap the ReadCloser in an http.Response
	if redirectURL == "" && rc != nil {
		return &http.Response{StatusCode: http.StatusOK, Body: rc}, nil
	}
	if rc != nil {
		rc.Close()
	}
	if redirectURL != "" {
		// Validate redirect URL to prevent SSRF
		parsedURL, err := url.Parse(redirectURL)
		if err != nil || parsedURL.Scheme != "https" {
			return nil, fmt.Errorf("invalid or non-HTTPS redirect URL for asset %d", assetID)
		}
		// Block private/internal hostnames
		host := parsedURL.Hostname()
		if host == "localhost" {
			return nil, fmt.Errorf("redirect to localhost blocked for asset %d", assetID)
		}
		if ip := net.ParseIP(host); ip != nil {
			if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
				return nil, fmt.Errorf("redirect to private/internal network blocked for asset %d", assetID)
			}
		}
		// Use a custom dialer that validates resolved IPs to prevent DNS rebinding
		safeDialer := &net.Dialer{Timeout: 30 * time.Second}
		client := &http.Client{
			Timeout: 10 * time.Minute,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					dialHost, port, err := net.SplitHostPort(addr)
					if err != nil {
						return nil, err
					}
					ips, err := net.LookupIP(dialHost)
					if err != nil {
						return nil, err
					}
					for _, ip := range ips {
						if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
							return nil, fmt.Errorf("DNS resolved to private/internal IP %s — blocked for SSRF protection", ip)
						}
					}
					return safeDialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
				},
			},
		}
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

// SearchRepos finds repositories in a given org whose names contain the query string.
// Returns a list of repo names (not full names — just the repo part).
func (c *Client) SearchRepos(org, nameFilter string) ([]string, error) {
	query := fmt.Sprintf("org:%s %s in:name", org, nameFilter)
	opts := &gh.SearchOptions{
		ListOptions: gh.ListOptions{PerPage: 100},
	}

	var repos []string
	for {
		result, resp, err := c.API.Search.Repositories(c.ctx, query, opts)
		if err != nil {
			return nil, fmt.Errorf("repo search failed: %w", err)
		}
		for _, r := range result.Repositories {
			repos = append(repos, r.GetName())
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return repos, nil
}

// maxRateLimitRetries is the maximum number of times to retry after hitting a rate limit.
const maxRateLimitRetries = 3

// maxRateLimitWait caps how long we'll sleep for a single rate limit reset.
const maxRateLimitWait = 2 * time.Minute

// rateLimitTransport wraps an http.RoundTripper and automatically retries
// requests that receive a 403 response with rate limit headers (X-RateLimit-Remaining: 0).
// It sleeps until the X-RateLimit-Reset time before retrying.
type rateLimitTransport struct {
	base http.RoundTripper
}

func (t *rateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for attempt := 0; attempt <= maxRateLimitRetries; attempt++ {
		resp, err := t.base.RoundTrip(req)
		if err != nil {
			return resp, err
		}

		// Determine wait duration based on rate limit type
		var wait time.Duration
		switch {
		case resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0":
			// Primary rate limit: 403 + X-RateLimit-Remaining: 0
			resetStr := resp.Header.Get("X-RateLimit-Reset")
			if resetStr == "" {
				return resp, nil
			}
			var resetEpoch int64
			if _, scanErr := fmt.Sscanf(resetStr, "%d", &resetEpoch); scanErr != nil {
				return resp, nil
			}
			wait = time.Until(time.Unix(resetEpoch, 0)) + 5*time.Second
		case resp.StatusCode == http.StatusTooManyRequests:
			// Secondary/abuse rate limit: 429 + Retry-After header
			retryAfter := resp.Header.Get("Retry-After")
			if retryAfter == "" {
				wait = 60 * time.Second
			} else {
				var secs int64
				if _, scanErr := fmt.Sscanf(retryAfter, "%d", &secs); scanErr != nil {
					wait = 60 * time.Second
				} else {
					wait = time.Duration(secs)*time.Second + 2*time.Second
				}
			}
		default:
			return resp, nil
		}

		if attempt == maxRateLimitRetries {
			return resp, nil
		}
		if wait <= 0 {
			wait = 2 * time.Second
		}
		if wait > maxRateLimitWait {
			fmt.Printf("  ⚠ Rate limit reset too far in the future (%v) — not waiting\n", wait.Truncate(time.Second))
			return resp, nil
		}
		if req.Body != nil && req.GetBody == nil {
			return resp, nil
		}
		resp.Body.Close()
		fmt.Printf("  ⏳ Rate limited — waiting %v until reset (attempt %d/%d)...\n", wait.Truncate(time.Second), attempt+1, maxRateLimitRetries)
		select {
		case <-time.After(wait):
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
		if req.Body != nil {
			body, bodyErr := req.GetBody()
			if bodyErr != nil {
				return nil, fmt.Errorf("cannot retry rate-limited request: %w", bodyErr)
			}
			req = req.Clone(req.Context())
			req.Body = body
		}
	}
	return nil, fmt.Errorf("rate limit retry loop exited unexpectedly")
}
