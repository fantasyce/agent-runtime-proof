package witness

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

type exitStatusError struct{ code int }

func (value *exitStatusError) Error() string {
	return fmt.Sprintf("witness child exited with status %d", value.code)
}

func (value *exitStatusError) ExitCode() int { return value.code }

type Streams struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type supervisor interface {
	Start() error
	PID() int
	Wait() error
	GracefulStop() error
	ForceStop() error
}

func processExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ExitCode()
	}
	var coded interface{ ExitCode() int }
	if errors.As(err, &coded) {
		return coded.ExitCode()
	}
	return -1
}

func closeFiles(files ...*os.File) {
	for _, file := range files {
		if file != nil {
			_ = file.Close()
		}
	}
}
