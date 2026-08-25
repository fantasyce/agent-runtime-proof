//go:build windows

package hostprofile

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const configReadAccess = windows.FILE_READ_DATA | windows.FILE_READ_ATTRIBUTES | windows.SYNCHRONIZE

func readPinnedConfig(ctx context.Context, path string, maximum int64) ([]byte, error) {
	cleaned := filepath.Clean(path)
	volume := filepath.VolumeName(cleaned)
	if !filepath.IsAbs(cleaned) || len(volume) != 2 || volume[1] != ':' || maximum <= 0 {
		return nil, errors.New("invalid configuration path")
	}
	current, err := openConfigVolume(volume + `\`)
	if err != nil {
		return nil, err
	}
	defer func() {
		if current != nil {
			current.Close()
		}
	}()
	relative := strings.TrimLeft(strings.TrimPrefix(cleaned, volume), `\/`)
	for _, component := range strings.FieldsFunc(relative, func(value rune) bool { return value == '\\' || value == '/' }) {
		next, openErr := openConfigChild(current, component)
		current.Close()
		current = nil
		if openErr != nil {
			return nil, openErr
		}
		current = next
	}
	before, err := current.Stat()
	if err != nil || !before.Mode().IsRegular() {
		return nil, errors.New("configuration is not a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(current, maximum+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maximum {
		return nil, errors.New("configuration exceeds limit")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if _, err := current.Seek(0, io.SeekStart); err != nil {
		return nil, errConfigChanged
	}
	confirmed, err := io.ReadAll(io.LimitReader(current, maximum+1))
	if err != nil || !bytes.Equal(data, confirmed) {
		return nil, errConfigChanged
	}
	after, err := current.Stat()
	if err != nil || !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return nil, errConfigChanged
	}
	return data, nil
}

func openConfigVolume(path string) (*os.File, error) {
	pointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(pointer, configReadAccess, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	if err := rejectConfigReparse(handle); err != nil {
		windows.CloseHandle(handle)
		return nil, err
	}
	return os.NewFile(uintptr(handle), "host-config-root"), nil
}

func openConfigChild(parent *os.File, name string) (*os.File, error) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `\/:`) || strings.ContainsRune(name, 0) {
		return nil, errors.New("invalid configuration component")
	}
	objectName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return nil, err
	}
	attributes := &windows.OBJECT_ATTRIBUTES{RootDirectory: windows.Handle(parent.Fd()), ObjectName: objectName, Attributes: windows.OBJ_CASE_INSENSITIVE}
	attributes.Length = uint32(unsafe.Sizeof(*attributes))
	var status windows.IO_STATUS_BLOCK
	var handle windows.Handle
	err = windows.NtCreateFile(&handle, configReadAccess, attributes, &status, nil, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, windows.FILE_OPEN, windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
	if err != nil {
		if errors.Is(err, windows.STATUS_NO_SUCH_FILE) || errors.Is(err, windows.STATUS_OBJECT_NAME_NOT_FOUND) || errors.Is(err, windows.STATUS_OBJECT_PATH_NOT_FOUND) {
			return nil, os.ErrNotExist
		}
		return nil, err
	}
	if err := rejectConfigReparse(handle); err != nil {
		windows.CloseHandle(handle)
		return nil, err
	}
	return os.NewFile(uintptr(handle), "host-config"), nil
}

func rejectConfigReparse(handle windows.Handle) error {
	var information windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &information); err != nil {
		return err
	}
	if information.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return errors.New("reparse point rejected")
	}
	return nil
}
