package proof

import (
	"encoding/json"
	"testing"

	"github.com/fantasyce/agent-runtime-proof/internal/contract"
	"github.com/fantasyce/agent-runtime-proof/internal/model"
)

func TestAssignIDProducesSchemaValidObservationOnlyProof(t *testing.T) {
	value := minimalProof()
	if err := AssignID(&value); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.ValidateProof(encoded); err != nil {
		t.Fatalf("observation-only proof is invalid: %v\n%s", err, encoded)
	}
	if len(value.ProofID) != len("sha256:")+64 {
		t.Fatalf("proof_id = %q", value.ProofID)
	}
}

func TestVerifyIDDetectsProtectedFieldChange(t *testing.T) {
	value := minimalProof()
	if err := AssignID(&value); err != nil {
		t.Fatal(err)
	}
	value.Subject.Version = "changed"
	if err := VerifyID(value); err == nil {
		t.Fatal("tampered proof ID accepted")
	}
}

func minimalProof() model.Proof {
	return model.Proof{
		SchemaVersion: "agent-runtime-proof/1.0",
		ObservedAt:    "2026-08-24T12:34:56Z",
		Tool: model.ToolInfo{
			Name: "agent-runtime-proof", Version: "0.1.0", Commit: "abcdef0", Toolchain: "go1.27.0",
		},
		Platform: model.Platform{OS: "darwin", Arch: "arm64"},
		Subject:  model.Subject{ID: "example", DisplayName: "Example", Version: "unknown"},
		Observation: model.Observation{
			Process:            &model.ProcessIdentity{PID: 42, CreatedAtUnixNano: "1787536210123456789", BootIDHash: "sha256:" + zeros64},
			Executable:         &model.ExecutableObservation{Basename: "example", PathHash: "sha256:" + ones64},
			InaccessibleFields: []string{},
		},
		Verdict:     "UNKNOWN",
		ProofLevel:  "PROCESS_OBSERVED",
		ReasonCodes: []string{"EXPECTATION_MISSING"},
		Evidence: []model.EvidenceItem{{
			Type: "process_identity", Digest: "sha256:" + twos64, ObservedAt: "2026-08-24T12:34:56Z",
		}},
		Privacy:     model.PrivacyProjection{RedactionMode: "safe-default", HomeRedacted: true, OmittedFields: []string{}},
		Limitations: []string{"EXPECTATION_MISSING"},
	}
}

const (
	zeros64 = "0000000000000000000000000000000000000000000000000000000000000000"
	ones64  = "1111111111111111111111111111111111111111111111111111111111111111"
	twos64  = "2222222222222222222222222222222222222222222222222222222222222222"
)
