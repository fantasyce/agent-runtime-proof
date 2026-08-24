//go:build linux

package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/fantasyce/agent-runtime-proof/internal/model"
	gopsprocess "github.com/shirou/gopsutil/v4/process"
)

type nativeObserver struct {
	bootHash string
}

func NewObserver() Observer {
	bootID, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return &nativeObserver{bootHash: hashIdentifier("arp:boot:v1", "unavailable")}
	}
	return &nativeObserver{bootHash: hashIdentifier("arp:boot:v1", strings.TrimSpace(string(bootID)))}
}

func (observer *nativeObserver) Snapshot(ctx context.Context, pid int) (model.Candidate, error) {
	if err := ctx.Err(); err != nil {
		return model.Candidate{}, &Error{Kind: ErrorInternal, Operation: "snapshot", Err: err}
	}
	if pid <= 0 {
		return model.Candidate{}, &Error{Kind: ErrorNotFound, Operation: "snapshot", Err: errors.New("PID must be positive")}
	}
	value, err := gopsprocess.NewProcessWithContext(ctx, int32(pid))
	if err != nil {
		return model.Candidate{}, classifyPortableError("read process identity", err)
	}
	createdMS, err := value.CreateTimeWithContext(ctx)
	if err != nil {
		return model.Candidate{}, classifyPortableError("read process creation time", err)
	}
	parentPID, _ := value.PpidWithContext(ctx)
	candidate := model.Candidate{
		Platform: model.Platform{OS: runtime.GOOS, Arch: runtime.GOARCH},
		Process: model.ProcessIdentity{
			PID: pid, CreatedAtUnixNano: strconv.FormatInt(createdMS*1_000_000, 10), BootIDHash: observer.bootHash,
		},
		ParentPID: int(parentPID), Inaccessible: []string{},
	}
	procExecutable := filepath.Join("/proc", strconv.Itoa(pid), "exe")
	linkedPath, err := os.Readlink(procExecutable)
	if err != nil {
		candidate.Inaccessible = []string{"process.image"}
		return candidate, classifyPortableError("read process image", err)
	}
	const deletedSuffix = " (deleted)"
	cleanPath := strings.TrimSuffix(linkedPath, deletedSuffix)
	candidate.ExecutableDeleted = cleanPath != linkedPath
	fileInfo, err := os.Stat(procExecutable)
	if err != nil {
		candidate.Inaccessible = []string{"process.image.file_identity"}
		return candidate, classifyPortableError("stat process image", err)
	}
	identity, err := linuxFileIdentity(fileInfo)
	if err != nil {
		return candidate, &Error{Kind: ErrorInternal, Operation: "identify process image", Err: err}
	}
	candidate.ExecutablePath = procExecutable
	candidate.DeclaredExecutablePath = cleanPath
	candidate.Executable = model.ExecutableObservation{
		Basename:   filepath.Base(cleanPath),
		PathHash:   hashIdentifier("arp:path:v1", cleanPath),
		FileIDHash: hashIdentifier("arp:file-id:v1", identity),
	}
	return candidate, nil
}

func (observer *nativeObserver) List(ctx context.Context, limit int) ([]model.Candidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, &Error{Kind: ErrorInternal, Operation: "list processes", Err: err}
	}
	if limit <= 0 {
		limit = 1000
	}
	pids, err := gopsprocess.PidsWithContext(ctx)
	if err != nil {
		return nil, classifyPortableError("list processes", err)
	}
	slices.Sort(pids)
	uid := uint32(os.Geteuid())
	result := make([]model.Candidate, 0, min(limit, len(pids)))
	for _, rawPID := range pids {
		if err := ctx.Err(); err != nil {
			return nil, &Error{Kind: ErrorInternal, Operation: "list processes", Err: err}
		}
		value, err := gopsprocess.NewProcessWithContext(ctx, rawPID)
		if err != nil {
			continue
		}
		uids, err := value.UidsWithContext(ctx)
		if err != nil || len(uids) == 0 || uids[0] != uid {
			continue
		}
		candidate, err := observer.Snapshot(ctx, int(rawPID))
		if err != nil {
			var processError *Error
			if errors.As(err, &processError) && (processError.Kind == ErrorNotFound || processError.Kind == ErrorInaccessible) {
				continue
			}
			return nil, err
		}
		result = append(result, candidate)
		if len(result) == limit {
			break
		}
	}
	return result, nil
}

func (observer *nativeObserver) Revalidate(ctx context.Context, expected model.Candidate) error {
	actual, err := observer.Snapshot(ctx, expected.Process.PID)
	if err != nil {
		return err
	}
	if !SameIdentity(expected, actual) {
		return &Error{Kind: ErrorIdentityChanged, Operation: "revalidate process", Err: errors.New("process identity changed")}
	}
	return nil
}

func linuxFileIdentity(info os.FileInfo) (string, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", errors.New("unexpected Linux stat type")
	}
	return fmt.Sprintf("%d:%d", stat.Dev, stat.Ino), nil
}

func classifyPortableError(operation string, err error) error {
	kind := ErrorInternal
	if os.IsNotExist(err) {
		kind = ErrorNotFound
	} else if os.IsPermission(err) {
		kind = ErrorInaccessible
	}
	return &Error{Kind: kind, Operation: operation, Err: err}
}
