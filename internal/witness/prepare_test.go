package witness

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fantasyce/agent-runtime-proof/internal/model"
	"github.com/fantasyce/agent-runtime-proof/internal/process"
	sdkmodel "github.com/fantasyce/agent-runtime-proof/sdk/model"
)

func TestPrepareLaunchKeepsDirectArgumentsOnlyInMemory(t *testing.T) {
	const secret = "token-super-secret"
	controller := NewController(Dependencies{
		Tool: testTool(), Home: t.TempDir(),
		LookPath:     func(string) (string, error) { return "/safe/helper", nil },
		EvalSymlinks: func(path string) (string, error) { return path, nil },
	})
	prepared, err := controller.PrepareLaunch(context.Background(), Request{Command: []string{"helper", "$(touch should-not-run)", secret}})
	if err != nil {
		t.Fatal(err)
	}
	command, arguments := prepared.Command()
	if command != "/safe/helper" || len(arguments) != 2 || arguments[0] != "$(touch should-not-run)" || arguments[1] != secret {
		t.Fatalf("resolved command=%q arguments=%#v", command, arguments)
	}
	encoded, err := json.Marshal(prepared.safeCommand)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), secret) || strings.Contains(string(encoded), "touch should-not-run") {
		t.Fatalf("safe projection leaked argv: %s", encoded)
	}
	if len(prepared.safeCommand.ArgumentFingerprints) != 2 || prepared.safeCommand.ArgumentFingerprints[0].Position != 1 {
		t.Fatalf("argument fingerprints = %#v", prepared.safeCommand.ArgumentFingerprints)
	}
}

func TestPrepareLaunchRejectsInvalidCommandsWithoutEchoingThem(t *testing.T) {
	const secret = "private-command-secret"
	controller := NewController(Dependencies{Tool: testTool(), Home: t.TempDir(), LookPath: func(string) (string, error) {
		return "", errors.New("lookup failed for " + secret)
	}})
	for _, command := range [][]string{nil, {""}, {"helper", "bad\x00argument"}, {secret}} {
		_, err := controller.PrepareLaunch(context.Background(), Request{Command: command})
		if err == nil {
			t.Fatalf("command %#v accepted", command)
		}
		if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "bad\x00argument") {
			t.Fatalf("error leaked command data: %q", err)
		}
	}
}

func TestPrepareLaunchBindsNativeExpectationBeforeSpawn(t *testing.T) {
	root, helper, manifest := writeNativeExpectation(t, "9a70c7154e4b5e5ac94301830029eee533227d98c04c50080e7359d3047477de")
	controller := NewController(Dependencies{Tool: testTool(), Home: filepath.Join(root, "state")})
	prepared, err := controller.PrepareLaunch(context.Background(), Request{Command: []string{helper, "serve"}, ExpectationPath: manifest})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.expectation == nil || prepared.artifact == nil || prepared.artifact.SHA256 != "9a70c7154e4b5e5ac94301830029eee533227d98c04c50080e7359d3047477de" {
		t.Fatalf("prepared expectation=%#v artifact=%#v", prepared.expectation, prepared.artifact)
	}
	if prepared.observationOnly {
		t.Fatal("expectation-bound preparation marked observation-only")
	}
}

func TestPrepareLaunchRejectsArtifactOrArgumentMismatch(t *testing.T) {
	root, helper, manifest := writeNativeExpectation(t, strings.Repeat("0", 64))
	controller := NewController(Dependencies{Tool: testTool(), Home: filepath.Join(root, "state")})
	if _, err := controller.PrepareLaunch(context.Background(), Request{Command: []string{helper, "serve"}, ExpectationPath: manifest}); err == nil {
		t.Fatal("artifact mismatch accepted")
	}
	_, helper, manifest = writeNativeExpectation(t, "9a70c7154e4b5e5ac94301830029eee533227d98c04c50080e7359d3047477de")
	if _, err := controller.PrepareLaunch(context.Background(), Request{Command: []string{helper, "wrong"}, ExpectationPath: manifest}); err == nil {
		t.Fatal("argument fingerprint mismatch accepted")
	}
}

