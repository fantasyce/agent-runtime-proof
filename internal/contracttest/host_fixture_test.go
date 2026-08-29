package contracttest

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fantasyce/agent-runtime-proof/internal/evaluator"
	"github.com/fantasyce/agent-runtime-proof/internal/expectation"
	"github.com/fantasyce/agent-runtime-proof/internal/model"
	"github.com/fantasyce/agent-runtime-proof/internal/proof"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

type hostFixture struct {
	Platform model.Platform `json:"platform"`
	Snapshot struct {
		Process    model.ProcessIdentity `json:"process"`
		Executable struct {
			Basename       string `json:"basename"`
			PathProjection string `json:"path_projection"`
			PathHash       string `json:"path_hash"`
			FileIDHash     string `json:"file_id_hash"`
		} `json:"executable"`
		Artifact struct {
			SHA256    *string `json:"sha256"`
			FileCount int     `json:"file_count"`
			ByteCount int64   `json:"byte_count"`
		} `json:"artifact"`
		DeniedFields []string `json:"denied_fields"`
	} `json:"snapshot"`
	Expectation model.Expectation `json:"expectation"`
	Expected    struct {
		Verdict     string   `json:"verdict"`
		ProofLevel  string   `json:"proof_level"`
		ReasonCodes []string `json:"reason_codes"`
	} `json:"expected"`
}

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

func TestInterpreterRuntimeFixtureDecision(t *testing.T) {
	document := loadJSON(t, "testdata/hosts/linux/interpreter-runtime.json")
	fixture := decodeViaJSON[hostFixture](t, document)
	if fixture.Snapshot.Artifact.SHA256 == nil {
		t.Fatal("interpreter fixture must include observed artifact bytes")
	}

	root := filepath.Clean(filepath.FromSlash(fixture.Expectation.Artifact.Root))
	resolved := &expectation.Resolved{
		Value:        fixture.Expectation,
		ArtifactRoot: root,
		AllowedRoots: []string{root},
	}
	candidate := &model.Candidate{
		Platform: fixture.Platform,
		Process:  fixture.Snapshot.Process,
		Executable: model.ExecutableObservation{
			Basename:   fixture.Snapshot.Executable.Basename,
			PathHash:   fixture.Snapshot.Executable.PathHash,
			FileIDHash: fixture.Snapshot.Executable.FileIDHash,
		},
		DeclaredExecutablePath: filepath.Clean(filepath.FromSlash(fixture.Snapshot.Executable.PathProjection)),
		ExecutableFileIdentity: fixture.Snapshot.Executable.FileIDHash,
		Inaccessible:           fixture.Snapshot.DeniedFields,
	}
	artifact := &model.ArtifactObservation{
		SHA256:     *fixture.Snapshot.Artifact.SHA256,
		FileCount:  fixture.Snapshot.Artifact.FileCount,
		ByteCount:  fixture.Snapshot.Artifact.ByteCount,
		DurationMS: 1,
	}

	decision := evaluator.Evaluate(evaluator.Input{Candidate: candidate, Expectation: resolved, Artifact: artifact})
	if decision.Verdict == "MATCHED" || decision.Verdict == "STALE" {
		t.Fatalf("interpreter fixture decision was over-promoted: %#v", decision)
	}
	if decision.Verdict != fixture.Expected.Verdict || decision.ProofLevel != fixture.Expected.ProofLevel || !slices.Equal(decision.ReasonCodes, fixture.Expected.ReasonCodes) {
		t.Fatalf("decision = %#v, expected %#v", decision, fixture.Expected)
	}
	if !slices.Contains(decision.Limitations, "DYNAMIC_DEPENDENCIES_UNPROVEN") {
		t.Fatalf("limitations = %v", decision.Limitations)
	}

	value, err := proof.Build(proof.Input{
		Candidate: candidate, Expectation: resolved, Artifact: artifact, Decision: decision,
		Tool:       model.ToolInfo{Name: "agent-runtime-proof", Version: "0.1.0", Commit: "abcdef0", Toolchain: "go1.27.0"},
		ObservedAt: time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if value.ProofLevel != fixture.Expected.ProofLevel || !slices.Equal(value.ReasonCodes, fixture.Expected.ReasonCodes) {
		t.Fatalf("proof = %#v", value)
	}
}
