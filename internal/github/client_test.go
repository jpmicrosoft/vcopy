package github

import (
	"testing"
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

func TestNewUploadFile_NilOnBadReader(t *testing.T) {
	// Test with a reader that fails immediately
	f := NewUploadFile(&failReader{}, "test.bin", 100)
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