func TestPrepareLaunchBindsInterpreterEntrypointArgument(t *testing.T) {
	root := t.TempDir()
	interpreter := filepath.Join(root, "interpreter")
	if err := os.WriteFile(interpreter, []byte("interpreter"), 0o700); err != nil {
		t.Fatal(err)
	}
	payload := filepath.Join(root, "payload")
	if err := os.MkdirAll(payload, 0o700); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(payload, "script.sh")
	if err := os.WriteFile(script, []byte("helper bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(root, "expectation.json")
	document := `{
		"schema_version":"agent-runtime-expectation/1.0",
		"subject":{"id":"script","display_name":"Script","version":"1.0.0"},
		"launch":{"kind":"interpreter-script","entrypoint":"script.sh","argument_fingerprints":[]},
		"artifact":{"root":"payload","include":["script.sh"],"exclude":[],"sha256":"dd804989bd6acd7a6edb3d39d9b750766576f0867d9b79b3591548140eb28609","max_files":10,"max_bytes":1024,"max_duration_ms":10000},
		"policy":{"allowed_roots":["payload"],"allow_symlinks":false},
		"source":{"kind":"user-file","locator_hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","trust":"declared"}
	}`
	if err := os.WriteFile(manifest, []byte(document), 0o600); err != nil {
		t.Fatal(err)
	}
	controller := NewController(Dependencies{Tool: testTool(), Home: filepath.Join(root, "state"), LookPath: func(string) (string, error) { return interpreter, nil }})
	if _, err := controller.PrepareLaunch(context.Background(), Request{Command: []string{"interpreter", script}, ExpectationPath: manifest}); err != nil {
		t.Fatalf("interpreter launch rejected: %v", err)
	}
	if _, err := controller.PrepareLaunch(context.Background(), Request{Command: []string{"interpreter", manifest}, ExpectationPath: manifest}); err == nil {
		t.Fatal("wrong interpreter entrypoint accepted")
	}
}

func TestSpawnedRevalidatesIdentityAndStoresSafeReceipt(t *testing.T) {
	const secret = "token-super-secret"
	path := "/safe/helper"
	candidate := model.Candidate{
		Platform:               model.Platform{OS: "linux", Arch: "amd64"},
		Process:                model.ProcessIdentity{PID: 42, CreatedAtUnixNano: "1787536210123456789", BootIDHash: "sha256:" + strings.Repeat("a", 64)},
		Executable:             model.ExecutableObservation{Basename: "helper", PathHash: "sha256:" + strings.Repeat("b", 64)},
		DeclaredExecutablePath: path,
	}
	observer := &fakeObserver{candidate: candidate}
	var stored sdkmodel.LaunchReceipt
	controller := NewController(Dependencies{
		Tool: testTool(), Home: t.TempDir(), Observer: observer,
		Clock:        fixedClock{value: time.Unix(5, 7)},
		LookPath:     func(string) (string, error) { return path, nil },
		EvalSymlinks: func(value string) (string, error) { return value, nil },
		WriteReceipt: func(_ string, value sdkmodel.LaunchReceipt) (string, error) { stored = value; return "stored", nil },
	})
	prepared, err := controller.PrepareLaunch(context.Background(), Request{Command: []string{"helper", secret}})
	if err != nil {
		t.Fatal(err)
	}
	value, err := prepared.Spawned(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if observer.snapshotCalls != 2 || stored.ReceiptID != value.ReceiptID || value.Process.CreatedAtUnixNano == "" {
		t.Fatalf("snapshot calls=%d stored=%#v value=%#v", observer.snapshotCalls, stored, value)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), secret) || strings.Contains(string(encoded), path) {
		t.Fatalf("receipt leaked private data: %s", encoded)
	}
}

func writeNativeExpectation(t *testing.T, digest string) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(root, "bin", "helper")
	if err := os.WriteFile(helper, []byte("helper bytes"), 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(root, "expectation.json")
	document := `{
		"schema_version":"agent-runtime-expectation/1.0",
		"subject":{"id":"example","display_name":"Example","version":"1.0.0"},
		"launch":{"kind":"native","entrypoint":"bin/helper","argument_fingerprints":[{"position":1,"sha256":"24c458cfb46d9a456d314c7897601e51578e43c1f9dc007adf1a745bbc15e0f5"}]},
		"artifact":{"root":".","include":["bin/helper"],"exclude":[],"sha256":"` + digest + `","max_files":10,"max_bytes":1024,"max_duration_ms":10000},
		"policy":{"allowed_roots":["."],"allow_symlinks":false},
		"source":{"kind":"user-file","locator_hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","trust":"declared"}
	}`
	if err := os.WriteFile(manifest, []byte(document), 0o600); err != nil {
		t.Fatal(err)
	}
	return root, helper, manifest
}

type fakeObserver struct {
	candidate     model.Candidate
	snapshotCalls int
}

func (fake *fakeObserver) Snapshot(context.Context, int) (model.Candidate, error) {
	fake.snapshotCalls++
	return fake.candidate, nil
}

func (fake *fakeObserver) List(context.Context, int) ([]model.Candidate, error) { return nil, nil }

func (fake *fakeObserver) Revalidate(ctx context.Context, expected model.Candidate) error {
	actual, err := fake.Snapshot(ctx, expected.Process.PID)
	if err != nil {
		return err
	}
	if !process.SameIdentity(actual, expected) {
		return errors.New("identity changed")
	}
	return nil
}

type fixedClock struct{ value time.Time }

func (clock fixedClock) Now() time.Time { return clock.value }

func testTool() model.ToolInfo {
	return model.ToolInfo{Name: "agent-runtime-proof", Version: "0.1.0", Commit: "abcdef0", Toolchain: "go1.26.3"}
}
