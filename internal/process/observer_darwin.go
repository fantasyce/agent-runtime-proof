//go:build darwin && cgo

package process

/*
#include <errno.h>
#include <libproc.h>
#include <stdlib.h>
#include <sys/proc_info.h>
#include <unistd.h>

static int arp_pid_bsdinfo(int pid, struct proc_bsdinfo *info, int *error_number) {
	int result = proc_pidinfo(pid, PROC_PIDTBSDINFO, 0, info, sizeof(*info));
	if (result <= 0) *error_number = errno;
	return result;
}

static int arp_pid_path(int pid, void *buffer, uint32_t size, int *error_number) {
	int result = proc_pidpath(pid, buffer, size);
	if (result <= 0) *error_number = errno;
	return result;
}
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"syscall"
	"unsafe"

	"github.com/fantasyce/agent-runtime-proof/internal/model"
	"golang.org/x/sys/unix"
)

type nativeObserver struct {
	bootHash string
}

func NewObserver() Observer {
	return &nativeObserver{bootHash: darwinBootHash()}
}

func (observer *nativeObserver) Snapshot(ctx context.Context, pid int) (model.Candidate, error) {
	if err := ctx.Err(); err != nil {
		return model.Candidate{}, &Error{Kind: ErrorInternal, Operation: "snapshot", Err: err}
	}
	if pid <= 0 {
		return model.Candidate{}, &Error{Kind: ErrorNotFound, Operation: "snapshot", Err: errors.New("PID must be positive")}
	}
	info, errno, ok := darwinBSDInfo(pid)
	if !ok {
		return model.Candidate{}, classifyDarwinError("read process identity", errno)
	}
	created := uint64(info.pbi_start_tvsec)*1_000_000_000 + uint64(info.pbi_start_tvusec)*1_000
	candidate := model.Candidate{
		Platform: model.Platform{OS: runtime.GOOS, Arch: runtime.GOARCH},
		Process: model.ProcessIdentity{
			PID: pid, CreatedAtUnixNano: strconv.FormatUint(created, 10), BootIDHash: observer.bootHash,
		},
		ParentPID:    int(info.pbi_ppid),
		Inaccessible: []string{},
	}
	path, errno, ok := darwinPIDPath(pid)
	if !ok {
		candidate.Inaccessible = []string{"process.image"}
		return candidate, classifyDarwinError("read process image", errno)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		candidate.Inaccessible = []string{"process.image.file_identity"}
		return candidate, &Error{Kind: ErrorInaccessible, Operation: "stat process image", Err: err}
	}
	identity, err := darwinFileIdentity(fileInfo)
	if err != nil {
		return candidate, &Error{Kind: ErrorInternal, Operation: "identify process image", Err: err}
	}
	candidate.ExecutablePath = path
	candidate.DeclaredExecutablePath = path
	candidate.Executable = model.ExecutableObservation{
		Basename:   filepath.Base(path),
		PathHash:   hashIdentifier("arp:path:v1", path),
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
	const capacity = 65536
	pids := make([]C.int, capacity)
	count := int(C.proc_listallpids(unsafe.Pointer(&pids[0]), C.int(len(pids)*int(C.sizeof_int))))
	if count < 0 {
		return nil, &Error{Kind: ErrorInternal, Operation: "list processes", Err: errors.New("proc_listallpids failed")}
	}
	effectiveUID := uint32(C.geteuid())
	result := make([]model.Candidate, 0, min(limit, count))
	for _, rawPID := range pids[:min(count, len(pids))] {
		if err := ctx.Err(); err != nil {
			return nil, &Error{Kind: ErrorInternal, Operation: "list processes", Err: err}
		}
		if rawPID <= 0 {
			continue
		}
		info, _, ok := darwinBSDInfo(int(rawPID))
		if !ok || uint32(info.pbi_uid) != effectiveUID {
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
	slices.SortFunc(result, func(left, right model.Candidate) int { return left.Process.PID - right.Process.PID })
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

func darwinBSDInfo(pid int) (C.struct_proc_bsdinfo, int, bool) {
	var info C.struct_proc_bsdinfo
	var errorNumber C.int
	result := C.arp_pid_bsdinfo(C.int(pid), &info, &errorNumber)
	return info, int(errorNumber), result == C.int(C.sizeof_struct_proc_bsdinfo)
}

func darwinPIDPath(pid int) (string, int, bool) {
	buffer := make([]byte, C.PROC_PIDPATHINFO_MAXSIZE)
	var errorNumber C.int
	result := C.arp_pid_path(C.int(pid), unsafe.Pointer(&buffer[0]), C.uint32_t(len(buffer)), &errorNumber)
	if result <= 0 {
		return "", int(errorNumber), false
	}
	return C.GoString((*C.char)(unsafe.Pointer(&buffer[0]))), 0, true
}

func darwinBootHash() string {
	value, err := unix.SysctlTimeval("kern.boottime")
	if err != nil {
		return hashIdentifier("arp:boot:v1", "unavailable")
	}
	return hashIdentifier("arp:boot:v1", fmt.Sprintf("%d:%d", value.Sec, value.Usec))
}

func darwinFileIdentity(info os.FileInfo) (string, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", errors.New("unexpected Darwin stat type")
	}
	return fmt.Sprintf("%d:%d", stat.Dev, stat.Ino), nil
}

func classifyDarwinError(operation string, errorNumber int) error {
	kind := ErrorInternal
	if errorNumber == int(syscall.ESRCH) || errorNumber == int(syscall.ENOENT) {
		kind = ErrorNotFound
	} else if errorNumber == int(syscall.EPERM) || errorNumber == int(syscall.EACCES) {
		kind = ErrorInaccessible
	}
	return &Error{Kind: kind, Operation: operation, Err: syscall.Errno(errorNumber)}
}
