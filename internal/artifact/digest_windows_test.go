//go:build windows

package artifact

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestDigestRejectsNestedJunctionWithoutReadingTarget(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	target := filepath.Join(base, "outside")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(target, "secret.txt"), "must-not-be-read")
	createJunction(t, filepath.Join(root, "junction"), target)

	result, err := Digest(context.Background(), resolvedFor(root, []string{"**"}, nil, 10, 1024, 1000), fixedClock{})
	assertReason(t, err, "ARTIFACT_SYMLINK_REJECTED")
	if result.SHA256 != "" {
		t.Fatalf("partial digest returned: %s", result.SHA256)
	}
}

func TestOpenArtifactRootRejectsJunctionInRootPath(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(target, "runtime"), "must-not-be-read")
	junction := filepath.Join(base, "root-junction")
	createJunction(t, junction, target)

	resolved := resolvedFor(target, []string{"**"}, nil, 10, 1024, 1000)
	resolved.ArtifactRoot = junction
	result, err := Digest(context.Background(), resolved, fixedClock{})
	assertReason(t, err, "ARTIFACT_SYMLINK_REJECTED")
	if result.SHA256 != "" {
		t.Fatalf("partial digest returned: %s", result.SHA256)
	}
}

func TestFinalContentBarrierPreservesScanLimitReason(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime")
	writeFile(t, path, "content")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	state, err := fileStateFromFile(file)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("content"))
	err = verifyPinnedContent(context.Background(), file, state, hex.EncodeToString(digest[:]), fixedClock{}, time.Unix(99, 0))
	assertReason(t, err, "ARTIFACT_SCAN_LIMIT_EXCEEDED")
}

func createJunction(t *testing.T, link, target string) {
	t.Helper()
	output, err := exec.Command("cmd.exe", "/d", "/c", "mklink", "/J", link, target).CombinedOutput()
	if err != nil {
		t.Fatalf("create junction: %v: %s", err, output)
	}
}
