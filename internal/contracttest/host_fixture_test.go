package contracttest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func compileFixtureSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	for _, path := range []string{
		"schemas/agent-runtime-expectation-1.0.schema.json",
		"schemas/agent-runtime-proof-1.0.schema.json",
		"schemas/agent-runtime-fixture-1.0.schema.json",
	} {
		document := loadJSON(t, path)
		location := schemaBaseURL + filepath.Base(path)
		if err := compiler.AddResource(location, document); err != nil {
			t.Fatalf("add schema %s: %v", path, err)
		}
	}
	schema, err := compiler.Compile(schemaBaseURL + "agent-runtime-fixture-1.0.schema.json")
	if err != nil {
		t.Fatalf("compile fixture schema: %v", err)
	}
	return schema
}

func TestHostFixtures(t *testing.T) {
	schema := compileFixtureSchema(t)
	for _, platform := range []string{"darwin", "windows", "linux"} {
		pattern := filepath.Join(repoRoot(t), "testdata", "hosts", platform, "*.json")
		paths, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}
		if len(paths) == 0 {
			t.Fatalf("no valid %s host fixtures", platform)
		}
		for _, absolutePath := range paths {
			relativePath := filepath.ToSlash(strings.TrimPrefix(absolutePath, repoRoot(t)+string(filepath.Separator)))
			if err := schema.Validate(loadJSON(t, relativePath)); err != nil {
				t.Errorf("valid fixture %s rejected: %v", filepath.Base(absolutePath), err)
			}
		}
	}

	invalidPattern := filepath.Join(repoRoot(t), "testdata", "hosts", "invalid", "*.json")
	invalidPaths, err := filepath.Glob(invalidPattern)
	if err != nil {
		t.Fatalf("glob %s: %v", invalidPattern, err)
	}
	if len(invalidPaths) == 0 {
		t.Fatal("no invalid host fixtures")
	}
	for _, absolutePath := range invalidPaths {
		relativePath := filepath.ToSlash(strings.TrimPrefix(absolutePath, repoRoot(t)+string(filepath.Separator)))
		if err := schema.Validate(loadJSON(t, relativePath)); err == nil {
			t.Errorf("invalid fixture %s accepted", filepath.Base(absolutePath))
		}
	}
}
