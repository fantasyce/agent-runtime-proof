package artifact

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fantasyce/agent-runtime-proof/internal/expectation"
)

func TestDigestHashesSingleFileBytes(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime")
	if err := os.WriteFile(root, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := Digest(context.Background(), resolvedFor(root, []string{"**"}, nil, 10, 1024, 1000), fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	if result.SHA256 != "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Fatalf("digest = %s", result.SHA256)
	}
	if result.FileCount != 1 || result.ByteCount != 5 {
		t.Fatalf("counts = %d files, %d bytes", result.FileCount, result.ByteCount)
	}
}

func TestDigestHashesCanonicalDirectoryTree(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "a.txt"), "alpha")
	writeFile(t, filepath.Join(root, "sub", "b.txt"), "beta")
	writeFile(t, filepath.Join(root, "ignored.log"), "not-hashed")

	result, err := Digest(context.Background(), resolvedFor(root, []string{"**/*.txt"}, []string{"ignored*"}, 10, 1024, 1000), fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	if result.SHA256 != "31cd577e9b5628134eb431249617f669bb243dbc8bc22b7579356525b1c36b32" {
		t.Fatalf("tree digest = %s", result.SHA256)
	}
	if result.FileCount != 2 || result.ByteCount != 9 {
		t.Fatalf("counts = %d files, %d bytes", result.FileCount, result.ByteCount)
	}
}

func TestDigestPreservesSafeUnicodeAndSpacePath(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "子 目录")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(directory, "文件.txt"), "data")
	result, err := Digest(context.Background(), resolvedFor(root, []string{"**"}, nil, 10, 1024, 1000), fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	if result.SHA256 != "caff76747e323b00a57107745f86f33a9e574b210c292838a3e402d937305594" {
		t.Fatalf("tree digest = %s", result.SHA256)
	}
}

func TestDigestRejectsSymlinkWithoutPartialHash(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "target"), "content")
	if err := os.Symlink("target", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	result, err := Digest(context.Background(), resolvedFor(root, []string{"**"}, nil, 10, 1024, 1000), fixedClock{})
	assertReason(t, err, "ARTIFACT_SYMLINK_REJECTED")
	if result.SHA256 != "" {
		t.Fatalf("partial digest returned: %s", result.SHA256)
	}
}

func TestDigestRejectsEmptySelection(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "runtime.bin"), "content")
	_, err := Digest(context.Background(), resolvedFor(root, []string{"*.txt"}, nil, 10, 1024, 1000), fixedClock{})
	assertReason(t, err, "EXPECTATION_INVALID")
}

func TestDigestEnforcesFileAndByteLimits(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a"), "1234")
	writeFile(t, filepath.Join(root, "b"), "5678")
	for _, test := range []struct {
		name     string
		maxFiles int
		maxBytes int64
	}{
		{"files", 1, 100},
		{"bytes", 10, 7},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := Digest(context.Background(), resolvedFor(root, []string{"**"}, nil, test.maxFiles, test.maxBytes, 1000), fixedClock{})
			assertReason(t, err, "ARTIFACT_SCAN_LIMIT_EXCEEDED")
			if result.SHA256 != "" {
				t.Fatalf("partial digest returned: %s", result.SHA256)
			}
		})
	}
}

func TestDigestHonorsCancellationAndTimeLimit(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "runtime"), "content")
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Digest(cancelled, resolvedFor(root, []string{"**"}, nil, 10, 1024, 1000), fixedClock{})
	assertReason(t, err, "SCAN_CANCELLED")

	_, err = Digest(context.Background(), resolvedFor(root, []string{"**"}, nil, 10, 1024, 1), &steppingClock{step: 2 * time.Millisecond})
	assertReason(t, err, "ARTIFACT_SCAN_LIMIT_EXCEEDED")
}

func TestNormalizedPathCollisionKeyUsesNFC(t *testing.T) {
	left, err := normalizeRelative("caf\u00e9.txt")
	if err != nil {
		t.Fatal(err)
	}
	right, err := normalizeRelative("cafe\u0301.txt")
	if err != nil {
		t.Fatal(err)
	}
	if left != right {
		t.Fatalf("normalized paths differ: %q != %q", left, right)
	}
}

func TestFileChangedDetectsIdentitySizeOrTimeMutation(t *testing.T) {
	base := fileState{size: 4, modUnixNano: 10, identity: "dev:1:ino:2"}
	for _, changed := range []fileState{
		{size: 5, modUnixNano: 10, identity: "dev:1:ino:2"},
		{size: 4, modUnixNano: 11, identity: "dev:1:ino:2"},
		{size: 4, modUnixNano: 10, identity: "dev:1:ino:3"},
	} {
		if !fileChanged(base, changed) {
			t.Fatalf("mutation not detected: %#v", changed)
		}
	}
}

func TestWouldExceedLimitsUsesPostOpenFileSize(t *testing.T) {
	if !wouldExceed(0, 6, 5, 10, 10) {
		t.Fatal("post-open byte growth was not limited")
	}
	if wouldExceed(0, 5, 5, 1, 10) {
		t.Fatal("exact file and byte limits were rejected")
	}
}

func resolvedFor(root string, include, exclude []string, maxFiles int, maxBytes, maxDurationMS int64) expectation.Resolved {
	resolved := expectation.Resolved{ArtifactRoot: root}
	resolved.Value.Artifact.Include = include
	resolved.Value.Artifact.Exclude = exclude
	resolved.Value.Artifact.MaxFiles = maxFiles
	resolved.Value.Artifact.MaxBytes = maxBytes
	resolved.Value.Artifact.MaxDurationMS = maxDurationMS
	return resolved
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertReason(t *testing.T, err error, reason string) {
	t.Helper()
	var domainError *Error
	if !errors.As(err, &domainError) || domainError.Reason != reason {
		t.Fatalf("error = %v, want reason %s", err, reason)
	}
}

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Unix(100, 0) }

type steppingClock struct {
	step time.Duration
	now  time.Time
}

func (clock *steppingClock) Now() time.Time {
	clock.now = clock.now.Add(clock.step)
	return clock.now
}
