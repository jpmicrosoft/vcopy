package github

import (
	"fmt"
	"io"
	"os"
)

// UploadFile wraps an io.Reader into a temporary *os.File for the GitHub API.
type UploadFile struct {
	*os.File
}

// NewUploadFile creates a temp file from a reader for upload.
func NewUploadFile(r io.Reader, name string, size int64) (*UploadFile, error) {
	tmpFile, err := os.CreateTemp("", "vcopy-asset-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file for asset %s: %w", name, err)
	}
	if _, err := io.Copy(tmpFile, r); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to write asset %s to temp file: %w", name, err)
	}
	if _, err := tmpFile.Seek(0, 0); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to seek temp file for asset %s: %w", name, err)
	}
	return &UploadFile{File: tmpFile}, nil
}

// Cleanup removes the temporary file.
func (f *UploadFile) Cleanup() {
	if f != nil && f.File != nil {
		f.Close()
		os.Remove(f.Name())
	}
}
