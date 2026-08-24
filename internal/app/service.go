package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fantasyce/agent-runtime-proof/internal/artifact"
	"github.com/fantasyce/agent-runtime-proof/internal/evaluator"
	"github.com/fantasyce/agent-runtime-proof/internal/expectation"
	"github.com/fantasyce/agent-runtime-proof/internal/model"
	processobserver "github.com/fantasyce/agent-runtime-proof/internal/process"
	"github.com/fantasyce/agent-runtime-proof/internal/proof"
)

var ErrInvalidInput = errors.New("invalid input")

const (
	defaultInventoryLimit = 256
	maximumInventoryLimit = 4096
)

type Clock interface {
	Now() time.Time
}

type wallClock struct{}

func (wallClock) Now() time.Time { return time.Now() }

type Service struct {
	Observer        processobserver.Observer
	Clock           Clock
	Tool            model.ToolInfo
	LoadExpectation func(string) (expectation.Resolved, error)
	DigestArtifact  func(context.Context, expectation.Resolved, artifact.Clock) (model.ArtifactObservation, error)
}

type InspectRequest struct {
	PID   int
	All   bool
	Limit int
}

type InspectResult struct {
	Proofs []model.Proof `json:"proofs"`
}

type VerifyRequest struct {
	PID               int
	ExpectationPath   string
	KnownPriorDigests map[string]bool
}

type VerifyResult struct {
	Proof model.Proof `json:"proof"`
}

func NewService(observer processobserver.Observer, tool model.ToolInfo) *Service {
	return &Service{Observer: observer, Clock: wallClock{}, Tool: tool, LoadExpectation: expectation.Load, DigestArtifact: artifact.Digest}
}

func (service *Service) Inspect(ctx context.Context, request InspectRequest) (InspectResult, error) {
	if err := ctx.Err(); err != nil {
		return InspectResult{}, err
	}
	if request.PID > 0 == request.All || request.PID < 0 || request.Limit < 0 || request.Limit > maximumInventoryLimit {
		return InspectResult{}, fmt.Errorf("%w: select exactly one of PID or all, with a bounded limit", ErrInvalidInput)
	}
	if service.Observer == nil {
		return InspectResult{}, errors.New("process observer is unavailable")
	}
	if request.PID > 0 {
		candidate, snapshotErr := service.Observer.Snapshot(ctx, request.PID)
		if err := ctx.Err(); err != nil {
			return InspectResult{}, err
		}
		var candidatePointer *model.Candidate
		if snapshotErr == nil {
			candidatePointer = &candidate
			if err := service.Observer.Revalidate(ctx, candidate); err != nil {
				snapshotErr = err
				candidatePointer = nil
			}
		}
		decision := evaluator.Evaluate(evaluator.Input{Candidate: candidatePointer, ProcessError: snapshotErr})
		value, err := proof.Build(proof.Input{Candidate: candidatePointer, Decision: decision, Tool: service.Tool, ObservedAt: service.now()})
		if err != nil {
			return InspectResult{}, err
		}
		return InspectResult{Proofs: []model.Proof{value}}, nil
	}
	limit := request.Limit
	if limit == 0 {
		limit = defaultInventoryLimit
	}
	candidates, err := service.Observer.List(ctx, limit)
	if err != nil {
		if contextError := ctx.Err(); contextError != nil {
			return InspectResult{}, contextError
		}
		return InspectResult{}, err
	}
	result := InspectResult{Proofs: make([]model.Proof, 0, len(candidates))}
	for index := range candidates {
		if err := ctx.Err(); err != nil {
			return InspectResult{}, err
		}
		candidate := candidates[index]
		processErr := service.Observer.Revalidate(ctx, candidate)
		var candidatePointer *model.Candidate
		if processErr == nil {
			candidatePointer = &candidate
		}
		decision := evaluator.Evaluate(evaluator.Input{Candidate: candidatePointer, ProcessError: processErr})
		value, err := proof.Build(proof.Input{Candidate: candidatePointer, Decision: decision, Tool: service.Tool, ObservedAt: service.now()})
		if err != nil {
			return InspectResult{}, err
		}
		result.Proofs = append(result.Proofs, value)
	}
	return result, nil
}

func (service *Service) Verify(ctx context.Context, request VerifyRequest) (VerifyResult, error) {
	if err := ctx.Err(); err != nil {
		return VerifyResult{}, err
	}
	if request.PID <= 0 || request.ExpectationPath == "" {
		return VerifyResult{}, fmt.Errorf("%w: verify requires a positive PID and expectation path", ErrInvalidInput)
	}
	resolved, err := service.LoadExpectation(request.ExpectationPath)
	if err != nil {
		return VerifyResult{}, fmt.Errorf("%w: load expectation: %v", ErrInvalidInput, err)
	}
	candidate, processErr := service.Observer.Snapshot(ctx, request.PID)
	if err := ctx.Err(); err != nil {
		return VerifyResult{}, err
	}
	var candidatePointer *model.Candidate
	var artifactPointer *model.ArtifactObservation
	var artifactErr error
	if processErr == nil {
		candidatePointer = &candidate
		observedArtifact, digestErr := service.DigestArtifact(ctx, resolved, service.Clock)
		artifactErr = digestErr
		if digestErr == nil {
			artifactPointer = &observedArtifact
		}
		if revalidateErr := service.Observer.Revalidate(ctx, candidate); revalidateErr != nil {
			processErr = revalidateErr
			artifactPointer = nil
			artifactErr = nil
		}
	}
	if err := ctx.Err(); err != nil {
		return VerifyResult{}, err
	}
	decision := evaluator.Evaluate(evaluator.Input{
		Candidate: candidatePointer, Expectation: &resolved, Artifact: artifactPointer,
		ProcessError: processErr, ArtifactError: artifactErr, KnownPriorDigests: request.KnownPriorDigests,
	})
	value, err := proof.Build(proof.Input{
		Candidate: candidatePointer, Expectation: &resolved, Artifact: artifactPointer,
		Decision: decision, Tool: service.Tool, ObservedAt: service.now(),
	})
	if err != nil {
		return VerifyResult{}, err
	}
	return VerifyResult{Proof: value}, nil
}

func (service *Service) now() time.Time {
	if service.Clock == nil {
		return time.Now()
	}
	return service.Clock.Now()
}
