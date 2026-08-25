package witness_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sdkmodel "github.com/fantasyce/agent-runtime-proof/sdk/model"
	"github.com/fantasyce/agent-runtime-proof/sdk/witness"
)

func TestEmbeddingPrepareLaunchAndSpawnedUseThePublicReceiptContract(t *testing.T) {
	home := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	controller, err := witness.New(witness.Config{
		Home: home,
		Tool: sdkmodel.ToolInfo{Name: "agent-runtime-proof", Version: "0.1.0", Commit: "abcdef0", Toolchain: "go1.26.3"},
	})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := controller.PrepareLaunch(context.Background(), witness.Request{Command: []string{executable}})
	if err != nil {
		t.Fatal(err)
	}
	command, arguments := prepared.Command()
	if command == "" || arguments == nil || len(arguments) != 0 {
		t.Fatalf("command=%q arguments=%#v", command, arguments)
	}
	arguments = append(arguments, "mutated")
	_, second := prepared.Command()
	if len(second) != 0 {
		t.Fatalf("Command returned mutable state: %#v", second)
	}
	value, err := prepared.Spawned(context.Background(), os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if value.SchemaVersion != "agent-runtime-launch-receipt/1.0" || !value.ObservationOnly || value.Process.PID != os.Getpid() {
		t.Fatalf("receipt = %#v", value)
	}
	path := filepath.Join(home, "launch-receipts", strings.TrimPrefix(value.ReceiptID, "sha256:")+".json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stored receipt unavailable: %v", err)
	}
}

func TestEmbeddingRejectsInvalidToolIdentity(t *testing.T) {
	if _, err := witness.New(witness.Config{Home: t.TempDir(), Tool: sdkmodel.ToolInfo{Name: "other-tool"}}); err == nil {
		t.Fatal("invalid tool identity accepted")
	}
}
