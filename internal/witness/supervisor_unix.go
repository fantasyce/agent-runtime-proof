//go:build darwin || linux

package witness

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

type unixSupervisor struct {
	command  string
	args     []string
	streams  Streams
	mu       sync.Mutex
	guardian *exec.Cmd
	owner    *os.File
	pid      int
	waited   bool
	complete bool
}

func newSupervisor(command string, arguments []string, streams Streams) (supervisor, error) {
	if command == "" {
		return nil, errors.New("witness supervisor command is empty")
	}
	if streams.Stdout == nil {
		streams.Stdout = io.Discard
	}
	if streams.Stderr == nil {
		streams.Stderr = io.Discard
	}
	return &unixSupervisor{command: command, args: append([]string{}, arguments...), streams: streams}, nil
}

func (value *unixSupervisor) Start() error {
	value.mu.Lock()
	defer value.mu.Unlock()
	if value.guardian != nil {
		return errors.New("witness process already started")
	}
	ownerReader, ownerWriter, err := os.Pipe()
	if err != nil {
		return err
	}
	configReader, configWriter, err := os.Pipe()
	if err != nil {
		closeFiles(ownerReader, ownerWriter)
		return err
	}
	statusReader, statusWriter, err := os.Pipe()
	if err != nil {
		closeFiles(ownerReader, ownerWriter, configReader, configWriter)
		return err
	}
	guardianExecutable, err := os.Executable()
	if err != nil {
		closeFiles(ownerReader, ownerWriter, configReader, configWriter, statusReader, statusWriter)
		return err
	}
	guardian := exec.Command(guardianExecutable)
	guardian.Env = append(os.Environ(), guardianEnvironment)
	guardian.ExtraFiles = []*os.File{ownerReader, configReader, statusWriter}
	guardian.Stdin = value.streams.Stdin
	guardian.Stdout = value.streams.Stdout
	guardian.Stderr = value.streams.Stderr
	guardian.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := guardian.Start(); err != nil {
		closeFiles(ownerReader, ownerWriter, configReader, configWriter, statusReader, statusWriter)
		return err
	}
	closeFiles(ownerReader, configReader, statusWriter)
	document, encodeErr := json.Marshal(guardianConfig{Command: append([]string{value.command}, value.args...)})
	if encodeErr == nil {
		_, encodeErr = configWriter.Write(document)
	}
	closeErr := configWriter.Close()
	if encodeErr == nil {
		encodeErr = closeErr
	}
	if encodeErr != nil {
		_ = guardian.Process.Kill()
		_ = guardian.Wait()
		closeFiles(ownerWriter, statusReader)
		return errors.New("witness guardian configuration failed")
	}
	line, readErr := bufio.NewReader(io.LimitReader(statusReader, 64)).ReadString('\n')
	_ = statusReader.Close()
	pid, parseErr := strconv.Atoi(strings.TrimSpace(line))
	if readErr != nil || parseErr != nil || pid <= 0 {
		_ = guardian.Process.Kill()
		_ = guardian.Wait()
		_ = ownerWriter.Close()
		return errors.New("witness guardian failed to start target")
	}
	value.guardian = guardian
	value.owner = ownerWriter
	value.pid = pid
	return nil
}

func (value *unixSupervisor) PID() int {
	value.mu.Lock()
	defer value.mu.Unlock()
	return value.pid
}

func (value *unixSupervisor) Wait() error {
	value.mu.Lock()
	if value.guardian == nil || value.pid <= 0 {
		value.mu.Unlock()
		return errors.New("witness process was not started")
	}
	if value.waited {
		value.mu.Unlock()
		return errors.New("witness process was already waited")
	}
	value.waited = true
	guardian := value.guardian
	owner := value.owner
	value.mu.Unlock()
	waitErr := guardian.Wait()
	if owner != nil {
		_ = owner.Close()
	}
	value.mu.Lock()
	value.complete = true
	value.owner = nil
	value.mu.Unlock()
	return waitErr
}

func (value *unixSupervisor) GracefulStop() error { return value.signalGroup(syscall.SIGTERM) }

func (value *unixSupervisor) ForceStop() error { return value.signalGroup(syscall.SIGKILL) }

func (value *unixSupervisor) ForwardSignal(signal os.Signal) error {
	unixSignal, ok := signal.(syscall.Signal)
	if !ok {
		return errors.New("unsupported witness signal")
	}
	return value.signalGroup(unixSignal)
}

func (value *unixSupervisor) signalGroup(signal syscall.Signal) error {
	value.mu.Lock()
	if value.complete {
		value.mu.Unlock()
		return nil
	}
	pid := value.pid
	value.mu.Unlock()
	if pid <= 0 {
		return errors.New("witness process was not started")
	}
	if err := syscall.Kill(-pid, signal); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}
