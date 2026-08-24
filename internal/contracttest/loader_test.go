package contracttest

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve contract test source path")
	}
	for current := filepath.Dir(filename); ; current = filepath.Dir(current) {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			t.Fatal("locate repository root containing go.mod")
		}
	}
}

func loadJSON(t *testing.T, path string) any {
	t.Helper()
	file, err := os.Open(filepath.Join(repoRoot(t), filepath.FromSlash(path)))
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.UseNumber()
	var document any
	if err := decoder.Decode(&document); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	var trailing any
	err = decoder.Decode(&trailing)
	if !errors.Is(err, io.EOF) {
		if err == nil {
			t.Fatalf("decode %s: trailing JSON value", path)
		}
		t.Fatalf("decode %s trailing data: %v", path, err)
	}
	return document
}

func TestLoadJSON(t *testing.T) {
	document := loadJSON(t, "internal/contracttest/testdata/valid.json")
	object, ok := document.(map[string]any)
	if !ok {
		t.Fatalf("decoded JSON type = %T, want map[string]any", document)
	}
	if ready, ok := object["ready"].(bool); !ok || !ready {
		t.Fatalf("ready = %#v, want true", object["ready"])
	}
}
