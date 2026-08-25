//go:build linux

package main

import (
	"os"
	"syscall"
)

func peakRSS(state *os.ProcessState) int64 {
	usage, ok := state.SysUsage().(*syscall.Rusage)
	if !ok {
		return 0
	}
	return usage.Maxrss * 1024
}
