package expectation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/fantasyce/agent-runtime-proof/internal/contract"
	"github.com/fantasyce/agent-runtime-proof/internal/model"
)

const maxExpectationBytes = 1 << 20

type Resolved struct {
	Value        model.Expectation
	ManifestPath string
	ArtifactRoot string
	AllowedRoots []string
}

func Load(inputPath string) (Resolved, error) {
	manifestPath, err := filepath.Abs(inputPath)
	if err != nil {
		return Resolved{}, fmt.Errorf("resolve expectation path: %w", err)
	}
	manifestPath, err = filepath.EvalSymlinks(manifestPath)
	if err != nil {
		return Resolved{}, fmt.Errorf("resolve expectation file: %w", err)
	}
	document, err := readBounded(manifestPath)
	if err != nil {
		return Resolved{}, err
	}
	if err := contract.ValidateExpectation(document); err != nil {
		return Resolved{}, err
	}
	var value model.Expectation
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return Resolved{}, fmt.Errorf("decode expectation: %w", err)
	}
	if err := validateSemantics(value); err != nil {
		return Resolved{}, err
	}

	base := filepath.Dir(manifestPath)
	allowedRoots := make([]string, 0, len(value.Policy.AllowedRoots))
	for _, declared := range value.Policy.AllowedRoots {
		resolved, err := resolveExistingPath(base, declared)
		if err != nil {
			return Resolved{}, fmt.Errorf("resolve allowed root: %w", err)
		}
		allowedRoots = append(allowedRoots, resolved)
	}
	declaredRoot, err := expandDeclaredPath(base, value.Artifact.Root)
	if err != nil {
		return Resolved{}, fmt.Errorf("resolve artifact root: %w", err)
	}
	artifactRoot, err := filepath.EvalSymlinks(declaredRoot)
	if err != nil {
		return Resolved{}, fmt.Errorf("resolve artifact root: %w", err)
	}
	if !value.Policy.AllowSymlinks && filepath.Clean(declaredRoot) != artifactRoot {
		return Resolved{}, errors.New("artifact root traverses a symbolic link")
	}
	inside := false
	for _, allowedRoot := range allowedRoots {
		if pathWithin(allowedRoot, artifactRoot) {
			inside = true
			break
		}
	}
	if !inside {
		return Resolved{}, errors.New("artifact root is outside every allowed root")
	}
	entrypoint := filepath.Join(artifactRoot, filepath.FromSlash(value.Launch.Entrypoint))
	if !pathWithin(artifactRoot, entrypoint) {
		return Resolved{}, errors.New("launch entrypoint escapes artifact root")
	}
	return Resolved{
		Value: value, ManifestPath: manifestPath, ArtifactRoot: artifactRoot, AllowedRoots: allowedRoots,
	}, nil
}

func readBounded(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open expectation: %w", err)
	}
	defer file.Close()
	contents, err := io.ReadAll(io.LimitReader(file, maxExpectationBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read expectation: %w", err)
	}
	if len(contents) > maxExpectationBytes {
		return nil, errors.New("expectation exceeds 1 MiB limit")
	}
	return contents, nil
}

func validateSemantics(value model.Expectation) error {
	positions := map[int]bool{}
	for _, fingerprint := range value.Launch.ArgumentFingerprints {
		if positions[fingerprint.Position] {
			return fmt.Errorf("duplicate argument fingerprint position %d", fingerprint.Position)
		}
		positions[fingerprint.Position] = true
	}
	if err := validateRelativeSlashPath(value.Launch.Entrypoint); err != nil {
		return fmt.Errorf("invalid launch entrypoint: %w", err)
	}
	for _, pattern := range append(append([]string{}, value.Artifact.Include...), value.Artifact.Exclude...) {
		if err := validatePattern(pattern); err != nil {
			return fmt.Errorf("invalid artifact pattern %q: %w", pattern, err)
		}
	}
	return nil
}

func expandDeclaredPath(base, declared string) (string, error) {
	if declared == "$HOME" || strings.HasPrefix(declared, "$HOME/") || declared == "%USERPROFILE%" || strings.HasPrefix(declared, "%USERPROFILE%/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if separator := strings.IndexByte(declared, '/'); separator >= 0 {
			declared = filepath.Join(home, filepath.FromSlash(declared[separator+1:]))
		} else {
			declared = home
		}
	}
	if !filepath.IsAbs(declared) {
		declared = filepath.Join(base, filepath.FromSlash(declared))
	}
	return filepath.Abs(filepath.Clean(declared))
}

func resolveExistingPath(base, declared string) (string, error) {
	resolved, err := expandDeclaredPath(base, declared)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(resolved)
}

func pathWithin(root, target string) bool {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
