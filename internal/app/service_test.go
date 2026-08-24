package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fantasyce/agent-runtime-proof/internal/artifact"
	"github.com/fantasyce/agent-runtime-proof/internal/expectation"
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
	root := "/fixture/runtime"
	return model.Candidate{
		Platform:               model.Platform{OS: "darwin", Arch: "arm64"},
		Process:                model.ProcessIdentity{PID: pid, CreatedAtUnixNano: "100", BootIDHash: "sha256:" + strings.Repeat("c", 64)},
		Executable:             model.ExecutableObservation{Basename: "runtime", PathHash: "sha256:" + strings.Repeat("d", 64)},
		DeclaredExecutablePath: root,
		ExecutableFileIdentity: "dev:1",
	}
}

func resolvedExpectation() expectation.Resolved {
	root := "/fixture"
	resolved := expectation.Resolved{ArtifactRoot: "/fixture/runtime", AllowedRoots: []string{root}}
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
