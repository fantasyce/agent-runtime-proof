package process

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/fantasyce/agent-runtime-proof/internal/model"
)

func TestCandidateJSONNeverExposesRuntimePath(t *testing.T) {
	candidate := model.Candidate{
		Platform:       model.Platform{OS: "darwin", Arch: "arm64"},
		Process:        model.ProcessIdentity{PID: 42, CreatedAtUnixNano: "100", BootIDHash: "sha256:" + strings.Repeat("0", 64)},
		Executable:     model.ExecutableObservation{Basename: "runtime", PathHash: "sha256:" + strings.Repeat("1", 64)},
		ExecutablePath: "/Users/private/repository/token-secret/runtime",
	}
	encoded, err := json.Marshal(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "/Users/private") || strings.Contains(string(encoded), "token-secret") {
		t.Fatalf("candidate leaked runtime path: %s", encoded)
	}
}

func TestSameIdentityRejectsPIDReuseAndExecutableReplacement(t *testing.T) {
	base := model.Candidate{
		Process:    model.ProcessIdentity{PID: 42, CreatedAtUnixNano: "100", BootIDHash: "sha256:" + strings.Repeat("0", 64)},
		Executable: model.ExecutableObservation{Basename: "runtime", PathHash: "sha256:" + strings.Repeat("1", 64), FileIDHash: "sha256:" + strings.Repeat("2", 64)},
	}
	if !SameIdentity(base, base) {
		t.Fatal("identical candidate rejected")
	}
	mutations := []model.Candidate{base, base, base, base}
	mutations[0].Process.PID++
	mutations[1].Process.CreatedAtUnixNano = "101"
	mutations[2].Process.BootIDHash = "sha256:" + strings.Repeat("3", 64)
	mutations[3].Executable.FileIDHash = "sha256:" + strings.Repeat("4", 64)
	for _, mutation := range mutations {
		if SameIdentity(base, mutation) {
			t.Fatalf("identity mutation accepted: %#v", mutation)
		}
	}
}
