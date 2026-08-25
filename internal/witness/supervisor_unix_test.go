//go:build darwin || linux

package witness

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if guarded, code := RunGuardianIfRequested(); guarded {
		os.Exit(code)
	}
	os.Exit(m.Run())
}

func TestUnixSupervisorPreservesNormalAndNonZeroExit(t *testing.T) {
	for _, code := range []int{0, 7} {
		var stdout, stderr bytes.Buffer
		supervisor, err := newSupervisor(testExecutable(t), helperArguments("exit", strconv.Itoa(code)), Streams{Stdout: &stdout, Stderr: &stderr})
		if err != nil {
			t.Fatal(err)
		}
		if err := supervisor.Start(); err != nil {
			t.Fatal(err)
		}
		waitErr := supervisor.Wait()
		if got := processExitCode(waitErr); got != code {
			t.Fatalf("exit code=%d want=%d err=%v", got, code, waitErr)
		}
		if stdout.String() != "stdout-bytes" || stderr.String() != "stderr-bytes" {
			t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
		}
	}
}

func TestUnixSupervisorCreatesOwnedProcessGroupAndForwardsTerm(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	supervisor, err := newSupervisor(testExecutable(t), helperArguments("term"), Streams{Stdout: writer, Stderr: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := supervisor.Start(); err != nil {
		t.Fatal(err)
	}
	line := readLine(t, reader)
	if line != "ready" {
		t.Fatalf("readiness = %q", line)
	}
	group, err := syscall.Getpgid(supervisor.PID())
	if err != nil || group != supervisor.PID() {
		t.Fatalf("pgid=%d pid=%d err=%v", group, supervisor.PID(), err)
	}
	if err := supervisor.GracefulStop(); err != nil {
		t.Fatal(err)
	}
	if line := readLine(t, reader); line != "terminated" {
		t.Fatalf("termination output = %q", line)
	}
	if err := supervisor.Wait(); err != nil {
		t.Fatalf("wait = %v", err)
	}
	_ = writer.Close()
}

func TestUnixSupervisorForceStopRemovesOwnedDescendants(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	supervisor, err := newSupervisor(testExecutable(t), helperArguments("tree"), Streams{Stdout: writer, Stderr: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := supervisor.Start(); err != nil {
		t.Fatal(err)
	}
	childPID, err := strconv.Atoi(readLine(t, reader))
	if err != nil || childPID <= 0 {
		t.Fatalf("child PID invalid: %d %v", childPID, err)
	}
	if err := supervisor.GracefulStop(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(25 * time.Millisecond)
	if err := supervisor.ForceStop(); err != nil {
		t.Fatal(err)
	}
	if processExitCode(supervisor.Wait()) == 0 {
		t.Fatal("forced process unexpectedly exited successfully")
	}
	_ = writer.Close()
	deadline := time.Now().Add(3 * time.Second)
	for processExists(childPID) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if processExists(childPID) {
		t.Fatalf("owned descendant %d survived force stop", childPID)
	}
}

func TestUnixSupervisorNormalExitRemovesOrphanedDescendants(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	supervisor, err := newSupervisor(testExecutable(t), helperArguments("orphan"), Streams{Stdout: writer, Stderr: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := supervisor.Start(); err != nil {
		t.Fatal(err)
	}
	childPID, err := strconv.Atoi(readLine(t, reader))
	if err != nil || childPID <= 0 {
		t.Fatalf("child PID invalid: %d %v", childPID, err)
	}
	if err := supervisor.Wait(); err != nil {
		t.Fatal(err)
	}
	_ = writer.Close()
	deadline := time.Now().Add(3 * time.Second)
	for processExists(childPID) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if processExists(childPID) {
		t.Fatalf("orphaned descendant %d survived normal parent exit", childPID)
	}
}

func TestUnixSupervisorOwnerDeathRemovesManagedProcess(t *testing.T) {
	owner := exec.Command(testExecutable(t), helperArguments("owner")...)
	stdout, err := owner.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	owner.Stderr = io.Discard
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	managedPID, err := strconv.Atoi(readLine(t, stdout))
	if err != nil || managedPID <= 0 {
		_ = owner.Process.Kill()
		_ = owner.Wait()
		t.Fatalf("managed PID invalid: %d %v", managedPID, err)
	}
	if err := owner.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = owner.Wait()
	deadline := time.Now().Add(3 * time.Second)
	for processExists(managedPID) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if processExists(managedPID) {
		_ = syscall.Kill(-managedPID, syscall.SIGKILL)
		t.Fatalf("managed process %d survived owner death", managedPID)
	}
}

func TestWitnessProcessHelper(t *testing.T) {
	separator := -1
	for index, value := range os.Args {
		if value == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || separator+1 >= len(os.Args) {
		return
	}
	switch os.Args[separator+1] {
	case "exit":
		_, _ = io.WriteString(os.Stdout, "stdout-bytes")
		_, _ = io.WriteString(os.Stderr, "stderr-bytes")
		code, _ := strconv.Atoi(os.Args[separator+2])
		os.Exit(code)
	case "term":
		channel := make(chan os.Signal, 1)
		signal.Notify(channel, syscall.SIGTERM)
		fmt.Fprintln(os.Stdout, "ready")
		<-channel
		fmt.Fprintln(os.Stdout, "terminated")
		os.Exit(0)
	case "tree", "orphan":
		signal.Ignore(syscall.SIGTERM)
		executable, err := os.Executable()
		if err != nil {
			os.Exit(89)
		}
		child := exec.Command(executable, helperArguments("sleep")...)
		if err := child.Start(); err != nil {
			os.Exit(90)
		}
		fmt.Fprintln(os.Stdout, child.Process.Pid)
		if os.Args[separator+1] == "orphan" {
			os.Exit(0)
		}
		_ = child.Wait()
		os.Exit(0)
	case "sleep":
		signal.Ignore(syscall.SIGTERM)
		for {
			time.Sleep(time.Hour)
		}
	case "owner":
		supervisor, err := newSupervisor(testExecutable(t), helperArguments("sleep"), Streams{Stdout: io.Discard, Stderr: io.Discard})
		if err != nil || supervisor.Start() != nil {
			os.Exit(91)
		}
		fmt.Fprintln(os.Stdout, supervisor.PID())
		for {
			time.Sleep(time.Hour)
		}
	}
}

func helperArguments(mode string, extra ...string) []string {
	return append([]string{"-test.run=TestWitnessProcessHelper", "--", mode}, extra...)
}

func testExecutable(t *testing.T) string {
	t.Helper()
	value, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func readLine(t *testing.T, reader io.Reader) string {
	t.Helper()
	line, err := bufio.NewReader(reader).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	return line[:len(line)-1]
}

func processExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || !errors.Is(err, syscall.ESRCH)
}
