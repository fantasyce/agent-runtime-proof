package hostprofile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

var errConfigChanged = errors.New("configuration changed during read")

type Error struct{ Code string }

func (value *Error) Error() string { return value.Code }
func newError(code string) error   { return &Error{Code: code} }

type Request struct {
	HostID             string
	Platform           string
	Home               string
	Workspace          string
	ProfileName        string
	ExplicitConfigPath string
}

type Result struct {
	Bindings []Binding `json:"bindings"`
}

var readConfigFile = readPinnedConfig

func Discover(ctx context.Context, request Request) (Result, error) {
	catalog, err := LoadEmbeddedCatalog()
	if err != nil {
		return Result{}, newError("HOST_PROFILE_INVALID")
	}
	profile, ok := catalog.Host(request.HostID)
	if !ok {
		return Result{}, newError("HOST_PROFILE_NOT_FOUND")
	}
	platform := request.Platform
	if platform == "" {
		platform = runtime.GOOS
	}
	if !contains(profile.Platforms, platform) {
		return Result{}, newError("HOST_PROFILE_UNSUPPORTED")
	}

	type input struct {
		source ConfigSource
		path   string
	}
	inputs := []input{}
	if request.ExplicitConfigPath != "" {
		source, sourceErr := selectExplicitSource(profile, platform, request.ExplicitConfigPath)
		if sourceErr != nil {
			return Result{}, sourceErr
		}
		inputs = append(inputs, input{source: source, path: request.ExplicitConfigPath})
	} else {
		for _, source := range profile.ConfigSources {
			if !contains(source.Platforms, platform) {
				continue
			}
			for _, candidate := range source.CandidatePaths {
				resolved, resolveErr := expandCandidate(candidate, request)
				if resolveErr != nil {
					return Result{}, newError("HOST_CONFIG_INACCESSIBLE")
				}
				inputs = append(inputs, input{source: source, path: resolved})
			}
		}
	}

	bindings := map[string]Binding{}
	for _, candidate := range inputs {
		select {
		case <-ctx.Done():
			return Result{}, newError("HOST_DISCOVERY_CANCELLED")
		default:
		}
		document, readErr := readConfigFile(ctx, candidate.path, candidate.source.MaximumBytes)
		if errors.Is(readErr, os.ErrNotExist) && request.ExplicitConfigPath == "" {
			continue
		}
		if readErr != nil {
			return Result{}, newError("HOST_CONFIG_INACCESSIBLE")
		}
		rawBindings, parseErr := parseConfig(profile, candidate.source, document)
		if parseErr != nil {
			return Result{}, newError("HOST_CONFIG_INVALID")
		}
		configHash := hashBytes("arp:host-config:v1", document)
		for _, raw := range rawBindings {
			binding := newBinding(raw, configHash)
			previous, exists := bindings[binding.ID]
			if exists && !equivalentBinding(previous, binding) {
				return Result{}, newError("HOST_BINDING_AMBIGUOUS")
			}
			if !exists {
				bindings[binding.ID] = binding
			}
		}
	}
	result := Result{Bindings: make([]Binding, 0, len(bindings))}
	for _, binding := range bindings {
		result.Bindings = append(result.Bindings, binding)
	}
	sort.Slice(result.Bindings, func(i, j int) bool { return result.Bindings[i].ID < result.Bindings[j].ID })
	return result, nil
}

func expandCandidate(template string, request Request) (string, error) {
	root := ""
	remainder := ""
	switch {
	case strings.HasPrefix(template, "${HOME}/"):
		root, remainder = request.Home, strings.TrimPrefix(template, "${HOME}/")
	case strings.HasPrefix(template, "${WORKSPACE}/"):
		root, remainder = request.Workspace, strings.TrimPrefix(template, "${WORKSPACE}/")
	default:
		return "", errors.New("unbounded template")
	}
	if root == "" || !filepath.IsAbs(root) {
		return "", errors.New("root is not absolute")
	}
	remainder = strings.ReplaceAll(remainder, "${PROFILE}", request.ProfileName)
	if strings.Contains(remainder, "${") || strings.ContainsRune(remainder, 0) {
		return "", errors.New("invalid placeholder")
	}
	for _, part := range strings.Split(filepath.ToSlash(remainder), "/") {
		if part == "" || part == "." || part == ".." {
			return "", errors.New("invalid component")
		}
	}
	resolved := filepath.Join(root, filepath.FromSlash(remainder))
	relative, err := filepath.Rel(filepath.Clean(root), resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("path escaped root")
	}
	return resolved, nil
}

func selectExplicitSource(profile Profile, platform, path string) (ConfigSource, error) {
	extension := strings.ToLower(filepath.Ext(path))
	format := map[string]string{".json": "json", ".jsonc": "jsonc", ".toml": "toml", ".yaml": "yaml", ".yml": "yaml"}[extension]
	var candidates []ConfigSource
	for _, source := range profile.ConfigSources {
		if contains(source.Platforms, platform) && (source.Format == format || format == "") {
			candidates = append(candidates, source)
		}
	}
	if len(candidates) == 0 {
		return ConfigSource{}, newError("HOST_CONFIG_INVALID")
	}
	return candidates[0], nil
}

func equivalentBinding(left, right Binding) bool {
	if left.ID != right.ID || left.ConfigSourceHash != right.ConfigSourceHash || left.resolvedCommand != right.resolvedCommand || len(left.arguments) != len(right.arguments) {
		return false
	}
	for index := range left.arguments {
		if left.arguments[index] != right.arguments[index] {
			return false
		}
	}
	return true
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func hashBytes(domain string, value []byte) string {
	digest := sha256.Sum256(append([]byte(domain+"\x00"), value...))
	return "sha256:" + hex.EncodeToString(digest[:])
}
