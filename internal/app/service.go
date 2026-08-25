package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fantasyce/agent-runtime-proof/internal/artifact"
	"github.com/fantasyce/agent-runtime-proof/internal/evaluator"
	"github.com/fantasyce/agent-runtime-proof/internal/expectation"
	"github.com/fantasyce/agent-runtime-proof/internal/hostprofile"
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
	ResolveInline   func(model.Expectation) (expectation.Resolved, error)
	DigestArtifact  func(context.Context, expectation.Resolved, artifact.Clock) (model.ArtifactObservation, error)
	HostProfiles    HostProfileResolver
}

type HostProfileResolver interface {
	Binding(context.Context, string) (hostprofile.Binding, error)
	Bindings(context.Context, string) ([]hostprofile.Binding, error)
}

type InspectRequest struct {
	PID       int
	HostID    string
	BindingID string
	All       bool
	Limit     int
}

type InspectResult struct {
	Proofs []model.Proof `json:"proofs"`
}

type VerifyRequest struct {
	PID               int
	BindingID         string
	ExpectationPath   string
	Expectation       *model.Expectation
	KnownPriorDigests map[string]bool
}

type VerifyResult struct {
	Proof model.Proof `json:"proof"`
}

func NewService(observer processobserver.Observer, tool model.ToolInfo) *Service {
	return &Service{Observer: observer, Clock: wallClock{}, Tool: tool, LoadExpectation: expectation.Load, ResolveInline: expectation.ResolveInline, DigestArtifact: artifact.Digest, HostProfiles: hostprofile.NewLocalResolver()}
}

func (service *Service) Inspect(ctx context.Context, request InspectRequest) (InspectResult, error) {
	if err := ctx.Err(); err != nil {
		return InspectResult{}, err
	}
	selectors := 0
	if request.PID > 0 {
		selectors++
	}
	if request.BindingID != "" {
		selectors++
	}
	if request.All {
		selectors++
	}
	if selectors != 1 || request.PID < 0 || request.Limit < 0 || request.Limit > maximumInventoryLimit || (!request.All && request.Limit != 0) || (request.HostID != "" && !request.All) {
		return InspectResult{}, fmt.Errorf("%w: select exactly one of PID, binding, or all, with a bounded limit", ErrInvalidInput)
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
	if request.BindingID != "" {
		candidate, binding, bindingErr := service.bindingCandidate(ctx, request.BindingID)
		var candidatePointer *model.Candidate
		var host *model.HostAttribution
		if binding.ID != "" {
			host = attribution(binding)
		}
		if bindingErr == nil {
			candidatePointer = &candidate
			if err := service.Observer.Revalidate(ctx, candidate); err != nil {
				bindingErr = err
				candidatePointer = nil
			}
		}
		decision := evaluator.Evaluate(evaluator.Input{Candidate: candidatePointer, HostError: hostErrorOnly(request.BindingID, bindingErr), ProcessError: bindingErr})
		value, err := proof.Build(proof.Input{Candidate: candidatePointer, Decision: decision, Host: host, Tool: service.Tool, ObservedAt: service.now()})
		if err != nil {
			return InspectResult{}, err
		}
		return InspectResult{Proofs: []model.Proof{value}}, nil
	}
	if request.HostID != "" {
		return service.inspectHost(ctx, request.HostID, request.Limit)
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
	if (request.PID > 0) == (request.BindingID != "") || request.PID < 0 || (request.ExpectationPath == "") == (request.Expectation == nil) {
		return VerifyResult{}, fmt.Errorf("%w: verify requires exactly one of PID or binding and exactly one expectation source", ErrInvalidInput)
	}
	var resolved expectation.Resolved
	var err error
	if request.Expectation != nil {
		resolved, err = service.ResolveInline(*request.Expectation)
	} else {
		resolved, err = service.LoadExpectation(request.ExpectationPath)
	}
	if err != nil {
		return VerifyResult{}, fmt.Errorf("%w: load expectation: %v", ErrInvalidInput, err)
	}
	var candidate model.Candidate
	var processErr error
	var host *model.HostAttribution
	if request.BindingID != "" {
		var binding hostprofile.Binding
		candidate, binding, processErr = service.bindingCandidate(ctx, request.BindingID)
		if binding.ID != "" {
			host = attribution(binding)
		}
	} else {
		candidate, processErr = service.Observer.Snapshot(ctx, request.PID)
	}
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
			candidatePointer = nil
			artifactPointer = nil
			artifactErr = nil
		}
	}
	if err := ctx.Err(); err != nil {
		return VerifyResult{}, err
	}
	decision := evaluator.Evaluate(evaluator.Input{
		Candidate: candidatePointer, Expectation: &resolved, Artifact: artifactPointer,
		ProcessError: processErr, HostError: hostErrorOnly(request.BindingID, processErr), ArtifactError: artifactErr, KnownPriorDigests: request.KnownPriorDigests,
	})
	value, err := proof.Build(proof.Input{
		Candidate: candidatePointer, Expectation: &resolved, Artifact: artifactPointer,
		Decision: decision, Host: host, Tool: service.Tool, ObservedAt: service.now(),
	})
	if err != nil {
		return VerifyResult{}, err
	}
	return VerifyResult{Proof: value}, nil
}

