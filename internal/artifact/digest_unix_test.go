//go:build !windows

package artifact

import (
	"context"
	"path/filepath"
	"syscall"
	"testing"
)

func TestDigestRejectsUnsupportedFileType(t *testing.T) {
	root := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(root, "pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Digest(context.Background(), resolvedFor(root, []string{"**"}, nil, 10, 1024, 1000), fixedClock{})
	assertReason(t, err, "ARTIFACT_UNSUPPORTED_TYPE")
}
