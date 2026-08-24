package artifact

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/fantasyce/agent-runtime-proof/internal/canonical"
	"github.com/fantasyce/agent-runtime-proof/internal/expectation"
	"github.com/fantasyce/agent-runtime-proof/internal/model"
	"golang.org/x/text/unicode/norm"
)

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

type treeEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type fileState struct {
	size        int64
	modUnixNano int64
	identity    string
	changeToken string
	info        os.FileInfo
}

// ReadingSupported reports whether the current platform provides the
// handle-pinned artifact traversal required by Digest.
func ReadingSupported() bool { return artifactReadingSupported() }

func Digest(ctx context.Context, resolved expectation.Resolved, clock Clock) (model.ArtifactObservation, error) {
	if !artifactReadingSupported() {
		return model.ArtifactObservation{}, domainError("PLATFORM_EVIDENCE_UNAVAILABLE", errors.New("safe handle-relative artifact traversal is unavailable on this platform"))
	}
	if clock == nil {
		clock = realClock{}
	}
	start := clock.Now()
	deadline := start.Add(time.Duration(resolved.Value.Artifact.MaxDurationMS) * time.Millisecond)
	if err := checkActive(ctx, clock, deadline); err != nil {
		return model.ArtifactObservation{}, err
	}
	rootFile, err := openArtifactRoot(resolved.ArtifactRoot)
	if err != nil {
		return model.ArtifactObservation{}, classifyArtifactOpen(err)
	}
	defer rootFile.Close()
	rootState, err := fileStateFromFile(rootFile)
	if err != nil {
		return model.ArtifactObservation{}, domainError("ARTIFACT_INACCESSIBLE", err)
	}
	rootInfo := rootState.info
	if rootInfo.Mode().IsRegular() {
		digest, size, openedState, err := digestOpenedFile(ctx, rootFile, clock, deadline, resolved.Value.Artifact.MaxBytes)
		if err != nil {
			return model.ArtifactObservation{}, err
		}
		if wouldExceed(0, 0, size, resolved.Value.Artifact.MaxFiles, resolved.Value.Artifact.MaxBytes) {
			return model.ArtifactObservation{}, domainError("ARTIFACT_SCAN_LIMIT_EXCEEDED", errors.New("artifact exceeds configured limits"))
		}
		if err := verifyArtifactPath(ctx, resolved.ArtifactRoot, openedState, digest, clock, deadline); err != nil {
			return model.ArtifactObservation{}, err
		}
		return model.ArtifactObservation{SHA256: digest, FileCount: 1, ByteCount: size, DurationMS: elapsedMS(start, clock.Now()), EntrypointFileIdentity: openedState.identity}, nil
	}
	if !rootInfo.IsDir() {
		return model.ArtifactObservation{}, domainError("ARTIFACT_UNSUPPORTED_TYPE", errors.New("artifact root is neither a regular file nor a directory"))
	}

	entries, totalBytes, entrypointIdentity, err := digestDirectory(ctx, rootFile, resolved, clock, deadline)
	if err != nil {
		return model.ArtifactObservation{}, err
	}
	if err := verifyArtifactPath(ctx, resolved.ArtifactRoot, rootState, "", clock, deadline); err != nil {
		return model.ArtifactObservation{}, err
	}
	if len(entries) == 0 {
		return model.ArtifactObservation{}, domainError("EXPECTATION_INVALID", errors.New("artifact patterns select no files"))
	}
	slices.SortFunc(entries, func(left, right treeEntry) int { return strings.Compare(left.Path, right.Path) })
	encoded, err := canonical.Marshal(entries)
	if err != nil {
		return model.ArtifactObservation{}, domainError("EXPECTATION_INVALID", err)
	}
	digest := sha256.Sum256(encoded)
	return model.ArtifactObservation{
		SHA256: hex.EncodeToString(digest[:]), FileCount: len(entries), ByteCount: totalBytes, DurationMS: elapsedMS(start, clock.Now()), EntrypointFileIdentity: entrypointIdentity,
	}, nil
}

func digestOpenedFile(ctx context.Context, file *os.File, clock Clock, deadline time.Time, maxBytes int64) (string, int64, fileState, error) {
	before, err := fileStateFromFile(file)
	if err != nil {
		return "", 0, fileState{}, domainError("ARTIFACT_INACCESSIBLE", err)
	}
	beforeInfo := before.info
	if !beforeInfo.Mode().IsRegular() {
		return "", 0, fileState{}, domainError("ARTIFACT_UNSUPPORTED_TYPE", errors.New("artifact is not a regular file"))
	}
	if beforeInfo.Size() > maxBytes {
		return "", 0, fileState{}, domainError("ARTIFACT_SCAN_LIMIT_EXCEEDED", errors.New("artifact exceeds configured byte limit"))
	}
	hash := sha256.New()
	buffer := make([]byte, 64*1024)
	var bytesRead int64
	for {
		if err := checkActive(ctx, clock, deadline); err != nil {
			return "", 0, fileState{}, err
		}
		count, readErr := file.Read(buffer)
		if count > 0 {
			if int64(count) > maxBytes-bytesRead {
				return "", 0, fileState{}, domainError("ARTIFACT_SCAN_LIMIT_EXCEEDED", errors.New("artifact grew beyond configured byte limit"))
			}
			bytesRead += int64(count)
			if _, err := hash.Write(buffer[:count]); err != nil {
				return "", 0, fileState{}, domainError("ARTIFACT_INACCESSIBLE", err)
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return "", 0, fileState{}, domainError("ARTIFACT_INACCESSIBLE", readErr)
		}
	}
	after, err := fileStateFromFile(file)
	if err != nil {
		return "", 0, fileState{}, domainError("ARTIFACT_INACCESSIBLE", err)
	}
	if fileChanged(before, after) {
		return "", 0, fileState{}, domainError("ARTIFACT_CHANGED_DURING_READ", errors.New("artifact identity, size, or modification time changed"))
	}
	return hex.EncodeToString(hash.Sum(nil)), before.size, before, nil
}

func stateFromInfo(info os.FileInfo) fileState {
	return fileState{size: info.Size(), modUnixNano: info.ModTime().UnixNano(), changeToken: fileChangeToken(info), info: info}
}

func fileChanged(before, after fileState) bool {
	if before.size != after.size || before.modUnixNano != after.modUnixNano || before.changeToken != after.changeToken {
		return true
	}
	if before.identity != "" || after.identity != "" {
		return before.identity != after.identity
	}
	return before.info != nil && after.info != nil && !os.SameFile(before.info, after.info)
}

func normalizeRelative(value string) (string, error) {
	if value == "" || strings.HasPrefix(value, "/") || strings.ContainsRune(value, 0) {
		return "", errors.New("invalid artifact relative path")
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", errors.New("invalid artifact path segment")
		}
	}
	return norm.NFC.String(value), nil
}

func checkActive(ctx context.Context, clock Clock, deadline time.Time) error {
	if err := ctx.Err(); err != nil {
		return domainError("SCAN_CANCELLED", err)
	}
	if clock.Now().After(deadline) {
		return domainError("ARTIFACT_SCAN_LIMIT_EXCEEDED", errors.New("artifact scan exceeded time limit"))
	}
	return nil
}

func elapsedMS(start, end time.Time) int64 {
	duration := end.Sub(start)
	if duration < 0 {
		return 0
	}
	return duration.Milliseconds()
}

func wouldExceed(currentFiles int, currentBytes, nextBytes int64, maxFiles int, maxBytes int64) bool {
	return currentFiles+1 > maxFiles || nextBytes < 0 || currentBytes > maxBytes-nextBytes
}
