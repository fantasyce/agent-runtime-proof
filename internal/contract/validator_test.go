package contract

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestValidateProofRejectsMatchedWithoutExpectation(t *testing.T) {
	document := readFixture(t, "../../testdata/contracts/invalid-semantic/proof-matched-without-expectation.json")
	if err := ValidateProof(document); err == nil {
		t.Fatal("semantically contradictory proof accepted")
	}
}

func TestValidateProofRejectsNonCanonicalExtensionNumber(t *testing.T) {
	document := readFixture(t, "../../testdata/contracts/valid/proof-observation-only.json")
	var value map[string]any
	if err := json.Unmarshal(document, &value); err != nil {
		t.Fatal(err)
	}
	value["extensions"] = map[string]any{"example.value": 1.5}
	document, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateProof(document); err == nil {
		t.Fatal("non-canonical extension float accepted")
	}
}

func TestValidateProofAcceptsObservationOnlyUnknown(t *testing.T) {
	document := readFixture(t, "../../testdata/contracts/valid/proof-observation-only.json")
	if err := ValidateProof(document); err != nil {
		t.Fatal(err)
	}
}

func TestValidateExpectationRejectsDuplicateObjectKey(t *testing.T) {
	document := readFixture(t, "../../testdata/contracts/valid/expectation-native.json")
	document = bytes.Replace(document, []byte(`"schema_version": "agent-runtime-expectation/1.0",`), []byte(`"schema_version": "agent-runtime-expectation/1.0", "schema_version": "agent-runtime-expectation/1.0",`), 1)
	if err := ValidateExpectation(document); err == nil {
		t.Fatal("duplicate object key accepted")
	}
}

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	document, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return document
}
