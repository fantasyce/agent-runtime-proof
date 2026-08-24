//go:build !windows

package artifact

import (
	"context"
	"os"
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

func TestOpenArtifactRootRejectsSymlinkInAnyAncestor(t *testing.T) {
	directory := t.TempDir()
	directory, err := filepath.EvalSymlinks(directory)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(directory, "target")
	if err := os.MkdirAll(filepath.Join(target, "payload"), 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if file, err := openArtifactRoot(filepath.Join(link, "payload")); err == nil {
		file.Close()
		t.Fatal("artifact root traversed a symbolic-link ancestor")
	}
}
