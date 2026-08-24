//go:build !windows

package artifact

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/fantasyce/agent-runtime-proof/internal/expectation"
	"golang.org/x/sys/unix"
)

var errLinkRejected = errors.New("symbolic link rejected")

func artifactReadingSupported() bool { return true }

type pinnedSnapshot struct {
	path  string
	state fileState
}

func openArtifactRoot(path string) (*os.File, error) {
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		return nil, errors.New("artifact root is not absolute")
	}
	descriptor, err := unix.Open(string(filepath.Separator), unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY, 0)
	if err != nil {
		return nil, err
	}
	current := os.NewFile(uintptr(descriptor), string(filepath.Separator))
	components := strings.Split(strings.TrimPrefix(cleaned, string(filepath.Separator)), string(filepath.Separator))
	if len(components) == 1 && components[0] == "" {
		return current, nil
	}
	for index, component := range components {
		flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW | unix.O_NONBLOCK
		if index < len(components)-1 {
			flags |= unix.O_DIRECTORY
		}
		nextDescriptor, openErr := unix.Openat(int(current.Fd()), component, flags, 0)
		if openErr != nil {
			current.Close()
			if errors.Is(openErr, unix.ELOOP) || errors.Is(openErr, unix.ENOTDIR) {
				return nil, errLinkRejected
			}
			return nil, openErr
		}
		next := os.NewFile(uintptr(nextDescriptor), component)
		current.Close()
		current = next
	}
	return current, nil
}

func openChildNoFollow(parent *os.File, name string) (*os.File, error) {
	descriptor, err := unix.Openat(int(parent.Fd()), name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		if errors.Is(err, unix.ELOOP) || errors.Is(err, unix.ENOTDIR) {
			return nil, errLinkRejected
		}
		return nil, err
	}
	return os.NewFile(uintptr(descriptor), name), nil
}

func classifyArtifactOpen(err error) error {
	if errors.Is(err, errLinkRejected) {
		return domainError("ARTIFACT_SYMLINK_REJECTED", err)
	}
	return domainError("ARTIFACT_INACCESSIBLE", err)
}

func verifyArtifactPath(_ context.Context, path string, expected fileState, _ string, _ Clock, _ time.Time) error {
	actual, err := openArtifactRoot(path)
	if err != nil {
		return domainError("ARTIFACT_CHANGED_DURING_READ", err)
	}
	defer actual.Close()
	actualState, err := fileStateFromFile(actual)
	if err != nil || fileChanged(expected, actualState) {
		return domainError("ARTIFACT_CHANGED_DURING_READ", errors.New("artifact root changed during read"))
	}
	return nil
}

func digestDirectory(ctx context.Context, root *os.File, resolved expectation.Resolved, clock Clock, deadline time.Time) ([]treeEntry, int64, string, error) {
	entries := []treeEntry{}
	seen := map[string]string{}
	var totalBytes int64
	var entrypointIdentity string
	rootState, err := fileStateFromFile(root)
	if err != nil {
		return nil, 0, "", domainError("ARTIFACT_INACCESSIBLE", err)
	}
	snapshots := []pinnedSnapshot{{path: "", state: rootState}}
	err = walkPinnedDirectory(ctx, root, "", resolved, clock, deadline, &entries, seen, &totalBytes, &entrypointIdentity, &snapshots)
	if err != nil {
		return nil, 0, "", err
	}
	for _, snapshot := range snapshots {
		if err := verifyRelativeSnapshot(ctx, root, snapshot, clock, deadline); err != nil {
			return nil, 0, "", err
		}
	}
	slices.SortFunc(entries, func(left, right treeEntry) int { return strings.Compare(left.Path, right.Path) })
	return entries, totalBytes, entrypointIdentity, nil
}

