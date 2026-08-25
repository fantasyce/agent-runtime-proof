//go:build windows

package hostprofile

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDiscoverWindowsWorkspacePath(t *testing.T) {
	workspace := t.TempDir()
	home := t.TempDir()
	directory := filepath.Join(workspace, ".cursor")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	config := []byte(`{"mcpServers":{"arp":{"command":"C:\\ARP\\agent-runtime-proof.exe","args":["mcp"]}}}`)
	if err := os.WriteFile(filepath.Join(directory, "mcp.json"), config, 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := Discover(context.Background(), Request{HostID: "cursor", Platform: "windows", Home: home, Workspace: workspace})
	if err != nil || len(result.Bindings) != 1 {
		t.Fatalf("result = %#v, %v", result, err)
	}
	if result.Bindings[0].Confidence != "bound" || result.Bindings[0].CommandBasename != "agent-runtime-proof.exe" {
		t.Fatalf("binding = %#v", result.Bindings[0])
	}
}

func TestDiscoverRejectsWindowsJunctionAncestor(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	junction := filepath.Join(root, "junction")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "mcp.json"), []byte(`{"mcpServers":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("cmd.exe", "/d", "/c", "mklink", "/J", junction, target).CombinedOutput(); err != nil {
		t.Skipf("junction creation unavailable: %v (%s)", err, output)
	}
	_, err := Discover(context.Background(), Request{HostID: "cursor", Platform: "windows", ExplicitConfigPath: filepath.Join(junction, "mcp.json")})
	assertHostError(t, err, "HOST_CONFIG_INACCESSIBLE")
}
