package expectation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasyce/agent-runtime-proof/internal/model"
)

func TestLoadResolvesRelativeRootInsideAllowedRoot(t *testing.T) {
	directory := t.TempDir()
	root := filepath.Join(directory, "payload")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	path := writeExpectation(t, directory, validExpectation("payload", []string{"."}))

	resolved, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	wantRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ArtifactRoot != wantRoot {
		t.Fatalf("artifact root = %q, want %q", resolved.ArtifactRoot, wantRoot)
	}
	wantManifest, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ManifestPath != wantManifest {
		t.Fatalf("manifest path = %q", resolved.ManifestPath)
	}
}

func TestResolveInlineRequiresUnambiguousAbsoluteRoots(t *testing.T) {
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	value := model.Expectation{
		SchemaVersion: "agent-runtime-expectation/1.0",
		Subject:       model.Subject{ID: "inline", DisplayName: "Inline", Version: "1.0.0"},
		Launch:        model.LaunchExpectation{Kind: "native", Entrypoint: "runtime", ArgumentFingerprints: []model.ArgumentFingerprint{}},
		Artifact:      model.ArtifactExpectation{Root: root, Include: []string{"**"}, Exclude: []string{}, SHA256: strings.Repeat("a", 64), MaxFiles: 1, MaxBytes: 1, MaxDurationMS: 1},
		Policy:        model.ExpectationPolicy{AllowedRoots: []string{root}},
		Source:        model.ExpectationSource{Kind: "user-file", LocatorHash: strings.Repeat("0", 64), Trust: "declared"},
	}
	resolved, err := ResolveInline(value)
	if err != nil || resolved.ArtifactRoot != root || resolved.ManifestPath != "" {
		t.Fatalf("resolved=%#v err=%v", resolved, err)
	}
	value.Artifact.Root = "relative"
	if _, err := ResolveInline(value); err == nil {
		t.Fatal("relative inline root was accepted")
	}
}

func TestLoadAcceptsAbsoluteRootInsideAllowedRoot(t *testing.T) {
	directory := t.TempDir()
	root := filepath.Join(directory, "payload")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	resolvedDirectory, err := filepath.EvalSymlinks(directory)
	if err != nil {
		t.Fatal(err)
	}
	path := writeExpectation(t, directory, validExpectation(resolvedRoot, []string{resolvedDirectory}))
	if _, err := Load(path); err != nil {
		t.Fatal(err)
	}
}

func TestLoadRejectsMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("missing expectation accepted")
	}
}

func TestLoadRejectsArtifactRootOutsideAllowedRoots(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"allowed", "outside"} {
		if err := os.Mkdir(filepath.Join(directory, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	path := writeExpectation(t, directory, validExpectation("outside", []string{"allowed"}))
	if _, err := Load(path); err == nil {
		t.Fatal("root outside allowed roots accepted")
	}
}

func TestLoadRejectsSymlinkRootWhenPolicyForbidsLinks(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(directory, "payload")); err != nil {
		t.Fatal(err)
	}
	value := validExpectation("payload", []string{"."})
	value.Policy.AllowSymlinks = false
	path := writeExpectation(t, directory, value)
	if _, err := Load(path); err == nil {
		t.Fatal("symlink root accepted")
	}
}

func TestLoadRejectsDuplicateArgumentPositions(t *testing.T) {
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "payload"), 0o700); err != nil {
		t.Fatal(err)
	}
	value := validExpectation("payload", []string{"."})
	value.Launch.ArgumentFingerprints = []model.ArgumentFingerprint{
		{Position: 1, SHA256: repeatedHex('1')},
		{Position: 1, SHA256: repeatedHex('2')},
	}
	path := writeExpectation(t, directory, value)
	if _, err := Load(path); err == nil {
		t.Fatal("duplicate argument position accepted")
	}
}

func TestLoadRejectsEntrypointEscape(t *testing.T) {
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "payload"), 0o700); err != nil {
		t.Fatal(err)
	}
	value := validExpectation("payload", []string{"."})
	value.Launch.Entrypoint = "../outside"
	path := writeExpectation(t, directory, value)
	if _, err := Load(path); err == nil {
		t.Fatal("entrypoint escape accepted")
	}
}

func TestLoadRejectsPatternEscape(t *testing.T) {
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "payload"), 0o700); err != nil {
		t.Fatal(err)
	}
	value := validExpectation("payload", []string{"."})
	value.Artifact.Include = []string{"../secret"}
	path := writeExpectation(t, directory, value)
	if _, err := Load(path); err == nil {
		t.Fatal("include escape accepted")
	}
}

func TestLoadRejectsUnknownField(t *testing.T) {
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "payload"), 0o700); err != nil {
		t.Fatal(err)
	}
	value := validExpectation("payload", []string{"."})
	document, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	document[len(document)-1] = ','
	document = append(document, []byte(`"unsafe":true}`)...)
	path := filepath.Join(directory, "expectation.json")
	if err := os.WriteFile(path, document, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("unknown field accepted")
	}
}

func TestLoadRejectsMalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "expectation.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("malformed JSON accepted")
	}
}

func validExpectation(root string, allowed []string) model.Expectation {
	return model.Expectation{
		SchemaVersion: "agent-runtime-expectation/1.0",
		Subject:       model.Subject{ID: "example", DisplayName: "Example", Version: "1.0.0"},
		Launch: model.LaunchExpectation{
			Kind: "native", Entrypoint: "bin/example", ArgumentFingerprints: []model.ArgumentFingerprint{},
		},
		Artifact: model.ArtifactExpectation{
			Root: root, Include: []string{"**"}, Exclude: []string{}, SHA256: repeatedHex('a'),
			MaxFiles: 20, MaxBytes: 1024, MaxDurationMS: 1000,
		},
		Policy: model.ExpectationPolicy{AllowedRoots: allowed, AllowSymlinks: false},
		Source: model.ExpectationSource{Kind: "user-file", LocatorHash: repeatedHex('b'), Trust: "declared"},
	}
}

func writeExpectation(t *testing.T, directory string, value model.Expectation) string {
	t.Helper()
	document, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "expectation.json")
	if err := os.WriteFile(path, document, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func repeatedHex(value byte) string {
	result := make([]byte, 64)
	for index := range result {
		result[index] = value
	}
	return string(result)
}
