//go:build darwin

package process

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestDarwinObserverSnapshotsCurrentProcess(t *testing.T) {
	observer := NewObserver()
	candidate, err := observer.Snapshot(context.Background(), os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Process.PID != os.Getpid() || candidate.Process.CreatedAtUnixNano == "" || candidate.Process.BootIDHash == "" {
		t.Fatalf("incomplete process identity: %#v", candidate.Process)
	}
	if candidate.Executable.Basename == "" || candidate.Executable.PathHash == "" || candidate.Executable.FileIDHash == "" {
		t.Fatalf("incomplete executable observation: %#v", candidate.Executable)
	}
	if !filepath.IsAbs(candidate.ExecutablePath) {
		t.Fatalf("internal executable path is not absolute: %q", candidate.ExecutablePath)
	}
}

func TestDarwinObserverClassifiesPermissionDenial(t *testing.T) {
	var processError *Error
	if !errors.As(classifyDarwinError("probe", int(syscall.EPERM)), &processError) || processError.Kind != ErrorInaccessible {
		t.Fatalf("permission error = %#v", processError)
	}
}

func TestDarwinObserverRevalidatesControlledHelper(t *testing.T) {
	command := exec.Command("/usr/bin/yes", "fake-token-secret")
	command.Env = append(os.Environ(), "ARP_FAKE_SECRET=must-not-appear")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	})

	observer := NewObserver()
	candidate, err := observer.Snapshot(context.Background(), command.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Executable.Basename != "yes" {
		t.Fatalf("basename = %q", candidate.Executable.Basename)
	}
	encoded := candidate.Executable.Basename + candidate.Executable.PathHash + strings.Join(candidate.Inaccessible, ",")
	if strings.Contains(encoded, "fake-token-secret") || strings.Contains(encoded, "must-not-appear") {
		t.Fatalf("candidate leaked command data: %s", encoded)
	}
	if err := observer.Revalidate(context.Background(), candidate); err != nil {
		t.Fatal(err)
	}

	mutated := candidate
	mutated.Process.CreatedAtUnixNano = "1"
	if err := observer.Revalidate(context.Background(), mutated); err == nil {
		t.Fatal("mutated process identity accepted")
	}
}

func TestDarwinObserverListsCurrentUserAndHonorsCancellation(t *testing.T) {
	observer := NewObserver()
	candidates, err := observer.List(context.Background(), 4096)
	if err != nil {
		t.Fatal(err)
	}
	foundSelf := false
	for index, candidate := range candidates {
		if index > 0 && candidates[index-1].Process.PID >= candidate.Process.PID {
			t.Fatalf("inventory is not strictly PID-sorted at %d", index)
		}
		if candidate.Process.PID == os.Getpid() {
			foundSelf = true
		}
	}
	if !foundSelf {
		t.Fatal("current process missing from current-user inventory")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	if _, err := observer.List(ctx, 4096); err == nil {
		t.Fatal("cancelled inventory succeeded")
	}
	if time.Since(started) > time.Second {
		t.Fatal("cancelled inventory did not stop promptly")
	}
}
