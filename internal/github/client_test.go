package github

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	gh "github.com/google/go-github/v58/github"
)

func TestCloneURL_WithToken(t *testing.T) {
	url := CloneURL("github.com", "owner", "repo", "mytoken123")
	want := "https://x-access-token:mytoken123@github.com/owner/repo.git"
	if url != want {
		t.Errorf("CloneURL with token = %q, want %q", url, want)
	}
}

func TestCloneURL_WithoutToken(t *testing.T) {
	url := CloneURL("github.com", "owner", "repo", "")
	want := "https://github.com/owner/repo.git"
	if url != want {
		t.Errorf("CloneURL without token = %q, want %q", url, want)
	}
}

func TestCloneURL_Enterprise(t *testing.T) {
	url := CloneURL("ghes.example.com", "myorg", "myrepo", "tok")
	want := "https://x-access-token:tok@ghes.example.com/myorg/myrepo.git"
	if url != want {
		t.Errorf("CloneURL enterprise = %q, want %q", url, want)
	}
}

func TestNewUploadFile_ErrorOnBadReader(t *testing.T) {
	// Test with a reader that fails immediately
	f, err := NewUploadFile(&failReader{}, "test.bin", 100)
	if err == nil {
		if f != nil {
			f.Cleanup()
		}
		t.Error("expected error for failing reader")
	}
	if f != nil {
		f.Cleanup()
		t.Error("expected nil UploadFile for failing reader")
	}
}

type failReader struct{}

func (f *failReader) Read(p []byte) (int, error) {
	return 0, &testError{"read failed"}
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

func TestRateLimitTransport_NoRateLimit(t *testing.T) {
	// Non-403 responses should pass through untouched
	base := &mockTransport{
		response: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader("ok")),
			Header:     http.Header{},
		},
	}
	transport := &rateLimitTransport{base: base}
	req, _ := http.NewRequest("GET", "https://api.github.com/test", nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if base.calls != 1 {
		t.Errorf("expected 1 call, got %d", base.calls)
	}
}

func TestRateLimitTransport_403_NotRateLimit(t *testing.T) {
	// 403 without rate limit headers should pass through (e.g., permission denied)
	base := &mockTransport{
		response: &http.Response{
			StatusCode: 403,
			Body:       io.NopCloser(strings.NewReader("forbidden")),
			Header:     http.Header{},
		},
	}
	transport := &rateLimitTransport{base: base}
	req, _ := http.NewRequest("GET", "https://api.github.com/test", nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 403 {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
	if base.calls != 1 {
		t.Errorf("expected 1 call (no retry for non-rate-limit 403), got %d", base.calls)
	}
}

func TestRateLimitTransport_403_RateLimitWithPastReset(t *testing.T) {
	// 403 with rate limit headers where reset is in the past — should retry once and succeed
	callCount := 0
	base := &mockTransportFunc{
		fn: func(req *http.Request) (*http.Response, error) {
			callCount++
			if callCount == 1 {
				h := http.Header{}
				h.Set("X-RateLimit-Remaining", "0")
				h.Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(-10*time.Second).Unix()))
				return &http.Response{
					StatusCode: 403,
					Body:       io.NopCloser(strings.NewReader("rate limited")),
					Header:     h,
				}, nil
			}
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("ok")),
				Header:     http.Header{},
			}, nil
		},
	}
	transport := &rateLimitTransport{base: base}
	req, _ := http.NewRequest("GET", "https://api.github.com/test", nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 after retry, got %d", resp.StatusCode)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls (original + retry), got %d", callCount)
	}
}

func TestRateLimitTransport_MaxRetriesExhausted(t *testing.T) {
	// All retries get rate-limited — should return the final 403 response
	base := &mockTransportFunc{
		fn: func(req *http.Request) (*http.Response, error) {
			h := http.Header{}
			h.Set("X-RateLimit-Remaining", "0")
			h.Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(-5*time.Second).Unix()))
			return &http.Response{
				StatusCode: 403,
				Body:       io.NopCloser(strings.NewReader("rate limited")),
				Header:     h,
			}, nil
		},
	}
	transport := &rateLimitTransport{base: base}
	req, _ := http.NewRequest("GET", "https://api.github.com/test", nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 403 {
		t.Errorf("expected 403 after exhausted retries, got %d", resp.StatusCode)
	}
}

type mockTransport struct {
	response *http.Response
	calls    int
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	m.calls++
	return m.response, nil
}

type mockTransportFunc struct {
	fn func(*http.Request) (*http.Response, error)
}

func (m *mockTransportFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.fn(req)
}