func walkPinnedDirectory(ctx context.Context, directory *os.File, prefix string, resolved expectation.Resolved, clock Clock, deadline time.Time, entries *[]treeEntry, seen map[string]string, totalBytes *int64, entrypointIdentity *string, snapshots *[]pinnedSnapshot) error {
	before, err := fileStateFromFile(directory)
	if err != nil {
		return domainError("ARTIFACT_INACCESSIBLE", err)
	}
	children, err := directory.ReadDir(-1)
	if err != nil {
		return domainError("ARTIFACT_INACCESSIBLE", err)
	}
	slices.SortFunc(children, func(left, right os.DirEntry) int { return strings.Compare(left.Name(), right.Name()) })
	for _, childEntry := range children {
		if err := checkActive(ctx, clock, deadline); err != nil {
			return err
		}
		child, err := openChildNoFollow(directory, childEntry.Name())
		if err != nil {
			return classifyArtifactOpen(err)
		}
		info, statErr := child.Stat()
		if statErr != nil {
			child.Close()
			return domainError("ARTIFACT_INACCESSIBLE", statErr)
		}
		relative := childEntry.Name()
		if prefix != "" {
			relative = prefix + "/" + childEntry.Name()
		}
		normalized, normalizeErr := normalizeRelative(relative)
		if normalizeErr != nil {
			child.Close()
			return domainError("ARTIFACT_PATH_ESCAPE", normalizeErr)
		}
		if info.IsDir() {
			state, stateErr := fileStateFromFile(child)
			if stateErr != nil {
				child.Close()
				return domainError("ARTIFACT_INACCESSIBLE", stateErr)
			}
			*snapshots = append(*snapshots, pinnedSnapshot{path: normalized, state: state})
			err = walkPinnedDirectory(ctx, child, normalized, resolved, clock, deadline, entries, seen, totalBytes, entrypointIdentity, snapshots)
			child.Close()
			if err != nil {
				return err
			}
			if err := verifyPinnedChild(directory, childEntry.Name(), state); err != nil {
				return err
			}
			continue
		}
		if !info.Mode().IsRegular() {
			child.Close()
			return domainError("ARTIFACT_UNSUPPORTED_TYPE", fmt.Errorf("unsupported artifact type %q", childEntry.Name()))
		}
		if !resolved.Includes(normalized) {
			child.Close()
			continue
		}
		key := normalized
		if runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		if previous, exists := seen[key]; exists {
			child.Close()
			return domainError("ARTIFACT_NORMALIZATION_COLLISION", fmt.Errorf("%q collides with %q", normalized, previous))
		}
		seen[key] = normalized
		if wouldExceed(len(*entries), *totalBytes, info.Size(), resolved.Value.Artifact.MaxFiles, resolved.Value.Artifact.MaxBytes) {
			child.Close()
			return domainError("ARTIFACT_SCAN_LIMIT_EXCEEDED", errors.New("artifact exceeds configured limits"))
		}
		digest, size, openedState, digestErr := digestOpenedFile(ctx, child, clock, deadline, resolved.Value.Artifact.MaxBytes-*totalBytes)
		child.Close()
		if digestErr != nil {
			return digestErr
		}
		if err := verifyPinnedChild(directory, childEntry.Name(), openedState); err != nil {
			return err
		}
		*totalBytes += size
		*entries = append(*entries, treeEntry{Path: normalized, SHA256: digest, Size: size})
		*snapshots = append(*snapshots, pinnedSnapshot{path: normalized, state: openedState})
		entrypoint, _ := normalizeRelative(filepath.ToSlash(resolved.Value.Launch.Entrypoint))
		if normalized == entrypoint {
			*entrypointIdentity = openedState.identity
		}
	}
	after, err := fileStateFromFile(directory)
	if err != nil || fileChanged(before, after) {
		return domainError("ARTIFACT_CHANGED_DURING_READ", errors.New("artifact directory changed during read"))
	}
	return nil
}

func verifyRelativeSnapshot(_ context.Context, root *os.File, snapshot pinnedSnapshot, _ Clock, _ time.Time) error {
	actual, err := openRelativeNoFollow(root, snapshot.path)
	if err != nil {
		return domainError("ARTIFACT_CHANGED_DURING_READ", err)
	}
	defer actual.Close()
	actualState, err := fileStateFromFile(actual)
	if err != nil || fileChanged(snapshot.state, actualState) {
		return domainError("ARTIFACT_CHANGED_DURING_READ", errors.New("artifact snapshot changed before completion"))
	}
	return nil
}

func openRelativeNoFollow(root *os.File, relative string) (*os.File, error) {
	descriptor, err := unix.Dup(int(root.Fd()))
	if err != nil {
		return nil, err
	}
	current := os.NewFile(uintptr(descriptor), "artifact-root")
	if relative == "" {
		return current, nil
	}
	for _, component := range strings.Split(relative, "/") {
		next, openErr := openChildNoFollow(current, component)
		current.Close()
		if openErr != nil {
			return nil, openErr
		}
		current = next
	}
	return current, nil
}

func verifyPinnedChild(parent *os.File, name string, expected fileState) error {
	actual, err := openChildNoFollow(parent, name)
	if err != nil {
		return domainError("ARTIFACT_CHANGED_DURING_READ", err)
	}
	defer actual.Close()
	actualState, err := fileStateFromFile(actual)
	if err != nil || fileChanged(expected, actualState) {
		return domainError("ARTIFACT_CHANGED_DURING_READ", errors.New("artifact entry changed during read"))
	}
	return nil
}

func fileIdentity(file *os.File) string {
	info, err := file.Stat()
	if err != nil {
		return ""
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%d:%d", stat.Dev, stat.Ino)
}

func fileStateFromFile(file *os.File) (fileState, error) {
	info, err := file.Stat()
	if err != nil {
		return fileState{}, err
	}
	state := stateFromInfo(info)
	state.identity = fileIdentity(file)
	return state, nil
}
