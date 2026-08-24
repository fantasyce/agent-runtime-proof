//go:build windows

package process

import (
	"testing"

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
