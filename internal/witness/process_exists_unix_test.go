//go:build darwin || linux

package witness

import (
	"errors"
	"syscall"
)

func portableProcessExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || !errors.Is(err, syscall.ESRCH)
}
