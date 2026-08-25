package hostprofile

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/fantasyce/agent-runtime-proof/internal/contract"
)

func TestEmbeddedCatalogContainsOnlyFrozenHosts(t *testing.T) {
	catalog, err := LoadEmbeddedCatalog()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"aaa", "claude-code", "codex", "cursor", "deepseek-harness", "opencode", "vscode-copilot"}
	if got := catalog.HostIDs(); !slices.Equal(got, want) {
		t.Fatalf("host IDs = %v, want %v", got, want)
	}
	for _, hostID := range want {
		profile, ok := catalog.Host(hostID)
		if !ok {
			t.Fatalf("missing profile %q", hostID)
		}
		encoded, err := json.Marshal(profile)
		if err != nil {
			t.Fatal(err)
		}
		if err := contract.ValidateHostProfile(encoded); err != nil {
			t.Fatalf("profile %q is invalid: %v", hostID, err)
		}
	}
}

func TestCatalogHostReturnsDefensiveCopy(t *testing.T) {
	catalog, err := LoadEmbeddedCatalog()
	if err != nil {
		t.Fatal(err)
	}
	first, ok := catalog.Host("codex")
	if !ok {
		t.Fatal("codex profile missing")
	}
	first.Platforms[0] = "mutated"
	first.ConfigSources[0].CandidatePaths[0] = "mutated"
	second, _ := catalog.Host("codex")
	if second.Platforms[0] == "mutated" || second.ConfigSources[0].CandidatePaths[0] == "mutated" {
		t.Fatal("catalog exposed mutable profile state")
	}
}

func TestProfileSemanticsRejectUnsafeValues(t *testing.T) {
	valid := Profile{
		SchemaVersion: "agent-runtime-host-profile/1.0",
		HostID:        "example-host", DisplayName: "Example Host", FixtureVersion: "1",
		Platforms:       []string{"darwin", "windows", "linux"},
		ProcessMatchers: []ProcessMatcher{{Basenames: []string{"example", "example.exe"}}},
		ConfigSources: []ConfigSource{{
			SourceID: "workspace", Platforms: []string{"darwin", "windows", "linux"},
			CandidatePaths: []string{"${WORKSPACE}/.example/mcp.json"}, Format: "json", Dialect: "mcp-servers", MaximumBytes: 1 << 20,
			SecretFields: []string{"env", "headers"},
		}},
	}
	cases := map[string]func(*Profile){
		"parent traversal":    func(value *Profile) { value.ConfigSources[0].CandidatePaths[0] = "${HOME}/../secret" },
		"literal home":        func(value *Profile) { value.ConfigSources[0].CandidatePaths[0] = "/Users/private/.example" },
		"unknown placeholder": func(value *Profile) { value.ConfigSources[0].CandidatePaths[0] = "${TOKEN}/config" },
		"unbounded bytes":     func(value *Profile) { value.ConfigSources[0].MaximumBytes = 0 },
		"duplicate source":    func(value *Profile) { value.ConfigSources = append(value.ConfigSources, value.ConfigSources[0]) },
		"secret default": func(value *Profile) {
			value.ConfigSources[0].SecretFields = append(value.ConfigSources[0].SecretFields, "token=private")
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			value := cloneProfile(valid)
			mutate(&value)
			encoded, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateProfile(encoded); err == nil {
				t.Fatal("unsafe profile accepted")
			}
		})
	}
}
