package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestInstalledCommandTransportListsAndCallsToolsThenExitsOnEOF(t *testing.T) {
	if testing.Short() {
		t.Skip("subprocess acceptance")
	}
	binary := buildCandidate(t)
	var stderr bytes.Buffer
	command := exec.Command(binary, "mcp")
	command.Stderr = &stderr
	client := mcp.NewClient(&mcp.Implementation{Name: "official-go-sdk-fixture", Version: "1"}, &mcp.ClientOptions{Capabilities: &mcp.ClientCapabilities{}})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: command, TerminateDuration: time.Second}, nil)
	if err != nil {
		t.Fatal(err)
	}
	tools, err := session.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) != 3 {
		t.Fatalf("list tools err=%v count=%d stderr=%q", err, len(tools.Tools), stderr.String())
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "inspect_local_runtimes", Arguments: map[string]any{"pid": os.Getpid()}})
	if err != nil || result.IsError || result.StructuredContent == nil {
		t.Fatalf("call err=%v result=%#v stderr=%q", err, result, stderr.String())
	}
	if err := session.Close(); err != nil {
		t.Fatalf("EOF shutdown: %v, stderr=%q", err, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected server diagnostics: %q", stderr.String())
	}
}

func TestStdioNegotiatesPreviousStableProtocolWithoutStdoutPollution(t *testing.T) {
	binary := buildCandidate(t)
	var stderr bytes.Buffer
	command := exec.Command(binary, "mcp")
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(stdout)
	encoder := json.NewEncoder(stdin)
	if err := encoder.Encode(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "previous-fixture", "version": "1"}},
	}); err != nil {
		t.Fatal(err)
	}
	var initialized map[string]any
	if err := decoder.Decode(&initialized); err != nil {
		t.Fatalf("initialize response: %v, stderr=%q", err, stderr.String())
	}
	result, ok := initialized["result"].(map[string]any)
	if !ok || result["protocolVersion"] != "2025-06-18" {
		t.Fatalf("initialize response = %#v", initialized)
	}
	_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
	_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": map[string]any{}})
	var listed map[string]any
	if err := decoder.Decode(&listed); err != nil {
		t.Fatalf("tools/list response: %v, stderr=%q", err, stderr.String())
	}
	listedResult, ok := listed["result"].(map[string]any)
	tools, toolsOK := listedResult["tools"].([]any)
	if !ok || !toolsOK || len(tools) != 3 {
		t.Fatalf("tools/list response = %#v", listed)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("EOF exit: %v, stderr=%q", err, stderr.String())
		}
	case <-time.After(2 * time.Second):
		_ = command.Process.Kill()
		t.Fatal("server did not exit after EOF")
	}
	remaining, err := io.ReadAll(stdout)
	if len(bytes.TrimSpace(remaining)) != 0 || stderr.Len() != 0 {
		t.Fatalf("pollution remaining=%q readErr=%v stderr=%q", remaining, err, stderr.String())
	}
}

func TestMalformedOversizedInputTerminatesWithoutLeakOrProtocolPollution(t *testing.T) {
	binary := buildCandidate(t)
	var stdout, stderr bytes.Buffer
	command := exec.Command(binary, "mcp")
	command.Stdout = &stdout
	command.Stderr = &stderr
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(stdin, "{\"token-secret\":"+strings.Repeat("{", 2<<20)+"\n")
	_ = stdin.Close()
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = command.Process.Kill()
		t.Fatal("malformed input left a running server")
	}
	for _, line := range bytes.Split(bytes.TrimSpace(stdout.Bytes()), []byte{'\n'}) {
		if len(line) > 0 && !json.Valid(line) {
			t.Fatalf("non-protocol stdout: %q", line)
		}
	}
	if strings.Contains(stdout.String(), "token-secret") || strings.Contains(stderr.String(), "token-secret") {
		t.Fatalf("malformed input leaked content: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if stderr.Len() > 0 && stderr.String() != "agent-runtime-proof: MCP server failed\n" {
		t.Fatalf("unsafe diagnostic: %q", stderr.String())
	}
}

func buildCandidate(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "agent-runtime-proof")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-trimpath", "-o", binary, "../../cmd/agent-runtime-proof")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build installed candidate: %v: %s", err, output)
	}
	return binary
}
