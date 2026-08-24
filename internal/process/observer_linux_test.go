//go:build linux

package process

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func TestLinuxObserverSnapshotsCurrentProcess(t *testing.T) {
	observer := NewObserver()
	candidate, err := observer.Snapshot(context.Background(), os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Platform.OS != "linux" || candidate.Process.PID != os.Getpid() {
		t.Fatalf("candidate = %#v", candidate)
	}
	if candidate.Process.CreatedAtUnixNano == "" || candidate.Process.BootIDHash == "" || candidate.Executable.FileIDHash == "" {
		t.Fatalf("incomplete candidate = %#v", candidate)
	}
}

func TestLinuxObserverClassifiesPermissionDenial(t *testing.T) {
	var processError *Error
	if !errors.As(classifyPortableError("probe", os.ErrPermission), &processError) || processError.Kind != ErrorInaccessible {
		t.Fatalf("permission error = %#v", processError)
	}
}

func TestLinuxObserverKeepsDeletedExecutableHandleObservable(t *testing.T) {
	directory := t.TempDir()
	helper := filepath.Join(directory, "sleep-copy")
	copyFile(t, "/bin/sleep", helper)
	command := exec.Command(helper, "30")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	})
	if err := os.Remove(helper); err != nil {
		t.Fatal(err)
	}

	candidate, err := NewObserver().Snapshot(context.Background(), command.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if !candidate.ExecutableDeleted {
		t.Fatal("deleted executable was not recorded")
	}
	if candidate.Executable.Basename != "sleep-copy" || candidate.ExecutablePath != filepath.Join("/proc", strconv.Itoa(candidate.Process.PID), "exe") {
		t.Fatalf("deleted executable candidate = %#v", candidate)
	}
}

func TestLinuxObserverListsCurrentUserAndHonorsCancellation(t *testing.T) {
	observer := NewObserver()
	candidates, err := observer.List(context.Background(), 4096)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for index, candidate := range candidates {
		if index > 0 && candidates[index-1].Process.PID >= candidate.Process.PID {
			t.Fatalf("inventory is not strictly PID-sorted at %d", index)
		}
		if candidate.Process.PID == os.Getpid() {
			found = true
		}
	}
	if !found {
		t.Fatal("current process missing from inventory")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := observer.List(ctx, 4096); err == nil {
		t.Fatal("cancelled inventory succeeded")
	}
}

func copyFile(t *testing.T, source, destination string) {
	t.Helper()
	input, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o700)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
}
