//go:build windows

package artifact

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/fantasyce/agent-runtime-proof/internal/expectation"
	"golang.org/x/sys/windows"
)

var errLinkRejected = errors.New("symbolic link or reparse point rejected")

const artifactReadAccess = windows.FILE_READ_DATA | windows.FILE_READ_ATTRIBUTES | windows.SYNCHRONIZE

func artifactReadingSupported() bool { return true }

type pinnedSnapshot struct {
	path   string
	state  fileState
	digest string
}

func openArtifactRoot(path string) (*os.File, error) {
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		return nil, errors.New("artifact root is not absolute")
	}
	volume := filepath.VolumeName(cleaned)
	if len(volume) != 2 || volume[1] != ':' {
		return nil, errors.New("artifact root is not on a local drive volume")
	}
	current, err := openWindowsVolumeRoot(volume + `\`)
	if err != nil {
		return nil, err
	}
	relative := strings.TrimLeft(strings.TrimPrefix(cleaned, volume), `\/`)
	if relative == "" {
		return current, nil
	}
	for _, component := range strings.FieldsFunc(relative, func(value rune) bool { return value == '\\' || value == '/' }) {
		next, openErr := openChildNoFollow(current, component)
		current.Close()
		if openErr != nil {
			return nil, openErr
		}
		current = next
	}
	return current, nil
}

func openWindowsVolumeRoot(path string) (*os.File, error) {
	pathPointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(pathPointer, artifactReadAccess,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	if err := rejectReparseHandle(handle); err != nil {
		windows.CloseHandle(handle)
		return nil, err
	}
	return os.NewFile(uintptr(handle), path), nil
}

func openChildNoFollow(parent *os.File, name string) (*os.File, error) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `\/:`) || strings.ContainsRune(name, 0) {
		return nil, errors.New("invalid artifact path component")
	}
	objectName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return nil, err
	}
	attributes := &windows.OBJECT_ATTRIBUTES{RootDirectory: windows.Handle(parent.Fd()), ObjectName: objectName, Attributes: windows.OBJ_CASE_INSENSITIVE}
	attributes.Length = uint32(unsafe.Sizeof(*attributes))
	var status windows.IO_STATUS_BLOCK
	var handle windows.Handle
	err = windows.NtCreateFile(&handle, artifactReadAccess, attributes, &status, nil, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, windows.FILE_OPEN,
		windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
	if err != nil {
		return nil, err
	}
	if err := rejectReparseHandle(handle); err != nil {
		windows.CloseHandle(handle)
		return nil, err
	}
	return os.NewFile(uintptr(handle), name), nil
}

func rejectReparseHandle(handle windows.Handle) error {
	var information windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &information); err != nil {
		return err
	}
	if information.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return errLinkRejected
	}
	return nil
}

func classifyArtifactOpen(err error) error {
	if errors.Is(err, errLinkRejected) {
		return domainError("ARTIFACT_SYMLINK_REJECTED", err)
	}
	return domainError("ARTIFACT_INACCESSIBLE", err)
}

func verifyArtifactPath(ctx context.Context, path string, expected fileState, expectedDigest string, clock Clock, deadline time.Time) error {
	actual, err := openArtifactRoot(path)
	if err != nil {
		return domainError("ARTIFACT_CHANGED_DURING_READ", err)
	}
	defer actual.Close()
	actualState, err := fileStateFromFile(actual)
	if err != nil || fileChanged(expected, actualState) {
		return domainError("ARTIFACT_CHANGED_DURING_READ", errors.New("artifact root changed during read"))
	}
	if expectedDigest != "" {
		if err := verifyPinnedContent(ctx, actual, expected, expectedDigest, clock, deadline); err != nil {
			return err
		}
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
		key := strings.ToLower(normalized)
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
		if digestErr != nil {
			child.Close()
			return digestErr
		}
		child.Close()
		if err := verifyPinnedChild(directory, childEntry.Name(), openedState); err != nil {
			return err
		}
		*totalBytes += size
		*entries = append(*entries, treeEntry{Path: normalized, SHA256: digest, Size: size})
		*snapshots = append(*snapshots, pinnedSnapshot{path: normalized, state: openedState, digest: digest})
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

func verifyRelativeSnapshot(ctx context.Context, root *os.File, snapshot pinnedSnapshot, clock Clock, deadline time.Time) error {
	actual, err := openRelativeNoFollow(root, snapshot.path)
	if err != nil {
		return domainError("ARTIFACT_CHANGED_DURING_READ", err)
	}
	defer actual.Close()
	actualState, err := fileStateFromFile(actual)
	if err != nil || fileChanged(snapshot.state, actualState) {
		return domainError("ARTIFACT_CHANGED_DURING_READ", errors.New("artifact snapshot changed before completion"))
	}
	if snapshot.digest != "" {
		if err := verifyPinnedContent(ctx, actual, snapshot.state, snapshot.digest, clock, deadline); err != nil {
			return err
		}
	}
	return nil
}

func verifyPinnedContent(ctx context.Context, file *os.File, expected fileState, expectedDigest string, clock Clock, deadline time.Time) error {
	digest, size, state, err := digestOpenedFile(ctx, file, clock, deadline, expected.size)
	if err != nil {
		return err
	}
	if size != expected.size || fileChanged(expected, state) || digest != expectedDigest {
		return domainError("ARTIFACT_CHANGED_DURING_READ", errors.New("artifact content changed before completion"))
	}
	return nil
}

func openRelativeNoFollow(root *os.File, relative string) (*os.File, error) {
	if relative == "" {
		return duplicateWindowsFile(root)
	}
	current := root
	owned := false
	for _, component := range strings.Split(relative, "/") {
		next, openErr := openChildNoFollow(current, component)
		if owned {
			current.Close()
		}
		if openErr != nil {
			return nil, openErr
		}
		current = next
		owned = true
	}
	return current, nil
}

func duplicateWindowsFile(file *os.File) (*os.File, error) {
	process, err := windows.GetCurrentProcess()
	if err != nil {
		return nil, err
	}
	var duplicate windows.Handle
	if err := windows.DuplicateHandle(process, windows.Handle(file.Fd()), process, &duplicate, 0, false, windows.DUPLICATE_SAME_ACCESS); err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(duplicate), file.Name()), nil
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

type windowsFileIDInfo struct {
	VolumeSerialNumber uint64
	FileID             [16]byte
}

type windowsFileBasicInfo struct {
	CreationTime   int64
	LastAccessTime int64
	LastWriteTime  int64
	ChangeTime     int64
	FileAttributes uint32
	_              uint32
}

func fileIdentity(file *os.File) string {
	var information windowsFileIDInfo
	err := windows.GetFileInformationByHandleEx(windows.Handle(file.Fd()), windows.FileIdInfo,
		(*byte)(unsafe.Pointer(&information)), uint32(unsafe.Sizeof(information)))
	if err == nil {
		return fmt.Sprintf("%016x:%s", information.VolumeSerialNumber, hex.EncodeToString(information.FileID[:]))
	}
	var fallback syscall.ByHandleFileInformation
	if fallbackErr := syscall.GetFileInformationByHandle(syscall.Handle(file.Fd()), &fallback); fallbackErr != nil {
		return ""
	}
	return fmt.Sprintf("%08x:%08x%08x", fallback.VolumeSerialNumber, fallback.FileIndexHigh, fallback.FileIndexLow)
}

func fileStateFromFile(file *os.File) (fileState, error) {
	info, err := file.Stat()
	if err != nil {
		return fileState{}, err
	}
	state := stateFromInfo(info)
	state.identity = fileIdentity(file)
	var basic windowsFileBasicInfo
	if err := windows.GetFileInformationByHandleEx(windows.Handle(file.Fd()), windows.FileBasicInfo,
		(*byte)(unsafe.Pointer(&basic)), uint32(unsafe.Sizeof(basic))); err != nil {
		return fileState{}, err
	}
	state.changeToken = fmt.Sprintf("%d", basic.ChangeTime)
	return state, nil
}
