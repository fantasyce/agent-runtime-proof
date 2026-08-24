package privacy

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasyce/agent-runtime-proof/internal/expectation"
	"github.com/fantasyce/agent-runtime-proof/internal/model"
)

func TestProjectOmitsAllLocalPathsAndCommandData(t *testing.T) {
	secretRoot := filepath.Join(string(filepath.Separator), "Users", "private", "token-secret")
	resolved := &expectation.Resolved{ManifestPath: filepath.Join(secretRoot, "expectation.json"), ArtifactRoot: secretRoot, AllowedRoots: []string{secretRoot}}
	resolved.Value.Subject = model.Subject{ID: "example", DisplayName: "Example", Version: "1.0.0"}
	resolved.Value.Source = model.ExpectationSource{Kind: "user-file", LocatorHash: strings.Repeat("a", 64), Trust: "declared"}
	resolved.Value.Artifact.SHA256 = strings.Repeat("b", 64)
	candidate := &model.Candidate{
		Platform:       model.Platform{OS: "darwin", Arch: "arm64"},
		Process:        model.ProcessIdentity{PID: 42, CreatedAtUnixNano: "100", BootIDHash: "sha256:" + strings.Repeat("c", 64)},
		Executable:     model.ExecutableObservation{Basename: "runtime", PathHash: "sha256:" + strings.Repeat("d", 64), FileIDHash: "sha256:" + strings.Repeat("e", 64)},
		ExecutablePath: filepath.Join(secretRoot, "cookie-secret"), DeclaredExecutablePath: filepath.Join(secretRoot, "private-key-secret"),
	}
	projection, err := Project(candidate, resolved, &model.ArtifactObservation{SHA256: strings.Repeat("b", 64), FileCount: 1, ByteCount: 10, DurationMS: 1}, "2026-08-24T12:34:56Z")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	for _, prohibited := range []string{"/Users/private", "token-secret", "cookie-secret", "private-key-secret"} {
		if strings.Contains(string(encoded), prohibited) {
			t.Fatalf("projection leaked %q: %s", prohibited, encoded)
		}
	}
}

func TestProjectTruncatesUnicodeDisplayNameWithoutBreakingUTF8(t *testing.T) {
	candidate := &model.Candidate{
		Platform:   model.Platform{OS: "darwin", Arch: "arm64"},
		Executable: model.ExecutableObservation{Basename: strings.Repeat("验", 200)},
	}
	projection, err := Project(candidate, nil, nil, "2026-08-24T12:34:56Z")
	if err != nil {
		t.Fatal(err)
	}
	if got := len([]rune(projection.Subject.DisplayName)); got != 128 {
		t.Fatalf("display-name rune count = %d", got)
	}
	if !json.Valid(mustJSON(t, projection)) {
		t.Fatal("projection is not valid UTF-8 JSON")
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
