package app

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/fantasyce/agent-runtime-proof/internal/artifact"
	"github.com/fantasyce/agent-runtime-proof/internal/model"
)

func TestDoctorReportsReadOnlyPhase1Capabilities(t *testing.T) {
	service := testService(&fakeObserver{candidates: map[int]model.Candidate{os.Getpid(): candidate(os.Getpid())}})
	result := service.Doctor(context.Background())
	if result.Status != "ok" || result.Platform.OS == "" || result.Platform.Arch == "" {
		t.Fatalf("doctor = %#v", result)
	}
	if !hasCapability(result.Capabilities, "process-observation") || !hasCapability(result.Capabilities, "embedded-contracts") {
		t.Fatalf("capabilities = %v", result.Capabilities)
	}
	if hasCapability(result.Capabilities, "host-profiles") || hasCapability(result.Capabilities, "mcp") || hasCapability(result.Capabilities, "witness") {
		t.Fatalf("doctor claimed deferred capability: %v", result.Capabilities)
	}
	if len(result.Limitations) == 0 {
		t.Fatal("doctor omitted Phase 1A limitations")
	}
}

func TestDoctorAggregatesObserverFailureWithoutWriting(t *testing.T) {
	observer := &fakeObserver{snapshotErr: errors.New("observer unavailable")}
	service := testService(observer)
	result := service.Doctor(context.Background())
	if result.Status != "warning" {
		t.Fatalf("status = %q", result.Status)
	}
	if len(result.Checks) < 2 {
		t.Fatalf("checks = %#v", result.Checks)
	}
}

func TestDoctorArtifactCapabilityMatchesPlatformSupport(t *testing.T) {
	service := testService(&fakeObserver{candidates: map[int]model.Candidate{os.Getpid(): candidate(os.Getpid())}})
	result := service.Doctor(context.Background())
	if got, want := hasCapability(result.Capabilities, "read-only-artifact-digest"), artifact.ReadingSupported(); got != want {
		t.Fatalf("artifact capability reported = %t, platform support = %t; capabilities = %v", got, want, result.Capabilities)
	}
}

func hasCapability(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
