//go:build windows

package main

import (
	"os"
	"os/signal"
)

func ignoreTermination() { signal.Ignore(os.Interrupt) }

func terminationChannel() <-chan os.Signal {
	channel := make(chan os.Signal, 1)
	signal.Notify(channel, os.Interrupt)
	return channel
}