func (service *Service) bindingCandidate(ctx context.Context, bindingID string) (model.Candidate, hostprofile.Binding, error) {
	if service.HostProfiles == nil {
		return model.Candidate{}, hostprofile.Binding{}, &hostprofile.Error{Code: "HOST_CONFIG_INACCESSIBLE"}
	}
	binding, err := service.HostProfiles.Binding(ctx, bindingID)
	if err != nil {
		return model.Candidate{}, hostprofile.Binding{}, err
	}
	candidates, err := service.Observer.List(ctx, maximumInventoryLimit)
	if err != nil {
		return model.Candidate{}, binding, err
	}
	candidate, err := binding.Match(candidates)
	return candidate, binding, err
}

func attribution(binding hostprofile.Binding) *model.HostAttribution {
	return &model.HostAttribution{HostID: binding.HostID, BindingID: binding.ID, ConfigSourceHash: binding.ConfigSourceHash, Confidence: binding.Confidence}
}

func (service *Service) inspectHost(ctx context.Context, hostID string, limit int) (InspectResult, error) {
	if service.HostProfiles == nil {
		return service.hostFailureProof(&hostprofile.Error{Code: "HOST_CONFIG_INACCESSIBLE"})
	}
	bindings, err := service.HostProfiles.Bindings(ctx, hostID)
	if err != nil {
		return service.hostFailureProof(err)
	}
	if limit == 0 {
		limit = defaultInventoryLimit
	}
	candidates, err := service.Observer.List(ctx, limit)
	if err != nil {
		return InspectResult{}, err
	}
	result := InspectResult{Proofs: make([]model.Proof, 0, len(bindings))}
	for _, binding := range bindings {
		candidate, matchErr := binding.Match(candidates)
		var candidatePointer *model.Candidate
		if matchErr == nil {
			candidatePointer = &candidate
			if err := service.Observer.Revalidate(ctx, candidate); err != nil {
				matchErr = err
				candidatePointer = nil
			}
		}
		decision := evaluator.Evaluate(evaluator.Input{Candidate: candidatePointer, HostError: hostErrorOnly(binding.ID, matchErr), ProcessError: matchErr})
		value, buildErr := proof.Build(proof.Input{Candidate: candidatePointer, Decision: decision, Host: attribution(binding), Tool: service.Tool, ObservedAt: service.now()})
		if buildErr != nil {
			return InspectResult{}, buildErr
		}
		result.Proofs = append(result.Proofs, value)
	}
	return result, nil
}

func (service *Service) hostFailureProof(hostErr error) (InspectResult, error) {
	decision := evaluator.Evaluate(evaluator.Input{HostError: hostErr})
	value, err := proof.Build(proof.Input{Decision: decision, Tool: service.Tool, ObservedAt: service.now()})
	if err != nil {
		return InspectResult{}, err
	}
	return InspectResult{Proofs: []model.Proof{value}}, nil
}

func hostErrorOnly(bindingID string, err error) error {
	if bindingID == "" || err == nil {
		return nil
	}
	var hostError *hostprofile.Error
	if errors.As(err, &hostError) {
		return err
	}
	return nil
}

func (service *Service) now() time.Time {
	if service.Clock == nil {
		return time.Now()
	}
	return service.Clock.Now()
}
