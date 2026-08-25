//go:build windows

package process

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/fantasyce/agent-runtime-proof/internal/model"
	gopsprocess "github.com/shirou/gopsutil/v4/process"
	"golang.org/x/sys/windows"
)

var getTickCount64 = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetTickCount64")

type nativeObserver struct {
	bootHash   string
	currentSID string
}

func NewObserver() Observer {
	return &nativeObserver{bootHash: windowsBootHash(), currentSID: currentProcessSID()}
}

func (observer *nativeObserver) Snapshot(ctx context.Context, pid int) (model.Candidate, error) {
	if err := ctx.Err(); err != nil {
		return model.Candidate{}, &Error{Kind: ErrorInternal, Operation: "snapshot", Err: err}
	}
	if pid <= 0 {
		return model.Candidate{}, &Error{Kind: ErrorNotFound, Operation: "snapshot", Err: errors.New("PID must be positive")}
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return model.Candidate{}, classifyWindowsError("open process", err)
	}
	defer windows.CloseHandle(handle)

	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err != nil {
		return model.Candidate{}, classifyWindowsError("read process creation time", err)
	}
	candidate := model.Candidate{
		Platform: model.Platform{OS: runtime.GOOS, Arch: runtime.GOARCH},
		Process: model.ProcessIdentity{
			PID: pid, CreatedAtUnixNano: strconv.FormatInt(filetimeUnixNano(creation), 10), BootIDHash: observer.bootHash,
		},
		Inaccessible: []string{},
	}
	if value, err := gopsprocess.NewProcessWithContext(ctx, int32(pid)); err == nil {
		if parentPID, err := value.PpidWithContext(ctx); err == nil {
			candidate.ParentPID = int(parentPID)
		}
	}
	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
		candidate.Inaccessible = []string{"process.image"}
		return candidate, classifyWindowsError("read process image", err)
	}
	path := windows.UTF16ToString(buffer[:size])
	identity, err := windowsFileIdentity(path)
	if err != nil {
		candidate.Inaccessible = []string{"process.image.file_identity"}
		return candidate, classifyWindowsError("identify process image", err)
	}
	normalizedPath := strings.ToLower(filepath.Clean(path))
	candidate.ExecutablePath = path
	candidate.DeclaredExecutablePath = path
	candidate.ExecutableFileIdentity = identity
	candidate.Executable = model.ExecutableObservation{
		Basename:   filepath.Base(path),
		PathHash:   hashIdentifier("arp:path:v1", normalizedPath),
		FileIDHash: hashIdentifier("arp:file-id:v1", identity),
	}
	observeArguments(ctx, pid, &candidate)
	return candidate, nil
}

func (observer *nativeObserver) List(ctx context.Context, limit int) ([]model.Candidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, &Error{Kind: ErrorInternal, Operation: "list processes", Err: err}
	}
	if limit <= 0 {
		limit = 1000
	}
	pids := make([]uint32, 4096)
	var returned uint32
	if err := windows.EnumProcesses(pids, &returned); err != nil {
		return nil, classifyWindowsError("list processes", err)
	}
	pids = pids[:returned/uint32(unsafe.Sizeof(pids[0]))]
	slices.Sort(pids)
	result := make([]model.Candidate, 0, min(limit, len(pids)))
	for _, pid := range pids {
		if err := ctx.Err(); err != nil {
			return nil, &Error{Kind: ErrorInternal, Operation: "list processes", Err: err}
		}
		if pid == 0 || observer.currentSID == "" || processSID(pid) != observer.currentSID {
			continue
		}
		candidate, err := observer.Snapshot(ctx, int(pid))
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

func filetimeUnixNano(value windows.Filetime) int64 {
	return value.Nanoseconds()
}

func windowsBootHash() string {
	uptimeMS, _, _ := getTickCount64.Call()
	bootUnixSecond := (time.Now().UnixMilli() - int64(uptimeMS)) / 1000
	return hashIdentifier("arp:boot:v1", strconv.FormatInt(bootUnixSecond, 10))
}

func windowsFileIdentity(path string) (string, error) {
	pointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	handle, err := windows.CreateFile(pointer, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(handle)
	var extended struct {
		VolumeSerialNumber uint64
		FileID             [16]byte
	}
	if err := windows.GetFileInformationByHandleEx(handle, windows.FileIdInfo, (*byte)(unsafe.Pointer(&extended)), uint32(unsafe.Sizeof(extended))); err == nil {
		return fmt.Sprintf("%016x:%s", extended.VolumeSerialNumber, hex.EncodeToString(extended.FileID[:])), nil
	}
	var fallback windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &fallback); err != nil {
		return "", err
	}
	return fmt.Sprintf("%08x:%08x%08x", fallback.VolumeSerialNumber, fallback.FileIndexHigh, fallback.FileIndexLow), nil
}

func currentProcessSID() string {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return ""
	}
	return user.User.Sid.String()
}

func processSID(pid uint32) string {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(handle)
	var token windows.Token
	if err := windows.OpenProcessToken(handle, windows.TOKEN_QUERY, &token); err != nil {
		return ""
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return ""
	}
	return user.User.Sid.String()
}

func classifyWindowsError(operation string, err error) error {
	kind := ErrorInternal
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) || errors.Is(err, windows.ERROR_NOT_FOUND) {
		kind = ErrorNotFound
	} else if errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
		kind = ErrorInaccessible
	}
	return &Error{Kind: kind, Operation: operation, Err: err}
}
