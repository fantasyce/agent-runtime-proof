package contracttest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

type canonicalVectorSet struct {
	SchemaVersion string `json:"schema_version"`
	Vectors       []struct {
		Name      string `json:"name"`
		Canonical string `json:"canonical"`
		SHA256    string `json:"sha256"`
	} `json:"vectors"`
}

func TestCanonicalVectors(t *testing.T) {
	for _, path := range []string{
		"testdata/canonical/proof-vectors.json",
		"testdata/canonical/artifact-tree-vectors.json",
	} {
		t.Run(path, func(t *testing.T) {
			vectors := decodeViaJSON[canonicalVectorSet](t, loadJSON(t, path))
			if vectors.SchemaVersion != "agent-runtime-canonical-vectors/1.0" {
				t.Fatalf("schema_version = %q", vectors.SchemaVersion)
			}
			if len(vectors.Vectors) == 0 {
				t.Fatal("vector set is empty")
			}
			seen := map[string]bool{}
			for _, vector := range vectors.Vectors {
				if strings.TrimSpace(vector.Name) == "" || seen[vector.Name] {
					t.Errorf("invalid or duplicate vector name %q", vector.Name)
				}
				seen[vector.Name] = true
				if strings.TrimSpace(vector.Canonical) != vector.Canonical || strings.ContainsAny(vector.Canonical, "\r\n\t") {
					t.Errorf("vector %s contains non-canonical surrounding whitespace", vector.Name)
				}
				requireSingleJSONValue(t, vector.Name, vector.Canonical)
				digest := sha256.Sum256([]byte(vector.Canonical))
				if got := hex.EncodeToString(digest[:]); got != vector.SHA256 {
					t.Errorf("vector %s sha256 = %s, want %s", vector.Name, got, vector.SHA256)
				}
			}
		})
	}
}

func requireSingleJSONValue(t *testing.T, name, document string) {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(document))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Errorf("vector %s canonical JSON is invalid: %v", name, err)
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		t.Errorf("vector %s has trailing JSON data", name)
	}
}
