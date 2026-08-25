package receipt

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	sdkmodel "github.com/fantasyce/agent-runtime-proof/sdk/model"
)

func TestBuildAssignsAValidContentID(t *testing.T) {
	value, err := Build(Input{
		CreatedAt:       time.Date(2026, 8, 25, 12, 34, 56, 123, time.FixedZone("test", 8*60*60)),
		Tool:            sdkmodel.ToolInfo{Name: "agent-runtime-proof", Version: "0.1.0", Commit: "abcdef0", Toolchain: "go1.26.3"},
		Platform:        sdkmodel.Platform{OS: "darwin", Arch: "arm64"},
		Process:         sdkmodel.ProcessIdentity{PID: 42, CreatedAtUnixNano: "1787536210123456789", BootIDHash: "sha256:" + strings.Repeat("b", 64)},
		Command:         sdkmodel.CommandObservation{ExecutableBasename: "helper", ExecutablePathHash: "sha256:" + strings.Repeat("c", 64), ArgumentFingerprints: []sdkmodel.ArgumentFingerprint{{Position: 1, SHA256: "sha256:" + strings.Repeat("d", 64)}}},
		ObservationOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(value.ReceiptID) != len("sha256:")+64 || value.CreatedAt != "2026-08-25T04:34:56.000000123Z" {
		t.Fatalf("receipt_id=%q created_at=%q", value.ReceiptID, value.CreatedAt)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Validate(encoded); err != nil {
		t.Fatalf("built receipt does not validate: %v\n%s", err, encoded)
	}
}

func TestValidateRejectsProtectedFieldMutation(t *testing.T) {
	value := minimalReceipt(t)
	value.Command.ExecutableBasename = "changed"
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Validate(encoded); err == nil {
		t.Fatal("receipt with stale ID accepted")
	}
}

func TestBuildRejectsContradictoryObservationOnlyInput(t *testing.T) {
	_, err := Build(Input{
		CreatedAt:       time.Now(),
		Tool:            sdkmodel.ToolInfo{Name: "agent-runtime-proof", Version: "0.1.0", Commit: "abcdef0", Toolchain: "go1.26.3"},
		Platform:        sdkmodel.Platform{OS: "linux", Arch: "amd64"},
		Process:         sdkmodel.ProcessIdentity{PID: 42, CreatedAtUnixNano: "1", BootIDHash: "sha256:" + strings.Repeat("b", 64)},
		Command:         sdkmodel.CommandObservation{ExecutableBasename: "helper", ExecutablePathHash: "sha256:" + strings.Repeat("c", 64)},
		ObservationOnly: true,
		Expectation:     &sdkmodel.ExpectationProjection{SourceKind: "verified", SourceLocatorHash: strings.Repeat("a", 64), Trust: "verified", ExpectedVersion: "1.0.0", ExpectedArtifactSHA256: strings.Repeat("d", 64)},
	})
	if err == nil {
		t.Fatal("observation-only receipt accepted expectation evidence")
	}
}

func minimalReceipt(t *testing.T) sdkmodel.LaunchReceipt {
	t.Helper()
	value, err := Build(Input{
		CreatedAt:       time.Unix(1, 0),
		Tool:            sdkmodel.ToolInfo{Name: "agent-runtime-proof", Version: "0.1.0", Commit: "abcdef0", Toolchain: "go1.26.3"},
		Platform:        sdkmodel.Platform{OS: "linux", Arch: "amd64"},
		Process:         sdkmodel.ProcessIdentity{PID: 42, CreatedAtUnixNano: "1", BootIDHash: "sha256:" + strings.Repeat("b", 64)},
		Command:         sdkmodel.CommandObservation{ExecutableBasename: "helper", ExecutablePathHash: "sha256:" + strings.Repeat("c", 64)},
		ObservationOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return value
}
