package witness

import (
	"bytes"
	"context"
	"io"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestRunProxiesBinaryStdoutAndChildStderrWithoutARPBytes(t *testing.T) {
	payload := []byte{0x00, 0xff, '\n', '{', '}', 0x00, 'x'}
	var stdout, stderr bytes.Buffer
	result, err := Run(context.Background(), testRunController(t), RunRequest{
		Command: append([]string{testExecutablePortable(t)}, portableHelperArguments("proxy", "0")...),
		Stdin:   bytes.NewReader(payload), Stdout: &stdout, Stderr: &stderr, GracePeriod: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 || result.ReceiptID == "" || result.PID <= 0 {
		t.Fatalf("result = %#v", result)
	}
	if !bytes.Equal(stdout.Bytes(), payload) || stderr.String() != "child-stderr" {
		t.Fatalf("stdout=%v stderr=%q", stdout.Bytes(), stderr.String())
	}
}

func TestRunPreservesChildNonZeroExit(t *testing.T) {
	result, err := Run(context.Background(), testRunController(t), RunRequest{
		Command: append([]string{testExecutablePortable(t)}, portableHelperArguments("proxy", "7")...),
		Stdin:   bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard, GracePeriod: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 7 {
		t.Fatalf("exit code = %d", result.ExitCode)
	}
}

func TestRunEscalatesAfterStdinEOFAndLeavesNoProcess(t *testing.T) {
	started := time.Now()
	result, err := Run(context.Background(), testRunController(t), RunRequest{
		Command: append([]string{testExecutablePortable(t)}, portableHelperArguments("hang-eof")...),
		Stdin:   bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard, GracePeriod: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode == 0 || time.Since(started) > 2*time.Second {
		t.Fatalf("result=%#v duration=%s", result, time.Since(started))
	}
	assertProcessGone(t, result.PID)
}

func TestRunCancellationEscalatesAndLeavesNoProcess(t *testing.T) {
	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdinReader.Close()
	defer stdinWriter.Close()
	stdoutReader, stdoutWriter := io.Pipe()
	defer stdoutReader.Close()
	ctx, cancel := context.WithCancel(context.Background())
	controller := testRunController(t)
	command := append([]string{testExecutablePortable(t)}, portableHelperArguments("ready-hang")...)
	resultChannel := make(chan Result, 1)
	errorChannel := make(chan error, 1)
	go func() {
		result, runErr := Run(ctx, controller, RunRequest{
			Command: command,
			Stdin:   stdinReader, Stdout: stdoutWriter, Stderr: io.Discard, GracePeriod: 50 * time.Millisecond,
		})
		resultChannel <- result
		errorChannel <- runErr
	}()
	if line := readPortableLine(t, stdoutReader); line != "ready" {
		t.Fatalf("readiness = %q", line)
	}
	cancel()
	result := <-resultChannel
	if runErr := <-errorChannel; runErr != nil {
		t.Fatal(runErr)
	}
	if result.ExitCode == 0 {
		t.Fatalf("result = %#v", result)
	}
	_ = stdoutWriter.Close()
	assertProcessGone(t, result.PID)
}

func testRunController(t *testing.T) *Controller {
	t.Helper()
	return NewController(Dependencies{Tool: testTool(), Home: t.TempDir()})
}

func testExecutablePortable(t *testing.T) string {
	t.Helper()
	value, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func readPortableLine(t *testing.T, reader io.Reader) string {
	t.Helper()
	var value []byte
	buffer := make([]byte, 1)
	for {
		count, err := reader.Read(buffer)
		if count == 1 {
			if buffer[0] == '\n' {
				return string(value)
			}
			value = append(value, buffer[0])
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}

func assertProcessGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for portableProcessExists(pid) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if portableProcessExists(pid) {
		t.Fatalf("process %s survived", strconv.Itoa(pid))
	}
}
