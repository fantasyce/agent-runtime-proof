//go:build linux

package witness

import (
	"errors"
	"syscall"

	"golang.org/x/sys/unix"
)

func configureGuardianReaping() error {
	return unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0)
}

func reapGuardianChildren() {
	for {
		var status syscall.WaitStatus
		_, err := syscall.Wait4(-1, &status, 0, nil)
		if errors.Is(err, syscall.ECHILD) {
			return
		}
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if err != nil {
			return
		}
	}
}
