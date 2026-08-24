//go:build linux

package artifact

import (
	"fmt"
	"os"
	"syscall"
)

func fileChangeToken(info os.FileInfo) string {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%d:%d", stat.Ctim.Sec, stat.Ctim.Nsec)
}
