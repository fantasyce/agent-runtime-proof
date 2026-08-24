package proof

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fantasyce/agent-runtime-proof/internal/contract"
	"github.com/fantasyce/agent-runtime-proof/internal/evaluator"
	"github.com/fantasyce/agent-runtime-proof/internal/expectation"
	"github.com/fantasyce/agent-runtime-proof/internal/model"
)

func TestBuildProducesSchemaValidSelfVerifyingProof(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "fixture", "runtime")
	resolved := &expectation.Resolved{ArtifactRoot: root, AllowedRoots: []string{root}}
	resolved.Value.Subject = model.Subject{ID: "example", DisplayName: "Example", Version: "1.0.0"}
	resolved.Value.Source = model.ExpectationSource{Kind: "user-file", LocatorHash: strings.Repeat("a", 64), Trust: "declared"}
	resolved.Value.Artifact.SHA256 = strings.Repeat("b", 64)
	candidate := &model.Candidate{
		Platform:               model.Platform{OS: "darwin", Arch: "arm64"},
		Process:                model.ProcessIdentity{PID: 42, CreatedAtUnixNano: "100", BootIDHash: "sha256:" + strings.Repeat("c", 64)},
		Executable:             model.ExecutableObservation{Basename: "runtime", PathHash: "sha256:" + strings.Repeat("d", 64), FileIDHash: "sha256:" + strings.Repeat("e", 64)},
		DeclaredExecutablePath: filepath.Join(root, "runtime"),
	}
	artifact := &model.ArtifactObservation{SHA256: strings.Repeat("b", 64), FileCount: 1, ByteCount: 10, DurationMS: 1}
	value, err := Build(Input{
		Candidate: candidate, Expectation: resolved, Artifact: artifact,
		Decision:   evaluator.Decision{Verdict: "MATCHED", ProofLevel: "ARTIFACT_OBSERVED", ReasonCodes: []string{"MATCH_CONFIRMED"}, Limitations: []string{}},
		Tool:       model.ToolInfo{Name: "agent-runtime-proof", Version: "0.1.0", Commit: "abcdef0", Toolchain: "go1.27.0"},
		ObservedAt: time.Date(2026, 8, 24, 12, 34, 56, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.ValidateProof(encoded); err != nil {
		t.Fatalf("proof is invalid: %v\n%s", err, encoded)
	}
	if err := VerifyID(value); err != nil {
		t.Fatal(err)
	}
	roundTripped, err := Validate(encoded)
	if err != nil {
		t.Fatalf("round-trip validation failed: %v", err)
	}
	if roundTripped.ProofID != value.ProofID {
		t.Fatalf("round-trip proof ID = %s, want %s", roundTripped.ProofID, value.ProofID)
	}
	roundTripped.Subject.DisplayName = "Tampered"
	tampered, err := json.Marshal(roundTripped)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Validate(tampered); err == nil {
		t.Fatal("protected-field mutation passed public proof validation")
	}
}

func TestBuildSupportsHonestObservationOnlyAndNotRunningProofs(t *testing.T) {
	tool := model.ToolInfo{Name: "agent-runtime-proof", Version: "0.1.0", Commit: "abcdef0", Toolchain: "go1.27.0"}
	now := time.Date(2026, 8, 24, 12, 34, 56, 0, time.UTC)
	candidate := &model.Candidate{
		Platform:   model.Platform{OS: "darwin", Arch: "arm64"},
		Process:    model.ProcessIdentity{PID: 42, CreatedAtUnixNano: "100", BootIDHash: "sha256:" + strings.Repeat("c", 64)},
		Executable: model.ExecutableObservation{Basename: "runtime", PathHash: "sha256:" + strings.Repeat("d", 64)},
	}
	observationOnly, err := Build(Input{Candidate: candidate, Decision: evaluator.Evaluate(evaluator.Input{Candidate: candidate}), Tool: tool, ObservedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if observationOnly.Expectation != nil || observationOnly.Verdict != "UNKNOWN" {
		t.Fatalf("observation-only proof = %#v", observationOnly)
	}

	root := filepath.Join(string(filepath.Separator), "fixture", "runtime")
	resolved := &expectation.Resolved{ArtifactRoot: root, AllowedRoots: []string{root}}
	resolved.Value.Subject = model.Subject{ID: "example", DisplayName: "Example", Version: "1.0.0"}
	resolved.Value.Source = model.ExpectationSource{Kind: "user-file", LocatorHash: strings.Repeat("a", 64), Trust: "declared"}
	resolved.Value.Artifact.SHA256 = strings.Repeat("b", 64)
	notRunning, err := Build(Input{Expectation: resolved, Decision: evaluator.Decision{Verdict: "NOT_RUNNING", ProofLevel: "CONFIG_BOUND", ReasonCodes: []string{"PROCESS_NOT_FOUND"}, Limitations: []string{}}, Tool: tool, ObservedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if notRunning.Observation.Process != nil || notRunning.Observation.Executable != nil || notRunning.Verdict != "NOT_RUNNING" {
		t.Fatalf("not-running proof = %#v", notRunning)
	}
}
