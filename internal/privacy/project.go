package privacy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/fantasyce/agent-runtime-proof/internal/expectation"
	"github.com/fantasyce/agent-runtime-proof/internal/model"
)

type Projection struct {
	Platform    model.Platform
	Subject     model.Subject
	Expectation *model.ExpectationProjection
	Observation model.Observation
	Evidence    []model.EvidenceItem
	Privacy     model.PrivacyProjection
}

func Project(candidate *model.Candidate, resolved *expectation.Resolved, artifact *model.ArtifactObservation, observedAt string) (Projection, error) {
	platform := model.Platform{OS: runtime.GOOS, Arch: runtime.GOARCH}
	if candidate != nil {
		platform = candidate.Platform
	}
	subject := inferredSubject(candidate)
	var expectationProjection *model.ExpectationProjection
	if resolved != nil {
		subject = safeSubject(resolved.Value.Subject)
		expectationProjection = &model.ExpectationProjection{
			SourceKind: resolved.Value.Source.Kind, SourceLocatorHash: resolved.Value.Source.LocatorHash,
			Trust: resolved.Value.Source.Trust, ExpectedVersion: truncateRunes(safeField(resolved.Value.Subject.Version), 64),
			ExpectedArtifactSHA256: resolved.Value.Artifact.SHA256,
		}
	}
	observation := model.Observation{Artifact: artifact, InaccessibleFields: []string{}}
	evidence := []model.EvidenceItem{}
	if candidate != nil {
		processIdentity := candidate.Process
		executable := candidate.Executable
		executable.Basename = safeField(executable.Basename)
		observation.Process = &processIdentity
		observation.Executable = &executable
		observation.InaccessibleFields = append([]string{}, candidate.Inaccessible...)
		digest, err := hashJSON(processIdentity)
		if err != nil {
			return Projection{}, err
		}
		evidence = append(evidence, model.EvidenceItem{Type: "process_identity", Digest: digest, ObservedAt: observedAt})
	}
	if artifact != nil {
		evidence = append(evidence, model.EvidenceItem{Type: "artifact_digest", Digest: "sha256:" + artifact.SHA256, ObservedAt: observedAt})
	}
	return Projection{
		Platform: platform, Subject: subject, Expectation: expectationProjection, Observation: observation, Evidence: evidence,
		Privacy: model.PrivacyProjection{RedactionMode: "safe-default", HomeRedacted: true, OmittedFields: []string{"paths.local", "process.argv", "process.environment"}},
	}, nil
}

func inferredSubject(candidate *model.Candidate) model.Subject {
	name := "unknown-runtime"
	if candidate != nil && candidate.Executable.Basename != "" {
		name = truncateRunes(candidate.Executable.Basename, 128)
	}
	name = safeField(name)
	id := sanitizeID(name)
	return model.Subject{ID: id, DisplayName: name, Version: "unknown"}
}

func safeSubject(value model.Subject) model.Subject {
	return model.Subject{ID: sanitizeID(safeField(value.ID)), DisplayName: safeField(value.DisplayName), Version: truncateRunes(safeField(value.Version), 64)}
}

func safeField(value string) string {
	lower := strings.ToLower(value)
	sensitiveMarkers := []string{"token", "secret", "cookie", "password", "passwd", "private-key", "credential", "authorization", "bearer", "api-key", "apikey"}
	sensitive := strings.ContainsAny(value, "/\\\x00")
	for _, marker := range sensitiveMarkers {
		sensitive = sensitive || strings.Contains(lower, marker)
	}
	sensitive = sensitive || strings.HasPrefix(lower, "sk-") || strings.HasPrefix(lower, "ghp_") || strings.HasPrefix(lower, "github_pat_") || strings.HasPrefix(strings.ToUpper(value), "AKIA")
	if !sensitive && tokenLike(value) {
		sensitive = true
	}
	if !sensitive {
		return truncateRunes(value, 128)
	}
	digest := sha256.Sum256([]byte("arp:safe-label:v1\x00" + value))
	return "redacted-" + hex.EncodeToString(digest[:6])
}

func tokenLike(value string) bool {
	if len(value) < 32 {
		return false
	}
	for _, char := range value {
		if char > 127 || !(char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || strings.ContainsRune("_-.=+", char)) {
			return false
		}
	}
	return true
}

func sanitizeID(value string) string {
	var builder strings.Builder
	for _, char := range value {
		if char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '.' || char == '_' || char == '-' {
			builder.WriteRune(char)
		} else {
			builder.WriteByte('-')
		}
		if builder.Len() >= 128 {
			break
		}
	}
	result := strings.Trim(builder.String(), ".-_")
	if result == "" {
		return "runtime"
	}
	return result
}

func truncateRunes(value string, limit int) string {
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	return string([]rune(value)[:limit])
}

func hashJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}
