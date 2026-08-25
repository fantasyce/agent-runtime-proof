//go:build darwin || linux

package main

import (
	"os"
	"os/signal"
	"syscall"
)

func ignoreTermination() { signal.Ignore(os.Interrupt, syscall.SIGTERM, syscall.SIGHUP) }

func terminationChannel() <-chan os.Signal {
	channel := make(chan os.Signal, 1)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	return channel
}
