package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/fantasyce/agent-runtime-proof/internal/artifact"
	"github.com/fantasyce/agent-runtime-proof/internal/expectation"
	"github.com/fantasyce/agent-runtime-proof/internal/hostprofile"
	"github.com/fantasyce/agent-runtime-proof/internal/model"
	processobserver "github.com/fantasyce/agent-runtime-proof/internal/process"
)

func TestInspectExplicitPIDAndBoundedInventory(t *testing.T) {
	observer := &fakeObserver{candidates: map[int]model.Candidate{42: candidate(42), 43: candidate(43)}}
	service := testService(observer)
	explicit, err := service.Inspect(context.Background(), InspectRequest{PID: 42})
	if err != nil {
		t.Fatal(err)
	}
	if len(explicit.Proofs) != 1 || explicit.Proofs[0].Observation.Process.PID != 42 || explicit.Proofs[0].Verdict != "UNKNOWN" {
		t.Fatalf("explicit result = %#v", explicit)
	}
	all, err := service.Inspect(context.Background(), InspectRequest{All: true, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Proofs) != 1 || observer.listLimit != 1 {
		t.Fatalf("inventory = %#v, limit = %d", all, observer.listLimit)
	}
}

func TestInspectRejectsInvalidSelectorsAndCancellation(t *testing.T) {
	service := testService(&fakeObserver{candidates: map[int]model.Candidate{}})
	for _, request := range []InspectRequest{{}, {PID: 42, All: true}, {All: true, Limit: -1}} {
		if _, err := service.Inspect(context.Background(), request); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("request %#v error = %v", request, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Inspect(ctx, InspectRequest{PID: 42}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error = %v", err)
	}
}

func TestInspectMissingPIDPreservesNotFoundReason(t *testing.T) {
	service := testService(&fakeObserver{snapshotErr: &processobserver.Error{Kind: processobserver.ErrorNotFound, Operation: "snapshot"}})
	result, err := service.Inspect(context.Background(), InspectRequest{PID: 999999})
	if err != nil {
		t.Fatal(err)
	}
	if !containsReason(result.Proofs[0].ReasonCodes, "PROCESS_NOT_FOUND") {
		t.Fatalf("reason codes = %v", result.Proofs[0].ReasonCodes)
	}
}

func TestVerifyProducesMatchedAndNegativeProofsAndRevalidates(t *testing.T) {
	observer := &fakeObserver{candidates: map[int]model.Candidate{42: candidate(42)}}
	service := testService(observer)
	service.LoadExpectation = func(string) (expectation.Resolved, error) { return resolvedExpectation(), nil }
	service.DigestArtifact = func(context.Context, expectation.Resolved, artifact.Clock) (model.ArtifactObservation, error) {
		return model.ArtifactObservation{SHA256: strings.Repeat("b", 64), FileCount: 1, ByteCount: 10, EntrypointFileIdentity: "dev:1"}, nil
	}
	matched, err := service.Verify(context.Background(), VerifyRequest{PID: 42, ExpectationPath: "expectation.json"})
	if err != nil {
		t.Fatal(err)
	}
	if matched.Proof.Verdict != "MATCHED" || observer.revalidated != 1 {
		t.Fatalf("matched = %#v, revalidated = %d", matched, observer.revalidated)
	}

	observer.revalidateErr = &processobserver.Error{Kind: processobserver.ErrorIdentityChanged, Operation: "revalidate"}
	changed, err := service.Verify(context.Background(), VerifyRequest{PID: 42, ExpectationPath: "expectation.json"})
	if err != nil {
		t.Fatal(err)
	}
	if changed.Proof.Verdict != "UNKNOWN" || changed.Proof.ReasonCodes[0] != "PROCESS_IDENTITY_CHANGED" {
		t.Fatalf("changed = %#v", changed)
	}
	if changed.Proof.Observation.Process != nil || changed.Proof.Observation.Executable != nil {
		t.Fatalf("identity-changed proof retained abandoned process evidence: %#v", changed.Proof.Observation)
	}
}

func TestVerifyReturnsNotRunningProofAndRejectsMissingExpectation(t *testing.T) {
	observer := &fakeObserver{snapshotErr: &processobserver.Error{Kind: processobserver.ErrorNotFound, Operation: "snapshot"}}
	service := testService(observer)
	service.LoadExpectation = func(string) (expectation.Resolved, error) { return resolvedExpectation(), nil }
	result, err := service.Verify(context.Background(), VerifyRequest{PID: 999999, ExpectationPath: "expectation.json"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Proof.Verdict != "NOT_RUNNING" || result.Proof.Observation.Process != nil {
		t.Fatalf("result = %#v", result)
	}
	if _, err := service.Verify(context.Background(), VerifyRequest{PID: 42}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("missing expectation error = %v", err)
	}
}

func TestInspectAndVerifyByBindingPreserveAttributionAndRevalidate(t *testing.T) {
	binding := testBinding(t)
	observer := &fakeObserver{candidates: map[int]model.Candidate{42: candidate(42)}}
	service := testService(observer)
	service.HostProfiles = fakeHostProfiles{binding: binding}
	inspected, err := service.Inspect(context.Background(), InspectRequest{BindingID: binding.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(inspected.Proofs) != 1 || inspected.Proofs[0].HostAttribution == nil || inspected.Proofs[0].HostAttribution.HostID != "cursor" || observer.revalidated != 1 {
		t.Fatalf("inspect = %#v, revalidated=%d", inspected, observer.revalidated)
	}
	service.LoadExpectation = func(string) (expectation.Resolved, error) { return resolvedExpectation(), nil }
	service.DigestArtifact = func(context.Context, expectation.Resolved, artifact.Clock) (model.ArtifactObservation, error) {
		return model.ArtifactObservation{SHA256: strings.Repeat("b", 64), FileCount: 1, ByteCount: 10, EntrypointFileIdentity: "dev:1"}, nil
	}
	verified, err := service.Verify(context.Background(), VerifyRequest{BindingID: binding.ID, ExpectationPath: "expectation.json"})
	if err != nil || verified.Proof.Verdict != "MATCHED" || verified.Proof.HostAttribution == nil {
		t.Fatalf("verify = %#v, %v", verified, err)
	}
}

func TestBindingFailureBecomesSafeDomainProofAndPIDFallbackStillWorks(t *testing.T) {
	observer := &fakeObserver{candidates: map[int]model.Candidate{42: candidate(42)}}
	service := testService(observer)
	service.HostProfiles = fakeHostProfiles{err: &hostprofile.Error{Code: "HOST_CONFIG_INVALID"}}
	result, err := service.Inspect(context.Background(), InspectRequest{BindingID: "cursor.arp"})
	if err != nil || result.Proofs[0].Verdict != "UNKNOWN" || !containsReason(result.Proofs[0].ReasonCodes, "HOST_CONFIG_INVALID") {
		t.Fatalf("binding failure = %#v, %v", result, err)
	}
	pIDResult, err := service.Inspect(context.Background(), InspectRequest{PID: 42})
	if err != nil || pIDResult.Proofs[0].Observation.Process.PID != 42 {
		t.Fatalf("PID fallback = %#v, %v", pIDResult, err)
	}
	encoded, _ := json.Marshal(result)
	if strings.Contains(string(encoded), "/Users/private") || strings.Contains(string(encoded), "token-secret") {
		t.Fatalf("unsafe result: %s", encoded)
	}
}

func TestBindingNotRunningRetainsSafeAttribution(t *testing.T) {
	binding := testBinding(t)
	service := testService(&fakeObserver{candidates: map[int]model.Candidate{}})
	service.HostProfiles = fakeHostProfiles{binding: binding}
	result, err := service.Inspect(context.Background(), InspectRequest{BindingID: binding.ID})
	if err != nil || result.Proofs[0].HostAttribution == nil || result.Proofs[0].HostAttribution.BindingID != binding.ID || !containsReason(result.Proofs[0].ReasonCodes, "PROCESS_NOT_FOUND") {
		t.Fatalf("not-running binding = %#v, %v", result, err)
	}
}

func TestInspectHostListsAttributedBindingsFromOneBoundedInventory(t *testing.T) {
	binding := testBinding(t)
	observer := &fakeObserver{candidates: map[int]model.Candidate{42: candidate(42)}}
	service := testService(observer)
	service.HostProfiles = fakeHostProfiles{binding: binding}
	result, err := service.Inspect(context.Background(), InspectRequest{All: true, HostID: "cursor", Limit: 7})
	if err != nil || len(result.Proofs) != 1 || result.Proofs[0].HostAttribution == nil || result.Proofs[0].HostAttribution.BindingID != binding.ID || observer.listLimit != 7 {
		t.Fatalf("host inventory = %#v, err=%v, limit=%d", result, err, observer.listLimit)
	}
}

type fakeHostProfiles struct {
	binding hostprofile.Binding
	err     error
}

func (fake fakeHostProfiles) Binding(context.Context, string) (hostprofile.Binding, error) {
	return fake.binding, fake.err
}
func (fake fakeHostProfiles) Bindings(context.Context, string) ([]hostprofile.Binding, error) {
	if fake.err != nil {
		return nil, fake.err
	}
	return []hostprofile.Binding{fake.binding}, nil
}

func testBinding(t *testing.T) hostprofile.Binding {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mcp.json")
	config, err := json.Marshal(map[string]any{"mcpServers": map[string]any{"arp": map[string]any{"command": testRuntimePath(), "args": []string{"mcp"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, config, 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := hostprofile.Discover(context.Background(), hostprofile.Request{HostID: "cursor", Platform: runtime.GOOS, ExplicitConfigPath: path})
	if err != nil || len(result.Bindings) != 1 {
		t.Fatalf("binding fixture = %#v, %v", result, err)
	}
	return result.Bindings[0]
}

type fakeObserver struct {
	candidates    map[int]model.Candidate
	snapshotErr   error
	revalidateErr error
	listLimit     int
	revalidated   int
}

func (fake *fakeObserver) Snapshot(ctx context.Context, pid int) (model.Candidate, error) {
	if err := ctx.Err(); err != nil {
		return model.Candidate{}, err
	}
	if fake.snapshotErr != nil {
		return model.Candidate{}, fake.snapshotErr
	}
	value, ok := fake.candidates[pid]
	if !ok {
		return model.Candidate{}, &processobserver.Error{Kind: processobserver.ErrorNotFound, Operation: "snapshot"}
	}
	return value, nil
}

func (fake *fakeObserver) List(ctx context.Context, limit int) ([]model.Candidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fake.listLimit = limit
	result := []model.Candidate{}
	for pid := 1; pid < 1000 && len(result) < limit; pid++ {
		if value, ok := fake.candidates[pid]; ok {
			result = append(result, value)
		}
	}
	return result, nil
}

func (fake *fakeObserver) Revalidate(ctx context.Context, _ model.Candidate) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	fake.revalidated++
	return fake.revalidateErr
}

func testService(observer processobserver.Observer) *Service {
	service := NewService(observer, model.ToolInfo{Name: "agent-runtime-proof", Version: "0.1.0", Commit: "abcdef0", Toolchain: "go1.27.0"})
	service.Clock = fixedClock{value: time.Date(2026, 8, 24, 12, 34, 56, 0, time.UTC)}
	return service
}

func candidate(pid int) model.Candidate {
	root := testRuntimePath()
	return model.Candidate{
		Platform:               model.Platform{OS: runtime.GOOS, Arch: runtime.GOARCH},
		Process:                model.ProcessIdentity{PID: pid, CreatedAtUnixNano: "100", BootIDHash: "sha256:" + strings.Repeat("c", 64)},
		Executable:             model.ExecutableObservation{Basename: "runtime", PathHash: "sha256:" + strings.Repeat("d", 64)},
		DeclaredExecutablePath: root,
		ExecutableFileIdentity: "dev:1",
		ArgumentFingerprints:   []model.ArgumentFingerprint{{Position: 1, SHA256: testArgumentHash("mcp")}},
	}
}

func testRuntimePath() string {
	if runtime.GOOS == "windows" {
		return `C:\fixture\runtime.exe`
	}
	return "/fixture/runtime"
}

func testArgumentHash(value string) string {
	digest := sha256.Sum256([]byte("arp:host-argument:v1\x00" + value))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func resolvedExpectation() expectation.Resolved {
	runtimePath := testRuntimePath()
	resolved := expectation.Resolved{ArtifactRoot: runtimePath, AllowedRoots: []string{filepath.Dir(runtimePath)}}
	resolved.Value.Subject = model.Subject{ID: "example", DisplayName: "Example", Version: "1.0.0"}
	resolved.Value.Source = model.ExpectationSource{Kind: "user-file", LocatorHash: strings.Repeat("a", 64), Trust: "declared"}
	resolved.Value.Artifact.SHA256 = strings.Repeat("b", 64)
	resolved.Value.Launch.Kind = "native"
	return resolved
}

type fixedClock struct{ value time.Time }

func (clock fixedClock) Now() time.Time { return clock.value }

func containsReason(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
