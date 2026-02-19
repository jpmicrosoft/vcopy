package github

import (
	"io"
	"os"
)

// UploadFile wraps an io.Reader into a temporary *os.File for the GitHub API.
type UploadFile struct {
	*os.File
}

// NewUploadFile creates a temp file from a reader for upload.
func NewUploadFile(r io.Reader, name string, size int64) *UploadFile {
	tmpFile, err := os.CreateTemp("", "vcopy-asset-*")
	if err != nil {
		return nil
	}
	if _, err := io.Copy(tmpFile, r); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil
	}
	tmpFile.Seek(0, 0)
	return &UploadFile{File: tmpFile}
}

// Cleanup removes the temporary file.
func (f *UploadFile) Cleanup() {
	if f != nil && f.File != nil {
		f.Close()
		os.Remove(f.Name())
	}
}
