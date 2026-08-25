package hostprofile

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseConfigExtractsDirectStdioBindings(t *testing.T) {
	catalog, err := LoadEmbeddedCatalog()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		hostID, sourceID, fixture string
	}{
		{"codex", "codex-config", "codex/config.toml"},
		{"claude-code", "claude-project-mcp", "claude-code/.mcp.json"},
		{"cursor", "cursor-mcp", "cursor/.cursor/mcp.json"},
		{"opencode", "opencode-project-jsonc", "opencode/opencode.jsonc"},
		{"deepseek-harness", "dsh-cordis-patch", "deepseek-harness/.dsh/cordis.patch.yml"},
		{"vscode-copilot", "copilot-agent-mcp", "vscode-copilot/.mcp.json"},
	}
	for _, test := range tests {
		t.Run(test.hostID, func(t *testing.T) {
			profile, ok := catalog.Host(test.hostID)
			if !ok {
				t.Fatal("profile missing")
			}
			source := sourceByID(t, profile, test.sourceID)
			document, err := os.ReadFile(filepath.Join("..", "..", "testdata", "host-configs", test.fixture))
			if err != nil {
				t.Fatal(err)
			}
			bindings, err := parseConfig(profile, source, document)
			if err != nil {
				t.Fatal(err)
			}
			if len(bindings) != 1 {
				t.Fatalf("bindings = %d, want 1", len(bindings))
			}
			binding := bindings[0]
			if binding.ServerName != "agent-runtime-proof" || binding.Command != "/opt/arp/agent-runtime-proof" || len(binding.Args) != 1 || binding.Args[0] != "mcp" {
				t.Fatalf("unexpected binding: %#v", binding)
			}
			if strings.Contains(fmt.Sprintf("%#v", binding), "test-secret-must-not-leak") {
				t.Fatal("binding retained environment secret")
			}
		})
	}
}

func TestParseConfigRejectsExecutableOrAmbiguousData(t *testing.T) {
	profile := Profile{HostID: "example"}
	tests := []struct {
		name, format, dialect, document string
	}{
		{"duplicate JSON key", "json", "mcp-servers", `{"mcpServers":{"arp":{"command":"arp","command":"other","args":["mcp"]}}}`},
		{"shell wrapper", "json", "mcp-servers", `{"mcpServers":{"arp":{"command":"sh","args":["-lc","arp mcp"]}}}`},
		{"interpolation", "json", "mcp-servers", `{"mcpServers":{"arp":{"command":"${ARP_BIN}","args":["mcp"]}}}`},
		{"non-string env", "json", "mcp-servers", `{"mcpServers":{"arp":{"command":"arp","args":["mcp"],"env":{"TOKEN":7}}}}`},
		{"toml collision", "toml", "codex-toml", "mcp_servers = 1\n[mcp_servers.arp]\ncommand='arp'\nargs=['mcp']\n"},
		{"yaml javascript tag", "yaml", "dsh-cordis", "- insert:\n  - id: arp\n    name: '@deepseek-ai/dsh-mcp-client'\n    config:\n      serverName: arp\n      transport: stdio\n      command: !!js process.env.ARP\n      args: [mcp]\n"},
		{"yaml anchor", "yaml", "dsh-cordis", "- &row {insert: []}\n- *row\n"},
		{"yaml merge key", "yaml", "dsh-cordis", "- insert:\n  - <<: {name: '@deepseek-ai/dsh-mcp-client'}\n"},
		{"yaml non-scalar key", "yaml", "dsh-cordis", "? [a, b]\n: value\n"},
		{"empty command array", "json", "opencode-v2", `{"mcp":{"servers":{"arp":{"type":"local","command":[]}}}}`},
		{"NUL", "json", "mcp-servers", "{\x00}"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := ConfigSource{SourceID: "fixture", Format: test.format, Dialect: test.dialect, MaximumBytes: 1 << 20}
			if _, err := parseConfig(profile, source, []byte(test.document)); err == nil {
				t.Fatal("unsafe config accepted")
			}
		})
	}
}

func TestParseConfigEnforcesLimits(t *testing.T) {
	profile := Profile{HostID: "example"}
	source := ConfigSource{SourceID: "fixture", Format: "json", Dialect: "mcp-servers", MaximumBytes: 64}
	if _, err := parseConfig(profile, source, bytes.Repeat([]byte("x"), 65)); err == nil {
		t.Fatal("oversized config accepted")
	}
	args := make([]string, 129)
	for index := range args {
		args[index] = `"x"`
	}
	document := `{"mcpServers":{"arp":{"command":"arp","args":[` + strings.Join(args, ",") + `]}}}`
	source.MaximumBytes = 1 << 20
	if _, err := parseConfig(profile, source, []byte(document)); err == nil {
		t.Fatal("excessive argument count accepted")
	}
	deep := strings.Repeat(`{"x":`, maximumConfigDepth+1) + `null` + strings.Repeat(`}`, maximumConfigDepth+1)
	if _, err := parseConfig(profile, source, []byte(deep)); err == nil {
		t.Fatal("excessive nesting accepted")
	}
}

func sourceByID(t *testing.T, profile Profile, sourceID string) ConfigSource {
	t.Helper()
	for _, source := range profile.ConfigSources {
		if source.SourceID == sourceID {
			return source
		}
	}
	t.Fatalf("source %q missing", sourceID)
	return ConfigSource{}
}
