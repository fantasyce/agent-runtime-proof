//go:build darwin || linux

package witness

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

const guardianEnvironment = "AGENT_RUNTIME_PROOF_INTERNAL_GUARD=1"

type guardianConfig struct {
	Command []string `json:"command"`
}

// RunGuardianIfRequested runs the internal parent-death guardian before any
// CLI parsing. It returns true only inside a process created by the Witness.
func RunGuardianIfRequested() (bool, int) {
	if os.Getenv("AGENT_RUNTIME_PROOF_INTERNAL_GUARD") != "1" {
		return false, 0
	}
	return true, runGuardian()
}

func runGuardian() int {
	if err := configureGuardianReaping(); err != nil {
		return 70
	}
	owner := os.NewFile(3, "witness-owner")
	configuration := os.NewFile(4, "witness-configuration")
	status := os.NewFile(5, "witness-status")
	if owner == nil || configuration == nil || status == nil {
		return 70
	}
	defer owner.Close()
	defer configuration.Close()
	defer status.Close()
	syscall.CloseOnExec(int(owner.Fd()))
	syscall.CloseOnExec(int(configuration.Fd()))
	syscall.CloseOnExec(int(status.Fd()))
	document, err := io.ReadAll(io.LimitReader(configuration, 1<<20+1))
	if err != nil || len(document) > 1<<20 {
		return 70
	}
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.DisallowUnknownFields()
	var config guardianConfig
	if err := decoder.Decode(&config); err != nil || len(config.Command) == 0 || len(config.Command) > maximumCommandArguments {
		return 70
	}
	for _, value := range config.Command {
		if value == "" || strings.ContainsRune(value, 0) {
			return 70
		}
	}
	command := exec.Command(config.Command[0], config.Command[1:]...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Env = withoutGuardianEnvironment(os.Environ())
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		return 70
	}
	pid := command.Process.Pid
	if _, err := status.WriteString(strconv.Itoa(pid) + "\n"); err != nil {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		_ = command.Wait()
		return 70
	}
	_ = status.Close()
	targetDone := make(chan error, 1)
	go func() { targetDone <- command.Wait() }()
	ownerDone := make(chan struct{}, 1)
	go func() {
		var buffer [1]byte
		_, _ = owner.Read(buffer[:])
		ownerDone <- struct{}{}
	}()
	select {
	case waitErr := <-targetDone:
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		reapGuardianChildren()
		return guardianExitCode(waitErr)
	case <-ownerDone:
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		<-targetDone
		reapGuardianChildren()
		return 70
	}
}

func withoutGuardianEnvironment(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !strings.HasPrefix(value, "AGENT_RUNTIME_PROOF_INTERNAL_GUARD=") {
			result = append(result, value)
		}
	}
	return result
}

func guardianExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) {
		return 70
	}
	if status, ok := exitError.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return 128 + int(status.Signal())
	}
	code := exitError.ExitCode()
	if code < 0 || code > 255 {
		return 70
	}
	return code
}
