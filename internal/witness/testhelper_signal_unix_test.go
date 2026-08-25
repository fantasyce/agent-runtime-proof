//go:build darwin || linux

package witness

import (
	"os/signal"
	"syscall"
)

func ignoreHelperTermination() { signal.Ignore(syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP) }
