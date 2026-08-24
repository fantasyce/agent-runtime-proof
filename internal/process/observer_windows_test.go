//go:build windows

package process

import (
	"encoding/hex"
	"fmt"
	"os"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestWindowsFiletimeUsesUnixNanoseconds(t *testing.T) {
	const unixEpochFiletime = uint64(116444736000000000)
	value := windows.Filetime{LowDateTime: uint32(unixEpochFiletime & 0xffffffff), HighDateTime: uint32(unixEpochFiletime >> 32)}
	if got := filetimeUnixNano(value); got != 0 {
		t.Fatalf("Unix epoch = %d ns", got)
	}
}

func TestWindowsObserverImplementsContract(t *testing.T) {
	var _ Observer = NewObserver()
}

func TestWindowsFileIdentityUses128BitFileID(t *testing.T) {
	path := t.TempDir() + `\runtime.exe`
	if err := os.WriteFile(path, []byte("runtime"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := windowsFileIdentity(path)
	if err != nil {
		t.Fatal(err)
	}
	pointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(pointer, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	var information struct {
		VolumeSerialNumber uint64
		FileID             [16]byte
	}
	if err := windows.GetFileInformationByHandleEx(handle, windows.FileIdInfo, (*byte)(unsafe.Pointer(&information)), uint32(unsafe.Sizeof(information))); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("%016x:%s", information.VolumeSerialNumber, hex.EncodeToString(information.FileID[:]))
	if got != want {
		t.Fatalf("file identity = %q, want 128-bit identity %q", got, want)
	}
}
