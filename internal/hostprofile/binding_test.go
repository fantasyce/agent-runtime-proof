package hostprofile

import (
	"errors"
	"runtime"
	"testing"

	"github.com/fantasyce/agent-runtime-proof/internal/model"
)

func TestBindingMatchesOnlyOneExactResolvedExecutable(t *testing.T) {
	path, basename := nativeRuntimePath()
	binding := newBinding(RawBinding{HostID: "cursor", SourceID: "cursor-mcp", ServerName: "arp", Command: path, Args: []string{"mcp"}}, "sha256:"+repeatHex('a'))
	candidate := candidateAt(41, path, basename)
	matched, err := binding.Match([]model.Candidate{candidate})
	if err != nil || matched.Process.PID != 41 {
		t.Fatalf("match = %#v, %v", matched, err)
	}
	_, err = binding.Match(nil)
	assertHostError(t, err, "HOST_PROCESS_NOT_RUNNING")
	_, err = binding.Match([]model.Candidate{candidate, candidateAt(42, path, basename)})
	assertHostError(t, err, "HOST_PROCESS_AMBIGUOUS")
}

func TestBindingUsesSafeArgumentFingerprintsToDisambiguateSameExecutable(t *testing.T) {
	path, basename := nativeRuntimePath()
	binding := newBinding(RawBinding{HostID: "cursor", SourceID: "cursor-mcp", ServerName: "arp", Command: path, Args: []string{"mcp"}}, "sha256:"+repeatHex('a'))
	server := candidateAt(41, path, basename)
	server.ArgumentFingerprints = append([]model.ArgumentFingerprint{}, binding.ArgumentFingerprints...)
	cli := candidateAt(42, path, basename)
	cli.ArgumentFingerprints = []model.ArgumentFingerprint{{Position: 1, SHA256: hashValue("arp:host-argument:v1", "verify")}}
	matched, err := binding.Match([]model.Candidate{cli, server})
	if err != nil || matched.Process.PID != 41 {
		t.Fatalf("match = %#v, %v", matched, err)
	}
}

func TestBindingBasenameIsHintAndWrapperDoesNotBind(t *testing.T) {
	binding := newBinding(RawBinding{HostID: "cursor", SourceID: "cursor-mcp", ServerName: "arp", Command: "agent-runtime-proof", Args: []string{"mcp"}}, "sha256:"+repeatHex('a'))
	if binding.Confidence != "hint" {
		t.Fatalf("confidence = %q", binding.Confidence)
	}
	_, err := binding.Match([]model.Candidate{candidateAt(41, "/different/agent-runtime-proof", "agent-runtime-proof")})
	assertHostError(t, err, "HOST_PROCESS_NOT_RUNNING")
	_, err = binding.Match([]model.Candidate{candidateAt(42, "/usr/bin/node", "node")})
	assertHostError(t, err, "HOST_PROCESS_NOT_RUNNING")
	var target *Error
	if !errors.As(err, &target) {
		t.Fatal("missing typed error")
	}
}

func candidateAt(pid int, path, basename string) model.Candidate {
	return model.Candidate{Process: model.ProcessIdentity{PID: pid, CreatedAtUnixNano: "1", BootIDHash: "sha256:" + repeatHex('b')}, Executable: model.ExecutableObservation{Basename: basename}, ExecutablePath: "/proc/pid/exe", DeclaredExecutablePath: path, ArgumentFingerprints: []model.ArgumentFingerprint{{Position: 1, SHA256: hashValue("arp:host-argument:v1", "mcp")}}}
}

func nativeRuntimePath() (string, string) {
	if runtime.GOOS == "windows" {
		return `C:\opt\arp\agent-runtime-proof.exe`, "agent-runtime-proof.exe"
	}
	return "/opt/arp/agent-runtime-proof", "agent-runtime-proof"
}

func repeatHex(value byte) string { return string(makeFilled(64, value)) }
func makeFilled(count int, value byte) []byte {
	result := make([]byte, count)
	for index := range result {
		result[index] = value
	}
	return result
}
