package proof

import (
	"encoding/json"
	"time"

	"github.com/fantasyce/agent-runtime-proof/internal/evaluator"
	"github.com/fantasyce/agent-runtime-proof/internal/expectation"
	"github.com/fantasyce/agent-runtime-proof/internal/model"
	"github.com/fantasyce/agent-runtime-proof/internal/privacy"
)

type Input struct {
	Candidate   *model.Candidate
	Expectation *expectation.Resolved
	Artifact    *model.ArtifactObservation
	Decision    evaluator.Decision
	Tool        model.ToolInfo
	ObservedAt  time.Time
	Host        *model.HostAttribution
}

func Build(input Input) (model.Proof, error) {
	observedAt := input.ObservedAt.UTC().Format(time.RFC3339Nano)
	projection, err := privacy.Project(input.Candidate, input.Expectation, input.Artifact, observedAt)
	if err != nil {
		return model.Proof{}, err
	}
	value := model.Proof{
		SchemaVersion: "agent-runtime-proof/1.0", ObservedAt: observedAt, Tool: input.Tool,
		Platform: projection.Platform, Subject: projection.Subject, Expectation: projection.Expectation,
		Observation: projection.Observation, HostAttribution: input.Host, Verdict: input.Decision.Verdict,
		ProofLevel: input.Decision.ProofLevel, ReasonCodes: append([]string{}, input.Decision.ReasonCodes...),
		Evidence: projection.Evidence, Privacy: projection.Privacy,
		Limitations: append([]string{}, input.Decision.Limitations...),
	}
	if err := AssignID(&value); err != nil {
		return model.Proof{}, err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return model.Proof{}, err
	}
	if _, err := Validate(encoded); err != nil {
		return model.Proof{}, err
	}
	return value, nil
}
