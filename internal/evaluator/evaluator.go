package evaluator

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fantasyce/agent-runtime-proof/internal/artifact"
	"github.com/fantasyce/agent-runtime-proof/internal/expectation"
	"github.com/fantasyce/agent-runtime-proof/internal/model"
	processobserver "github.com/fantasyce/agent-runtime-proof/internal/process"
)

type Input struct {
	Candidate         *model.Candidate
	Expectation       *expectation.Resolved
	Artifact          *model.ArtifactObservation
	ProcessError      error
	ArtifactError     error
	KnownPriorDigests map[string]bool
}

type Decision struct {
	Verdict     string
	ProofLevel  string
	ReasonCodes []string
	Limitations []string
}

func Evaluate(input Input) Decision {
	if input.Expectation == nil {
		result := Decision{Verdict: "UNKNOWN", ProofLevel: "PROCESS_OBSERVED", ReasonCodes: []string{"EXPECTATION_MISSING"}, Limitations: []string{"EXPECTATION_MISSING"}}
		if reason := processReason(input.ProcessError); reason != "" {
			result.ReasonCodes = append(result.ReasonCodes, reason)
		}
		return result
	}
	if input.ProcessError != nil {
		var processError *processobserver.Error
		if errors.As(input.ProcessError, &processError) {
			switch processError.Kind {
			case processobserver.ErrorNotFound:
				return decision("NOT_RUNNING", "CONFIG_BOUND", "PROCESS_NOT_FOUND")
			case processobserver.ErrorInaccessible:
				return decision("UNKNOWN", "CONFIG_BOUND", "PROCESS_INACCESSIBLE")
			case processobserver.ErrorIdentityChanged:
				return decision("UNKNOWN", "CONFIG_BOUND", "PROCESS_IDENTITY_CHANGED")
			}
		}
		return decision("UNKNOWN", "CONFIG_BOUND", "PROCESS_INACCESSIBLE")
	}
	if input.Candidate == nil {
		return decision("UNKNOWN", "CONFIG_BOUND", "PROCESS_INACCESSIBLE")
	}
	if input.Expectation.Value.Launch.Kind == "native" {
		if !withinAny(input.Expectation.AllowedRoots, input.Candidate.DeclaredExecutablePath) {
			return decision("LEAKED", "CONFIG_BOUND", "RUNTIME_OUTSIDE_ALLOWED_ROOT")
		}
		expectedExecutable := filepath.Join(input.Expectation.ArtifactRoot, filepath.FromSlash(input.Expectation.Value.Launch.Entrypoint))
		if !equivalentPath(expectedExecutable, input.Candidate.DeclaredExecutablePath) {
			return decision("UNKNOWN", "CONFIG_BOUND", "HOST_BINDING_AMBIGUOUS")
		}
	}
	if input.ArtifactError != nil {
		var artifactError *artifact.Error
		if errors.As(input.ArtifactError, &artifactError) && artifactError.Reason != "" {
			return decision("UNKNOWN", "CONFIG_BOUND", artifactError.Reason)
		}
		return decision("UNKNOWN", "CONFIG_BOUND", "ARTIFACT_INACCESSIBLE")
	}
	if input.Artifact == nil {
		return decision("UNKNOWN", "CONFIG_BOUND", "ARTIFACT_INACCESSIBLE")
	}
	if input.Artifact.SHA256 != input.Expectation.Value.Artifact.SHA256 {
		if input.KnownPriorDigests[input.Artifact.SHA256] {
			return decision("STALE", "ARTIFACT_OBSERVED", "ARTIFACT_MISMATCH")
		}
		return decision("UNKNOWN", "ARTIFACT_OBSERVED", "POSSIBLE_STALE_AFTER_REPLACEMENT")
	}
	if input.Expectation.Value.Launch.Kind != "native" {
		result := decision("UNKNOWN", "ARTIFACT_OBSERVED", "PLATFORM_EVIDENCE_UNAVAILABLE")
		if input.Expectation.Value.Source.Trust == "untrusted" {
			result.Limitations = append(result.Limitations, "EXPECTATION_UNTRUSTED")
		}
		result.Limitations = append(result.Limitations, "DYNAMIC_DEPENDENCIES_UNPROVEN")
		return result
	}
	result := decision("MATCHED", "ARTIFACT_OBSERVED", "MATCH_CONFIRMED")
	if input.Expectation.Value.Source.Trust == "untrusted" {
		result.Limitations = append(result.Limitations, "EXPECTATION_UNTRUSTED")
	}
	return result
}

func processReason(value error) string {
	var processError *processobserver.Error
	if !errors.As(value, &processError) {
		return ""
	}
	switch processError.Kind {
	case processobserver.ErrorInaccessible:
		return "PROCESS_INACCESSIBLE"
	case processobserver.ErrorIdentityChanged:
		return "PROCESS_IDENTITY_CHANGED"
	default:
		return ""
	}
}

func decision(verdict, level, reason string) Decision {
	return Decision{Verdict: verdict, ProofLevel: level, ReasonCodes: []string{reason}, Limitations: []string{}}
}

func withinAny(roots []string, target string) bool {
	if target == "" {
		return false
	}
	for _, root := range roots {
		relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
		if err == nil && !filepath.IsAbs(relative) && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func equivalentPath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
