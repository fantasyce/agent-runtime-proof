package app

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/fantasyce/agent-runtime-proof/internal/artifact"
	"github.com/fantasyce/agent-runtime-proof/internal/model"
)

func TestDoctorReportsImplementedReadOnlyCapabilities(t *testing.T) {
	service := testService(&fakeObserver{candidates: map[int]model.Candidate{os.Getpid(): candidate(os.Getpid())}})
	result := service.Doctor(context.Background(), DoctorRequest{})
	if result.Status != "ok" || result.Platform.OS == "" || result.Platform.Arch == "" {
		t.Fatalf("doctor = %#v", result)
	}
	if !hasCapability(result.Capabilities, "process-observation") || !hasCapability(result.Capabilities, "embedded-contracts") {
		t.Fatalf("capabilities = %v", result.Capabilities)
	}
	if !hasCapability(result.Capabilities, "host-profiles") || !hasCapability(result.Capabilities, "mcp") || !hasCapability(result.Capabilities, "witness") {
		t.Fatalf("doctor omitted implemented capability: %v", result.Capabilities)
	}
}

func TestDoctorAggregatesObserverFailureWithoutWriting(t *testing.T) {
	observer := &fakeObserver{snapshotErr: errors.New("observer unavailable")}
	service := testService(observer)
	result := service.Doctor(context.Background(), DoctorRequest{})
	if result.Status != "warning" {
		t.Fatalf("status = %q", result.Status)
	}
	if len(result.Checks) < 2 {
		t.Fatalf("checks = %#v", result.Checks)
	}
}

func TestDoctorArtifactCapabilityMatchesPlatformSupport(t *testing.T) {
	service := testService(&fakeObserver{candidates: map[int]model.Candidate{os.Getpid(): candidate(os.Getpid())}})
	result := service.Doctor(context.Background(), DoctorRequest{})
	if got, want := hasCapability(result.Capabilities, "read-only-artifact-digest"), artifact.ReadingSupported(); got != want {
		t.Fatalf("artifact capability reported = %t, platform support = %t; capabilities = %v", got, want, result.Capabilities)
	}
}

func TestDoctorChecksExplicitHostWithoutLeakingConfiguration(t *testing.T) {
	service := testService(&fakeObserver{candidates: map[int]model.Candidate{os.Getpid(): candidate(os.Getpid())}})
	service.HostProfiles = fakeHostProfiles{binding: testBinding(t)}
	result := service.Doctor(context.Background(), DoctorRequest{HostID: "cursor"})
	if result.Status != "ok" || !hasCheck(result.Checks, "host-profile", "ok") {
		t.Fatalf("doctor = %#v", result)
	}
	service.HostProfiles = fakeHostProfiles{err: errors.New("private /Users/example token-secret")}
	result = service.Doctor(context.Background(), DoctorRequest{HostID: "cursor"})
	if result.Status != "warning" || !hasCheck(result.Checks, "host-profile", "warning") {
		t.Fatalf("failed doctor = %#v", result)
	}
	for _, check := range result.Checks {
		if strings.Contains(check.Detail, "/Users/example") || strings.Contains(check.Detail, "token-secret") {
			t.Fatalf("doctor leaked error: %#v", result)
		}
	}
}

func hasCheck(values []DoctorCheck, name, status string) bool {
	for _, value := range values {
		if value.Name == name && value.Status == status {
			return true
		}
	}
	return false
}

func hasCapability(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
