package contracttest

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const schemaBaseURL = "https://agent-runtime-proof.dev/schemas/"

type decisionRegistry struct {
	SchemaVersion string `json:"schema_version"`
	Verdicts      []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"verdicts"`
	ProofLevels []struct {
		Name        string `json:"name"`
		Order       int    `json:"order"`
		Description string `json:"description"`
	} `json:"proof_levels"`
	ReasonCodes []struct {
		Code            string   `json:"code"`
		Category        string   `json:"category"`
		Description     string   `json:"description"`
		AllowedVerdicts []string `json:"allowed_verdicts"`
	} `json:"reason_codes"`
}

func compileSchema(t *testing.T, path string) *jsonschema.Schema {
	t.Helper()
	document := loadJSON(t, path)
	compiler := jsonschema.NewCompiler()
	location := schemaBaseURL + filepath.Base(path)
	if err := compiler.AddResource(location, document); err != nil {
		t.Fatalf("add schema %s: %v", path, err)
	}
	schema, err := compiler.Compile(location)
	if err != nil {
		t.Fatalf("compile schema %s: %v", path, err)
	}
	return schema
}

func loadDecisionRegistry(t *testing.T) decisionRegistry {
	t.Helper()
	document := loadJSON(t, "contracts/decision-registry.json")
	return decodeViaJSON[decisionRegistry](t, document)
}

func decodeViaJSON[T any](t *testing.T, value any) T {
	t.Helper()
	encoded := mustMarshalJSON(t, value)
	var decoded T
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode typed JSON: %v", err)
	}
	return decoded
}

func TestSchemasCompile(t *testing.T) {
	for _, path := range []string{
		"schemas/agent-runtime-expectation-1.0.schema.json",
		"schemas/agent-runtime-proof-1.0.schema.json",
	} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			compileSchema(t, path)
		})
	}
}

func TestContractFixtures(t *testing.T) {
	schemas := map[string]*jsonschema.Schema{
		"expectation": compileSchema(t, "schemas/agent-runtime-expectation-1.0.schema.json"),
		"proof":       compileSchema(t, "schemas/agent-runtime-proof-1.0.schema.json"),
	}
	for _, fixtureClass := range []string{"valid", "invalid"} {
		pattern := filepath.Join(repoRoot(t), "testdata", "contracts", fixtureClass, "*.json")
		paths, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}
		if len(paths) == 0 {
			t.Fatalf("no %s contract fixtures", fixtureClass)
		}
		for _, absolutePath := range paths {
			name := filepath.Base(absolutePath)
			kind := strings.SplitN(name, "-", 2)[0]
			schema := schemas[kind]
			if schema == nil {
				t.Fatalf("fixture %s has unknown contract prefix %q", name, kind)
			}
			document := loadJSON(t, filepath.ToSlash(strings.TrimPrefix(absolutePath, repoRoot(t)+string(filepath.Separator))))
			err := schema.Validate(document)
			if fixtureClass == "valid" && err != nil {
				t.Errorf("valid fixture %s rejected: %v", name, err)
			}
			if fixtureClass == "invalid" && err == nil {
				t.Errorf("invalid fixture %s accepted", name)
			}
		}
	}
}

func TestDecisionRegistryMatchesSchemas(t *testing.T) {
	registry := loadDecisionRegistry(t)
	proof := loadJSON(t, "schemas/agent-runtime-proof-1.0.schema.json").(map[string]any)

	wantVerdicts := registryVerdictNames(registry)
	wantProofLevels := registryProofLevelNames(registry)
	gotVerdicts := schemaEnum(t, proof, "verdict")
	gotProofLevels := schemaEnum(t, proof, "proofLevel")
	if !slices.Equal(gotVerdicts, wantVerdicts) {
		t.Fatalf("proof verdict enum = %v, registry = %v", gotVerdicts, wantVerdicts)
	}
	if !slices.Equal(gotProofLevels, wantProofLevels) {
		t.Fatalf("proof level enum = %v, registry = %v", gotProofLevels, wantProofLevels)
	}
}

func TestDecisionRegistryInvariants(t *testing.T) {
	registry := loadDecisionRegistry(t)
	if registry.SchemaVersion != "agent-runtime-decision-registry/1.0" {
		t.Fatalf("schema_version = %q", registry.SchemaVersion)
	}
	verdictSet := map[string]bool{}
	for _, verdict := range registry.Verdicts {
		requireUniqueNamedValue(t, verdictSet, verdict.Name, verdict.Description)
	}
	proofSet := map[string]bool{}
	for index, level := range registry.ProofLevels {
		requireUniqueNamedValue(t, proofSet, level.Name, level.Description)
		if level.Order != index+1 {
			t.Errorf("proof level %s order = %d, want %d", level.Name, level.Order, index+1)
		}
	}
	reasonSet := map[string]bool{}
	for _, reason := range registry.ReasonCodes {
		requireUniqueNamedValue(t, reasonSet, reason.Code, reason.Description)
		if strings.TrimSpace(reason.Category) == "" {
			t.Errorf("reason %s has empty category", reason.Code)
		}
		if len(reason.AllowedVerdicts) == 0 {
			t.Errorf("reason %s has no allowed verdicts", reason.Code)
		}
		for _, verdict := range reason.AllowedVerdicts {
			if !verdictSet[verdict] {
				t.Errorf("reason %s references unknown verdict %s", reason.Code, verdict)
			}
		}
	}
}

func schemaEnum(t *testing.T, schema map[string]any, definition string) []string {
	t.Helper()
	definitions := schema["$defs"].(map[string]any)
	values := definitions[definition].(map[string]any)["enum"].([]any)
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = value.(string)
	}
	return result
}

func registryVerdictNames(registry decisionRegistry) []string {
	names := make([]string, len(registry.Verdicts))
	for index, verdict := range registry.Verdicts {
		names[index] = verdict.Name
	}
	return names
}

func registryProofLevelNames(registry decisionRegistry) []string {
	names := make([]string, len(registry.ProofLevels))
	for index, level := range registry.ProofLevels {
		names[index] = level.Name
	}
	return names
}

func requireUniqueNamedValue(t *testing.T, seen map[string]bool, name, description string) {
	t.Helper()
	if strings.TrimSpace(name) == "" || strings.TrimSpace(description) == "" {
		t.Errorf("registry entry has empty name or description: %q", name)
	}
	if seen[name] {
		t.Errorf("duplicate registry value %q", name)
	}
	seen[name] = true
}

func mustMarshalJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	return encoded
}
