//go:build windows

package witness

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWindowsSupervisorPreservesCommandArgumentsAndStdio(t *testing.T) {
	arguments := []string{"space value", `quote"value`, "中文"}
	var stdout, stderr bytes.Buffer
	supervisor, err := newSupervisor(windowsTestExecutable(t), windowsHelperArguments("arguments", arguments...), Streams{
		Stdin: bytes.NewBufferString("stdin-bytes"), Stdout: &stdout, Stderr: &stderr,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := supervisor.Start(); err != nil {
		t.Fatal(err)
	}
	if err := supervisor.Wait(); err != nil {
		t.Fatal(err)
	}
	var value struct {
		Arguments []string `json:"arguments"`
		Stdin     string   `json:"stdin"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &value); err != nil {
		t.Fatalf("stdout=%q: %v", stdout.String(), err)
	}
	if fmt.Sprint(value.Arguments) != fmt.Sprint(arguments) || value.Stdin != "stdin-bytes" || stderr.String() != "stderr-bytes" {
		t.Fatalf("result=%#v stderr=%q", value, stderr.String())
	}
}

func TestWindowsSupervisorJobStopsDescendantTree(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	supervisor, err := newSupervisor(windowsTestExecutable(t), windowsHelperArguments("tree"), Streams{Stdout: writer, Stderr: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := supervisor.Start(); err != nil {
		t.Fatal(err)
	}
	childPID, err := strconv.Atoi(windowsReadLine(t, reader))
	if err != nil || childPID <= 0 {
		t.Fatalf("child PID invalid: %d %v", childPID, err)
	}
	if err := supervisor.ForceStop(); err != nil {
		t.Fatal(err)
	}
	if processExitCode(supervisor.Wait()) == 0 {
		t.Fatal("terminated Job reported success")
	}
	_ = writer.Close()
	deadline := time.Now().Add(5 * time.Second)
	for windowsProcessExists(uint32(childPID)) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if windowsProcessExists(uint32(childPID)) {
		t.Fatalf("Job descendant %d survived", childPID)
	}
}

func TestWindowsSupervisorNormalExitClosesJobAndRemovesOrphan(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	supervisor, err := newSupervisor(windowsTestExecutable(t), windowsHelperArguments("orphan"), Streams{Stdout: writer, Stderr: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := supervisor.Start(); err != nil {
		t.Fatal(err)
	}
	childPID, err := strconv.Atoi(windowsReadLine(t, reader))
	if err != nil || childPID <= 0 {
		t.Fatalf("child PID invalid: %d %v", childPID, err)
	}
	if err := supervisor.Wait(); err != nil {
		t.Fatal(err)
	}
	_ = writer.Close()
	deadline := time.Now().Add(5 * time.Second)
	for windowsProcessExists(uint32(childPID)) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if windowsProcessExists(uint32(childPID)) {
		t.Fatalf("orphaned Job descendant %d survived", childPID)
	}
}

func TestWitnessWindowsProcessHelper(t *testing.T) {
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
	case "arguments":
		contents, _ := io.ReadAll(os.Stdin)
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"arguments": os.Args[separator+2:], "stdin": string(contents)})
		_, _ = io.WriteString(os.Stderr, "stderr-bytes")
		os.Exit(0)
	case "tree", "orphan":
		child := exec.Command(windowsTestExecutable(t), windowsHelperArguments("sleep")...)
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
		<-context.Background().Done()
	}
}

func windowsHelperArguments(mode string, extra ...string) []string {
	return append([]string{"-test.run=TestWitnessWindowsProcessHelper", "--", mode}, extra...)
}

func windowsTestExecutable(t *testing.T) string {
	t.Helper()
	value, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func windowsProcessExists(pid uint32) bool {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)
	result, err := windows.WaitForSingleObject(handle, 0)
	return err == nil && result == uint32(windows.WAIT_TIMEOUT)
}

func windowsReadLine(t *testing.T, reader io.Reader) string {
	t.Helper()
	line, err := bufio.NewReader(reader).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	return string(bytes.TrimSpace([]byte(line)))
}
