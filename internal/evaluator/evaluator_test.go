package evaluator

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/fantasyce/agent-runtime-proof/internal/artifact"
	"github.com/fantasyce/agent-runtime-proof/internal/expectation"
	"github.com/fantasyce/agent-runtime-proof/internal/model"
	processobserver "github.com/fantasyce/agent-runtime-proof/internal/process"
)

func TestEvaluateVerdictMatrix(t *testing.T) {
	allowed := filepath.Join(string(filepath.Separator), "fixture", "managed")
	resolved := &expectation.Resolved{ArtifactRoot: allowed, AllowedRoots: []string{allowed}}
	resolved.Value.Artifact.SHA256 = repeated('a')
	resolved.Value.Source.Trust = "verified"
	resolved.Value.Launch.Kind = "native"
	resolved.Value.Launch.Entrypoint = filepath.Join("bin", "runtime")
	candidate := &model.Candidate{DeclaredExecutablePath: filepath.Join(allowed, "bin", "runtime"), ExecutableFileIdentity: "dev:1"}
	observedMatch := &model.ArtifactObservation{SHA256: repeated('a'), EntrypointFileIdentity: "dev:1"}
	observedOld := &model.ArtifactObservation{SHA256: repeated('b'), EntrypointFileIdentity: "dev:1"}

	tests := []struct {
		name       string
		input      Input
		verdict    string
		reason     string
		limitation string
	}{
		{"matched", Input{Candidate: candidate, Expectation: resolved, Artifact: observedMatch}, "MATCHED", "MATCH_CONFIRMED", ""},
		{"outside root", Input{Candidate: &model.Candidate{DeclaredExecutablePath: filepath.Join(string(filepath.Separator), "dev", "runtime"), ExecutableFileIdentity: "dev:1"}, Expectation: resolved, Artifact: observedMatch}, "LEAKED", "RUNTIME_OUTSIDE_ALLOWED_ROOT", ""},
		{"wrong executable inside root", Input{Candidate: &model.Candidate{DeclaredExecutablePath: filepath.Join(allowed, "bin", "other"), ExecutableFileIdentity: "dev:1"}, Expectation: resolved, Artifact: observedMatch}, "UNKNOWN", "HOST_BINDING_AMBIGUOUS", ""},
		{"loaded image replaced", Input{Candidate: &model.Candidate{DeclaredExecutablePath: filepath.Join(allowed, "bin", "runtime"), ExecutableFileIdentity: "dev:old"}, Expectation: resolved, Artifact: observedMatch}, "UNKNOWN", "POSSIBLE_STALE_AFTER_REPLACEMENT", ""},
		{"known old digest", Input{Candidate: candidate, Expectation: resolved, Artifact: observedOld, KnownPriorDigests: map[string]bool{repeated('b'): true}}, "STALE", "ARTIFACT_MISMATCH", ""},
		{"unattributed mismatch", Input{Candidate: candidate, Expectation: resolved, Artifact: observedOld}, "UNKNOWN", "POSSIBLE_STALE_AFTER_REPLACEMENT", ""},
		{"not running", Input{Expectation: resolved, ProcessError: &processobserver.Error{Kind: processobserver.ErrorNotFound, Operation: "snapshot"}}, "NOT_RUNNING", "PROCESS_NOT_FOUND", ""},
		{"permission denied", Input{Expectation: resolved, ProcessError: &processobserver.Error{Kind: processobserver.ErrorInaccessible, Operation: "snapshot"}}, "UNKNOWN", "PROCESS_INACCESSIBLE", ""},
		{"identity race", Input{Expectation: resolved, ProcessError: &processobserver.Error{Kind: processobserver.ErrorIdentityChanged, Operation: "revalidate"}}, "UNKNOWN", "PROCESS_IDENTITY_CHANGED", ""},
		{"scan limit", Input{Candidate: candidate, Expectation: resolved, ArtifactError: &artifact.Error{Reason: "ARTIFACT_SCAN_LIMIT_EXCEEDED", Err: errors.New("limit")}}, "UNKNOWN", "ARTIFACT_SCAN_LIMIT_EXCEEDED", ""},
		{"missing expectation", Input{Candidate: candidate}, "UNKNOWN", "EXPECTATION_MISSING", "EXPECTATION_MISSING"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision := Evaluate(test.input)
			if decision.Verdict != test.verdict || len(decision.ReasonCodes) == 0 || decision.ReasonCodes[0] != test.reason {
				t.Fatalf("decision = %#v", decision)
			}
			if test.limitation != "" && !contains(decision.Limitations, test.limitation) {
				t.Fatalf("limitations = %v", decision.Limitations)
			}
		})
	}
}

func TestEvaluateRecordsUntrustedAndDynamicLimitations(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "fixture", "managed")
	resolved := &expectation.Resolved{ArtifactRoot: root, AllowedRoots: []string{root}}
	resolved.Value.Artifact.SHA256 = repeated('a')
	resolved.Value.Source.Trust = "untrusted"
	resolved.Value.Launch.Kind = "interpreter-script"
	resolved.Value.Launch.Entrypoint = "runtime"
	decision := Evaluate(Input{
		Candidate:   &model.Candidate{DeclaredExecutablePath: filepath.Join(root, "runtime"), ExecutableFileIdentity: "dev:1"},
		Expectation: resolved,
		Artifact:    &model.ArtifactObservation{SHA256: repeated('a'), EntrypointFileIdentity: "dev:1"},
	})
	if decision.Verdict != "UNKNOWN" || !contains(decision.Limitations, "EXPECTATION_UNTRUSTED") || !contains(decision.Limitations, "DYNAMIC_DEPENDENCIES_UNPROVEN") || !contains(decision.ReasonCodes, "PLATFORM_EVIDENCE_UNAVAILABLE") {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestEvaluateRefusesDeclaredArgumentsWithoutObservedFingerprints(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "fixture", "managed")
	resolved := &expectation.Resolved{ArtifactRoot: root, AllowedRoots: []string{root}}
	resolved.Value.Artifact.SHA256 = repeated('a')
	resolved.Value.Source.Trust = "verified"
	resolved.Value.Launch.Kind = "native"
	resolved.Value.Launch.Entrypoint = "runtime"
	resolved.Value.Launch.ArgumentFingerprints = []model.ArgumentFingerprint{{Position: 1, SHA256: repeated('c')}}
	decision := Evaluate(Input{
		Candidate:   &model.Candidate{DeclaredExecutablePath: filepath.Join(root, "runtime"), ExecutableFileIdentity: "dev:1"},
		Expectation: resolved,
		Artifact:    &model.ArtifactObservation{SHA256: repeated('a'), EntrypointFileIdentity: "dev:1"},
	})
	if decision.Verdict != "UNKNOWN" || !contains(decision.ReasonCodes, "PLATFORM_EVIDENCE_UNAVAILABLE") {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestEvaluateObservationOnlyRetainsProcessFailure(t *testing.T) {
	decision := Evaluate(Input{ProcessError: &processobserver.Error{Kind: processobserver.ErrorIdentityChanged, Operation: "revalidate"}})
	if decision.Verdict != "UNKNOWN" || !contains(decision.ReasonCodes, "EXPECTATION_MISSING") || !contains(decision.ReasonCodes, "PROCESS_IDENTITY_CHANGED") {
		t.Fatalf("decision = %#v", decision)
	}
}

func repeated(value byte) string {
	result := make([]byte, 64)
	for index := range result {
		result[index] = value
	}
	return string(result)
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