func TestRateLimitTransport_429_RetryAfter(t *testing.T) {
	callCount := 0
	base := &mockTransportFunc{
		fn: func(req *http.Request) (*http.Response, error) {
			callCount++
			if callCount == 1 {
				h := http.Header{}
				h.Set("Retry-After", "1")
				return &http.Response{
					StatusCode: 429,
					Body:       io.NopCloser(strings.NewReader("secondary rate limit")),
					Header:     h,
				}, nil
			}
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("ok")),
				Header:     http.Header{},
			}, nil
		},
	}
	transport := &rateLimitTransport{base: base}
	req, _ := http.NewRequest("GET", "https://api.github.com/test", nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 after 429 retry, got %d", resp.StatusCode)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

func TestRateLimitTransport_MaxWaitExceeded(t *testing.T) {
	// Reset time far in the future — should return 403 without sleeping
	base := &mockTransportFunc{
		fn: func(req *http.Request) (*http.Response, error) {
			h := http.Header{}
			h.Set("X-RateLimit-Remaining", "0")
			h.Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(1*time.Hour).Unix()))
			return &http.Response{
				StatusCode: 403,
				Body:       io.NopCloser(strings.NewReader("rate limited")),
				Header:     h,
			}, nil
		},
	}
	transport := &rateLimitTransport{base: base}
	req, _ := http.NewRequest("GET", "https://api.github.com/test", nil)
	start := time.Now()
	resp, err := transport.RoundTrip(req)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 403 {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
	if elapsed > 5*time.Second {
		t.Errorf("should not sleep when wait exceeds cap, but took %v", elapsed)
	}
}

func TestIsRateLimitError_Primary(t *testing.T) {
	err := &gh.RateLimitError{Message: "rate limit"}
	if !IsRateLimitError(err) {
		t.Error("expected IsRateLimitError to return true for RateLimitError")
	}
}

func TestIsRateLimitError_Abuse(t *testing.T) {
	err := &gh.AbuseRateLimitError{Message: "abuse"}
	if !IsRateLimitError(err) {
		t.Error("expected IsRateLimitError to return true for AbuseRateLimitError")
	}
}

func TestIsRateLimitError_Wrapped(t *testing.T) {
	inner := &gh.RateLimitError{Message: "rate limit"}
	wrapped := fmt.Errorf("outer: %w", inner)
	if !IsRateLimitError(wrapped) {
		t.Error("expected IsRateLimitError to detect wrapped RateLimitError")
	}
}

func TestIsRateLimitError_NonRateLimit(t *testing.T) {
	err := errors.New("some other error")
	if IsRateLimitError(err) {
		t.Error("expected IsRateLimitError to return false for non-rate-limit error")
	}
}

func TestRetryAfterFromError_AbuseWithRetryAfter(t *testing.T) {
	d := 5 * time.Minute
	err := &gh.AbuseRateLimitError{
		Message:    "secondary rate limit",
		RetryAfter: &d,
	}
	got := RetryAfterFromError(err)
	if got != 5*time.Minute {
		t.Errorf("RetryAfterFromError = %v, want 5m", got)
	}
}

func TestRetryAfterFromError_AbuseWithoutRetryAfter(t *testing.T) {
	err := &gh.AbuseRateLimitError{Message: "secondary rate limit"}
	got := RetryAfterFromError(err)
	if got != 0 {
		t.Errorf("RetryAfterFromError = %v, want 0", got)
	}
}

func TestRetryAfterFromError_Wrapped(t *testing.T) {
	d := 3 * time.Minute
	inner := &gh.AbuseRateLimitError{
		Message:    "secondary rate limit",
		RetryAfter: &d,
	}
	wrapped := fmt.Errorf("create repo: %w", inner)
	got := RetryAfterFromError(wrapped)
	if got != 3*time.Minute {
		t.Errorf("RetryAfterFromError wrapped = %v, want 3m", got)
	}
}

func TestRetryAfterFromError_NonRateLimit(t *testing.T) {
	err := errors.New("some other error")
	got := RetryAfterFromError(err)
	if got != 0 {
		t.Errorf("RetryAfterFromError non-rate-limit = %v, want 0", got)
	}
}

func TestRetryAfterFromError_PrimaryRateLimit(t *testing.T) {
	err := &gh.RateLimitError{Message: "primary"}
	got := RetryAfterFromError(err)
	if got != 0 {
		t.Errorf("RetryAfterFromError primary = %v, want 0 (only abuse has RetryAfter)", got)
	}
}

func TestRateLimitTransport_Secondary403WithRetryAfter(t *testing.T) {
	attempts := 0
	transport := &rateLimitTransport{
		base: &mockTransportFunc{
			fn: func(req *http.Request) (*http.Response, error) {
				attempts++
				if attempts == 1 {
					return &http.Response{
						StatusCode: http.StatusForbidden,
						Header: http.Header{
							"Retry-After": []string{"1"},
						},
						Body: io.NopCloser(strings.NewReader("secondary rate limit")),
					}, nil
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("ok")),
				}, nil
			},
		},
	}

	req, _ := http.NewRequest("POST", "https://api.github.com/orgs/test/repos", nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts (retry on secondary 403), got %d", attempts)
	}
}
