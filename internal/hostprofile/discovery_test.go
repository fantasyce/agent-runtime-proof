package hostprofile

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDiscoverExpandsBoundedPathsAndBuildsSafeBinding(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "work")
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(filepath.Join(workspace, ".cursor"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	rawCommand := "/opt/arp/agent-runtime-proof"
	wantBasename := "agent-runtime-proof"
	if runtime.GOOS == "windows" {
		rawCommand = `C:\ARP\agent-runtime-proof.exe`
		wantBasename = "agent-runtime-proof.exe"
	}
	config, err := json.Marshal(map[string]any{"mcpServers": map[string]any{"arp": map[string]any{"command": rawCommand, "args": []string{"mcp"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".cursor", "mcp.json"), config, 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := Discover(context.Background(), Request{HostID: "cursor", Platform: runtime.GOOS, Home: home, Workspace: workspace})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bindings) != 1 {
		t.Fatalf("bindings = %d", len(result.Bindings))
	}
	binding := result.Bindings[0]
	if binding.ID != "cursor.arp" || binding.HostID != "cursor" || binding.ServerName != "arp" {
		t.Fatalf("unexpected binding: %#v", binding)
	}
	if binding.CommandBasename != wantBasename || !strings.HasPrefix(binding.CommandPathHash, "sha256:") || !strings.HasPrefix(binding.ConfigSourceHash, "sha256:") {
		t.Fatalf("unsafe or incomplete binding: %#v", binding)
	}
	encoded := mustJSON(t, binding)
	if strings.Contains(encoded, rawCommand) || strings.Contains(encoded, `"mcp"`) {
		t.Fatalf("raw command data escaped: %s", encoded)
	}
	repeated, err := Discover(context.Background(), Request{HostID: "cursor", Platform: runtime.GOOS, Home: home, Workspace: workspace})
	if err != nil || repeated.Bindings[0].ConfigSourceHash != binding.ConfigSourceHash {
		t.Fatalf("configuration hash is not deterministic: %#v, %v", repeated, err)
	}
}

func TestDiscoverMissingOptionalAndExplicitConfig(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "custom.json")
	if err := os.WriteFile(configPath, []byte(`{"mcpServers":{"arp":{"command":"arp","args":["mcp"]}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	missing, err := Discover(context.Background(), Request{HostID: "cursor", Platform: runtime.GOOS, Home: filepath.Join(root, "missing-home"), Workspace: filepath.Join(root, "missing-work")})
	if err != nil || len(missing.Bindings) != 0 {
		t.Fatalf("missing optional files: %#v, %v", missing, err)
	}
	explicit, err := Discover(context.Background(), Request{HostID: "cursor", Platform: runtime.GOOS, ExplicitConfigPath: configPath})
	if err != nil || len(explicit.Bindings) != 1 {
		t.Fatalf("explicit result: %#v, %v", explicit, err)
	}
}

func TestDiscoverRejectsSymlinkAndReportsOnlyTypedSafeError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink privilege varies; junction coverage runs on Windows acceptance")
	}
	root := t.TempDir()
	realPath := filepath.Join(root, "real.json")
	linkPath := filepath.Join(root, "link.json")
	secret := "never-report-this-secret"
	if err := os.WriteFile(realPath, []byte(`{"mcpServers":{"arp":{"command":"arp","args":["`+secret+`"]}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Fatal(err)
	}
	_, err := Discover(context.Background(), Request{HostID: "cursor", Platform: runtime.GOOS, ExplicitConfigPath: linkPath})
	assertHostError(t, err, "HOST_CONFIG_INACCESSIBLE")
	if strings.Contains(err.Error(), root) || strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaked sensitive data: %v", err)
	}
}

func TestDiscoverDeduplicatesIdenticalBindingsAndRejectsConflict(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "work")
	home := filepath.Join(root, "home")
	for _, directory := range []string{filepath.Join(workspace, ".cursor"), filepath.Join(home, ".cursor")} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	first := []byte(`{"mcpServers":{"arp":{"command":"arp","args":["mcp"]}}}`)
	for _, path := range []string{filepath.Join(workspace, ".cursor", "mcp.json"), filepath.Join(home, ".cursor", "mcp.json")} {
		if err := os.WriteFile(path, first, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := Discover(context.Background(), Request{HostID: "cursor", Platform: runtime.GOOS, Home: home, Workspace: workspace})
	if err != nil || len(result.Bindings) != 1 {
		t.Fatalf("dedup: %#v, %v", result, err)
	}
	if err := os.WriteFile(filepath.Join(home, ".cursor", "mcp.json"), []byte(`{"mcpServers":{"arp":{"command":"other","args":["mcp"]}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Discover(context.Background(), Request{HostID: "cursor", Platform: runtime.GOOS, Home: home, Workspace: workspace})
	assertHostError(t, err, "HOST_BINDING_AMBIGUOUS")
}

func TestDiscoverRejectsMutationDuringPinnedRead(t *testing.T) {
	original := readConfigFile
	t.Cleanup(func() { readConfigFile = original })
	readConfigFile = func(_ context.Context, _ string, _ int64) ([]byte, error) { return nil, errConfigChanged }
	_, err := Discover(context.Background(), Request{HostID: "cursor", Platform: runtime.GOOS, ExplicitConfigPath: filepath.Join(t.TempDir(), "config.json")})
	assertHostError(t, err, "HOST_CONFIG_INACCESSIBLE")
}

func assertHostError(t *testing.T, err error, code string) {
	t.Helper()
	var target *Error
	if !errors.As(err, &target) || target.Code != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
