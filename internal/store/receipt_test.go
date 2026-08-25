package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fantasyce/agent-runtime-proof/internal/receipt"
	sdkmodel "github.com/fantasyce/agent-runtime-proof/sdk/model"
)

func TestWriteReceiptPublishesContentAddressedFileWithoutTemporaryResidue(t *testing.T) {
	root := t.TempDir()
	value := testReceipt(t)
	path, err := WriteReceipt(root, value)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "launch-receipts", strings.TrimPrefix(value.ReceiptID, "sha256:")+".json")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	document, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := receipt.Validate(document); err != nil {
		t.Fatalf("stored receipt invalid: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(path) {
		t.Fatalf("receipt directory entries = %#v", entries)
	}
	assertStoredReceiptPermissions(t, path)
}

func TestWriteReceiptIsIdempotentForIdenticalContent(t *testing.T) {
	root := t.TempDir()
	value := testReceipt(t)
	first, err := WriteReceipt(root, value)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	second, err := WriteReceipt(root, value)
	if err != nil || second != first {
		t.Fatalf("second path=%q err=%v", second, err)
	}
	after, err := os.Stat(second)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("idempotent write replaced the receipt")
	}
}

func TestWriteReceiptRejectsExistingDifferentBytesAndCleansTemporaryFile(t *testing.T) {
	root := t.TempDir()
	value := testReceipt(t)
	directory := filepath.Join(root, "launch-receipts")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(directory, strings.TrimPrefix(value.ReceiptID, "sha256:")+".json")
	const sentinel = "do-not-overwrite"
	if err := os.WriteFile(target, []byte(sentinel), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteReceipt(root, value); err == nil {
		t.Fatal("conflicting existing bytes accepted")
	}
	contents, err := os.ReadFile(target)
	if err != nil || string(contents) != sentinel {
		t.Fatalf("target contents=%q err=%v", contents, err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(target) {
		t.Fatalf("temporary residue remains: %#v", entries)
	}
}

func TestWriteReceiptRejectsUnverifiedID(t *testing.T) {
	value := testReceipt(t)
	value.ReceiptID = "sha256:" + strings.Repeat("f", 64)
	if _, err := WriteReceipt(t.TempDir(), value); err == nil {
		t.Fatal("receipt with unverified caller-controlled ID accepted")
	}
}

func testReceipt(t *testing.T) sdkmodel.LaunchReceipt {
	t.Helper()
	value, err := receipt.Build(receipt.Input{
		CreatedAt:       time.Unix(1, 0),
		Tool:            sdkmodel.ToolInfo{Name: "agent-runtime-proof", Version: "0.1.0", Commit: "abcdef0", Toolchain: "go1.26.3"},
		Platform:        sdkmodel.Platform{OS: "linux", Arch: "amd64"},
		Process:         sdkmodel.ProcessIdentity{PID: 42, CreatedAtUnixNano: "1", BootIDHash: "sha256:" + strings.Repeat("a", 64)},
		Command:         sdkmodel.CommandObservation{ExecutableBasename: "helper", ExecutablePathHash: "sha256:" + strings.Repeat("b", 64)},
		ObservationOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := json.Marshal(value); err != nil {
		t.Fatal(err)
	}
	return value
}
