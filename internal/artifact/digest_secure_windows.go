//go:build windows

package artifact

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/fantasyce/agent-runtime-proof/internal/expectation"
)

func artifactReadingSupported() bool { return false }

func openArtifactRoot(path string) (*os.File, error) { return openNoFollow(path) }

func classifyArtifactOpen(err error) error { return domainError("ARTIFACT_INACCESSIBLE", err) }

func verifyArtifactPath(path string, expected os.FileInfo) error {
	actual, err := os.Lstat(path)
	if err != nil || actual.Mode()&os.ModeSymlink != 0 || !os.SameFile(expected, actual) || fileChanged(stateFromInfo(expected), stateFromInfo(actual)) {
		return domainError("ARTIFACT_CHANGED_DURING_READ", errors.New("artifact root changed during read"))
	}
	return nil
}

func digestDirectory(ctx context.Context, _ *os.File, resolved expectation.Resolved, clock Clock, deadline time.Time) ([]treeEntry, int64, string, error) {
	entries := []treeEntry{}
	seen := map[string]string{}
	var totalBytes int64
	err := filepath.WalkDir(resolved.ArtifactRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return domainError("ARTIFACT_INACCESSIBLE", walkErr)
		}
		if err := checkActive(ctx, clock, deadline); err != nil {
			return err
		}
		if path == resolved.ArtifactRoot {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return domainError("ARTIFACT_SYMLINK_REJECTED", fmt.Errorf("symbolic link %q", entry.Name()))
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return domainError("ARTIFACT_INACCESSIBLE", err)
		}
		if !info.Mode().IsRegular() {
			return domainError("ARTIFACT_UNSUPPORTED_TYPE", fmt.Errorf("unsupported artifact type %q", entry.Name()))
		}
		relative, err := filepath.Rel(resolved.ArtifactRoot, path)
		if err != nil || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return domainError("ARTIFACT_PATH_ESCAPE", errors.New("artifact path escapes root"))
		}
		normalized, err := normalizeRelative(filepath.ToSlash(relative))
		if err != nil {
			return domainError("ARTIFACT_PATH_ESCAPE", err)
		}
		if !resolved.Includes(normalized) {
			return nil
		}
		key := strings.ToLower(normalized)
		if previous, exists := seen[key]; exists {
			return domainError("ARTIFACT_NORMALIZATION_COLLISION", fmt.Errorf("%q collides with %q", normalized, previous))
		}
		seen[key] = normalized
		file, err := openNoFollow(path)
		if err != nil {
			return domainError("ARTIFACT_INACCESSIBLE", err)
		}
		digest, size, openedInfo, err := digestOpenedFile(ctx, file, clock, deadline, resolved.Value.Artifact.MaxBytes-totalBytes)
		file.Close()
		if err != nil {
			return err
		}
		if err := verifyArtifactPath(path, openedInfo); err != nil {
			return err
		}
		if wouldExceed(len(entries), totalBytes, size, resolved.Value.Artifact.MaxFiles, resolved.Value.Artifact.MaxBytes) {
			return domainError("ARTIFACT_SCAN_LIMIT_EXCEEDED", errors.New("artifact exceeds configured limits"))
		}
		totalBytes += size
		entries = append(entries, treeEntry{Path: normalized, SHA256: digest, Size: size})
		return nil
	})
	slices.SortFunc(entries, func(left, right treeEntry) int { return strings.Compare(left.Path, right.Path) })
	return entries, totalBytes, "", err
}

func fileIdentity(os.FileInfo) string { return "" }
