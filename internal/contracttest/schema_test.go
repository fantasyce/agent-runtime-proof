package contracttest

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const schemaBaseURL = "https://agent-runtime-proof.dev/schemas/"

type decisionRegistry struct {
	SchemaVersion            string            `json:"schema_version"`
	DefaultMinimumProofLevel string            `json:"default_minimum_proof_level"`
	MinimumProofOverrides    map[string]string `json:"minimum_proof_level_overrides"`
	Verdicts                 []struct {
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

func validateDecisionSemantics(proof map[string]any, registry decisionRegistry) error {
	verdict, _ := proof["verdict"].(string)
	proofLevel, _ := proof["proof_level"].(string)
	observation, _ := proof["observation"].(map[string]any)
	if verdict == "MATCHED" || verdict == "STALE" || verdict == "LEAKED" {
		if proof["expectation"] == nil {
			return fmt.Errorf("verdict %s requires an expectation", verdict)
		}
		if observation["process"] == nil || observation["executable"] == nil {
			return fmt.Errorf("verdict %s requires process and executable observations", verdict)
		}
	}
	if verdict == "NOT_RUNNING" && (observation["process"] != nil || observation["executable"] != nil || observation["artifact"] != nil) {
		return fmt.Errorf("NOT_RUNNING cannot contain a process, executable, or artifact observation")
	}
	proofOrder := map[string]int{}
	for _, level := range registry.ProofLevels {
		proofOrder[level.Name] = level.Order
	}
	reasons := map[string]struct {
		allowedVerdicts map[string]bool
		minimumOrder    int
	}{}
	for _, reason := range registry.ReasonCodes {
		minimumLevel := registry.DefaultMinimumProofLevel
		if override := registry.MinimumProofOverrides[reason.Code]; override != "" {
			minimumLevel = override
		}
		allowed := map[string]bool{}
		for _, allowedVerdict := range reason.AllowedVerdicts {
			allowed[allowedVerdict] = true
		}
		reasons[reason.Code] = struct {
			allowedVerdicts map[string]bool
			minimumOrder    int
		}{allowedVerdicts: allowed, minimumOrder: proofOrder[minimumLevel]}
	}
	for _, field := range []string{"reason_codes", "limitations"} {
		for _, rawReason := range proof[field].([]any) {
			reasonCode := rawReason.(string)
			rule, ok := reasons[reasonCode]
			if !ok {
				return fmt.Errorf("unknown %s code %s", field, reasonCode)
			}
			if !rule.allowedVerdicts[verdict] {
				return fmt.Errorf("%s code %s is not allowed for verdict %s", field, reasonCode, verdict)
			}
			if proofOrder[proofLevel] < rule.minimumOrder {
				return fmt.Errorf("%s code %s requires stronger proof level than %s", field, reasonCode, proofLevel)
			}
		}
	}
	return nil
}

func TestProofDecisionSemantics(t *testing.T) {
	registry := loadDecisionRegistry(t)
	schema := compileSchema(t, "schemas/agent-runtime-proof-1.0.schema.json")
	for _, fixtureClass := range []string{"valid", "invalid-semantic"} {
		pattern := filepath.Join(repoRoot(t), "testdata", "contracts", fixtureClass, "proof-*.json")
		paths, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}
		if len(paths) == 0 {
			t.Fatalf("no %s proof fixtures", fixtureClass)
		}
		for _, absolutePath := range paths {
			relativePath := filepath.ToSlash(strings.TrimPrefix(absolutePath, repoRoot(t)+string(filepath.Separator)))
			document := loadJSON(t, relativePath)
			if err := schema.Validate(document); err != nil {
				t.Fatalf("semantic fixture %s must be structurally valid: %v", filepath.Base(absolutePath), err)
			}
			err := validateDecisionSemantics(document.(map[string]any), registry)
			if fixtureClass == "valid" && err != nil {
				t.Errorf("valid proof %s failed decision semantics: %v", filepath.Base(absolutePath), err)
			}
			if fixtureClass == "invalid-semantic" && err == nil {
				t.Errorf("invalid semantic proof %s accepted", filepath.Base(absolutePath))
			}
		}
	}
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
	proofOrder := map[string]int{}
	for index, level := range registry.ProofLevels {
		requireUniqueNamedValue(t, proofSet, level.Name, level.Description)
		proofOrder[level.Name] = level.Order
		if level.Order != index+1 {
			t.Errorf("proof level %s order = %d, want %d", level.Name, level.Order, index+1)
		}
	}
	if proofOrder[registry.DefaultMinimumProofLevel] == 0 {
		t.Errorf("unknown default minimum proof level %s", registry.DefaultMinimumProofLevel)
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
	for reasonCode, proofLevel := range registry.MinimumProofOverrides {
		if !reasonSet[reasonCode] {
			t.Errorf("minimum proof override references unknown reason %s", reasonCode)
		}
		if proofOrder[proofLevel] == 0 {
			t.Errorf("minimum proof override for %s references unknown level %s", reasonCode, proofLevel)
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
