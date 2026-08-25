package hostprofile

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fantasyce/agent-runtime-proof/internal/model"
)

type ArgumentFingerprint struct {
	Position int    `json:"position"`
	SHA256   string `json:"sha256"`
}

type Binding struct {
	ID                   string                `json:"id"`
	HostID               string                `json:"host_id"`
	ServerName           string                `json:"server_name"`
	CommandBasename      string                `json:"command_basename"`
	CommandPathHash      string                `json:"command_path_hash,omitempty"`
	ArgumentFingerprints []ArgumentFingerprint `json:"argument_fingerprints"`
	ConfigSourceHash     string                `json:"config_source_hash"`
	Confidence           string                `json:"confidence"`
	resolvedCommand      string
	arguments            []string
}

func newBinding(raw RawBinding, configHash string) Binding {
	command := raw.Command
	confidence := "hint"
	if filepath.IsAbs(command) {
		command = filepath.Clean(command)
		confidence = "bound"
	}
	arguments := make([]ArgumentFingerprint, len(raw.Args))
	for index, argument := range raw.Args {
		arguments[index] = ArgumentFingerprint{Position: index + 1, SHA256: hashValue("arp:host-argument:v1", argument)}
	}
	result := Binding{
		ID: raw.HostID + "." + raw.ServerName, HostID: raw.HostID, ServerName: raw.ServerName,
		CommandBasename: filepath.Base(command), ArgumentFingerprints: arguments,
		ConfigSourceHash: configHash, Confidence: confidence,
		arguments: append([]string{}, raw.Args...),
	}
	if confidence == "bound" {
		result.resolvedCommand = command
		result.CommandPathHash = hashValue("arp:host-command-path:v1", normalizeCommandPath(command))
	}
	return result
}

func (binding Binding) Match(candidates []model.Candidate) (model.Candidate, error) {
	if binding.Confidence != "bound" || binding.resolvedCommand == "" {
		return model.Candidate{}, newError("HOST_PROCESS_NOT_RUNNING")
	}
	matches := make([]model.Candidate, 0, 1)
	for _, candidate := range candidates {
		candidatePath := candidate.DeclaredExecutablePath
		if candidatePath == "" {
			candidatePath = candidate.ExecutablePath
		}
		if sameCommandPath(candidatePath, binding.resolvedCommand) {
			matches = append(matches, candidate)
		}
	}
	switch len(matches) {
	case 0:
		return model.Candidate{}, newError("HOST_PROCESS_NOT_RUNNING")
	case 1:
		return matches[0], nil
	default:
		return model.Candidate{}, newError("HOST_PROCESS_AMBIGUOUS")
	}
}

func sameCommandPath(left, right string) bool {
	left, right = normalizeCommandPath(left), normalizeCommandPath(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func normalizeCommandPath(value string) string {
	value = filepath.Clean(value)
	if runtime.GOOS == "windows" {
		return strings.ToLower(value)
	}
	return value
}

func hashValue(domain, value string) string {
	digest := sha256.Sum256([]byte(domain + "\x00" + value))
	return "sha256:" + hex.EncodeToString(digest[:])
}
