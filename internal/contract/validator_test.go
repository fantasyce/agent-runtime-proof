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

func TestValidateLaunchReceiptContract(t *testing.T) {
	valid := []byte(`{
		"schema_version":"agent-runtime-launch-receipt/1.0",
		"receipt_id":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"created_at":"2026-08-25T12:34:56Z",
		"tool":{"name":"agent-runtime-proof","version":"0.1.0","commit":"abcdef0","toolchain":"go1.26.3"},
		"platform":{"os":"darwin","arch":"arm64"},
		"subject":null,
		"process":{"pid":42,"created_at_unix_nano":"1787536210123456789","boot_id_hash":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
		"command":{"executable_basename":"helper","executable_path_hash":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","argument_fingerprints":[{"position":1,"sha256":"sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"}]},
		"expectation":null,
		"artifact":null,
		"observation_only":true,
		"reason_codes":["WITNESS_EXPECTATION_MISSING"],
		"privacy":{"redaction_mode":"safe-default","home_redacted":true,"omitted_fields":["command.argv","process.environment"]}
	}`)
	if err := ValidateLaunchReceipt(valid); err != nil {
		t.Fatalf("valid receipt rejected: %v", err)
	}
	for _, forbidden := range []string{"argv", "environment", "executable_path"} {
		var value map[string]any
		if err := json.Unmarshal(valid, &value); err != nil {
			t.Fatal(err)
		}
		value[forbidden] = "private-value"
		document, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := ValidateLaunchReceipt(document); err == nil {
			t.Fatalf("receipt accepted forbidden top-level field %q", forbidden)
		}
	}
}

func TestValidateLaunchReceiptRequiresObservationOnlyReason(t *testing.T) {
	document := []byte(`{
		"schema_version":"agent-runtime-launch-receipt/1.0",
		"receipt_id":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"created_at":"2026-08-25T12:34:56Z",
		"tool":{"name":"agent-runtime-proof","version":"0.1.0","commit":"abcdef0","toolchain":"go1.26.3"},
		"platform":{"os":"linux","arch":"amd64"},
		"subject":null,
		"process":{"pid":42,"created_at_unix_nano":"1787536210123456789","boot_id_hash":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
		"command":{"executable_basename":"helper","executable_path_hash":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","argument_fingerprints":[]},
		"expectation":null,
		"artifact":null,
		"observation_only":true,
		"reason_codes":[],
		"privacy":{"redaction_mode":"safe-default","home_redacted":true,"omitted_fields":[]}
	}`)
	if err := ValidateLaunchReceipt(document); err == nil {
		t.Fatal("observation-only receipt without WITNESS_EXPECTATION_MISSING accepted")
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
