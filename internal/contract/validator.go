package contract

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"sync"

	registryassets "github.com/fantasyce/agent-runtime-proof/contracts"
	publicschemas "github.com/fantasyce/agent-runtime-proof/schemas"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const schemaBaseURL = "https://agent-runtime-proof.dev/schemas/"

var (
	compileOnce sync.Once
	schemaSet   map[string]*jsonschema.Schema
	decisions   decisionRegistry
	compileErr  error
)

type decisionRegistry struct {
	DefaultMinimumProofLevel string            `json:"default_minimum_proof_level"`
	MinimumProofOverrides    map[string]string `json:"minimum_proof_level_overrides"`
	ProofLevels              []struct {
		Name  string `json:"name"`
		Order int    `json:"order"`
	} `json:"proof_levels"`
	ReasonCodes []struct {
		Code            string   `json:"code"`
		AllowedVerdicts []string `json:"allowed_verdicts"`
	} `json:"reason_codes"`
}

func ValidateExpectation(document []byte) error {
	return validate("agent-runtime-expectation-1.0.schema.json", document)
}

func ValidateLaunchReceipt(document []byte) error {
	value, err := validateValue("agent-runtime-launch-receipt-1.0.schema.json", document)
	if err != nil {
		return err
	}
	if err := validateLaunchReceiptSemantics(value.(map[string]any)); err != nil {
		return fmt.Errorf("validate launch receipt semantics: %w", err)
	}
	return nil
}

func ValidateProof(document []byte) error {
	value, err := validateValue("agent-runtime-proof-1.0.schema.json", document)
	if err != nil {
		return err
	}
	if err := validateProofSemantics(value.(map[string]any)); err != nil {
		return fmt.Errorf("validate proof semantics: %w", err)
	}
	return nil
}

func validate(name string, document []byte) error {
	_, err := validateValue(name, document)
	return err
}

func validateValue(name string, document []byte) (any, error) {
	compileOnce.Do(compileSchemas)
	if compileErr != nil {
		return nil, compileErr
	}
	value, err := decodeOne(document)
	if err != nil {
		return nil, err
	}
	if err := schemaSet[name].Validate(value); err != nil {
		return nil, fmt.Errorf("validate %s: %w", name, err)
	}
	return value, nil
}

func compileSchemas() {
	compiler := jsonschema.NewCompiler()
	names := []string{
		"agent-runtime-expectation-1.0.schema.json",
		"agent-runtime-launch-receipt-1.0.schema.json",
		"agent-runtime-proof-1.0.schema.json",
		"agent-runtime-fixture-1.0.schema.json",
	}
	for _, name := range names {
		contents, err := publicschemas.Files.ReadFile(name)
		if err != nil {
			compileErr = fmt.Errorf("read embedded schema %s: %w", name, err)
			return
		}
		value, err := decodeOne(contents)
		if err != nil {
			compileErr = fmt.Errorf("decode embedded schema %s: %w", name, err)
			return
		}
		if err := compiler.AddResource(schemaBaseURL+path.Base(name), value); err != nil {
			compileErr = fmt.Errorf("add embedded schema %s: %w", name, err)
			return
		}
	}
	schemaSet = map[string]*jsonschema.Schema{}
	for _, name := range names {
		compiled, err := compiler.Compile(schemaBaseURL + name)
		if err != nil {
			compileErr = fmt.Errorf("compile embedded schema %s: %w", name, err)
			return
		}
		schemaSet[name] = compiled
	}
	registryJSON, err := registryassets.Files.ReadFile("decision-registry.json")
	if err != nil {
		compileErr = fmt.Errorf("read embedded decision registry: %w", err)
		return
	}
	if err := json.Unmarshal(registryJSON, &decisions); err != nil {
		compileErr = fmt.Errorf("decode embedded decision registry: %w", err)
	}
}

func validateLaunchReceiptSemantics(value map[string]any) error {
	observationOnly := value["observation_only"].(bool)
	reasons := stringArray(value["reason_codes"])
	if observationOnly {
		if value["subject"] != nil || value["expectation"] != nil || value["artifact"] != nil || !contains(reasons, "WITNESS_EXPECTATION_MISSING") {
			return errors.New("observation-only receipt requires no expectation evidence and WITNESS_EXPECTATION_MISSING")
		}
		return nil
	}
	if value["subject"] == nil || value["expectation"] == nil || value["artifact"] == nil || len(reasons) != 0 {
		return errors.New("expectation-bound receipt requires subject, expectation, artifact, and no reason codes")
	}
	return nil
}

func decodeOne(document []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()
	value, err := decodeValue(decoder)
	if err != nil {
		return nil, fmt.Errorf("decode JSON: %w", err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("JSON contains trailing data")
	}
	return value, nil
}

func decodeValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return token, nil
	}
	switch delimiter {
	case '{':
		object := map[string]any{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, errors.New("JSON object key is not a string")
			}
			if _, exists := object[key]; exists {
				return nil, fmt.Errorf("duplicate JSON object key %q", key)
			}
			value, err := decodeValue(decoder)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return nil, errors.New("unterminated JSON object")
		}
		return object, nil
	case '[':
		array := []any{}
		for decoder.More() {
			value, err := decodeValue(decoder)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return nil, errors.New("unterminated JSON array")
		}
		return array, nil
	default:
		return nil, fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}
}

func validateProofSemantics(proof map[string]any) error {
	verdict := proof["verdict"].(string)
	proofLevel := proof["proof_level"].(string)
	observation := proof["observation"].(map[string]any)
	reasonCodes := stringArray(proof["reason_codes"])
	limitations := stringArray(proof["limitations"])

	if proof["expectation"] == nil {
		if verdict != "UNKNOWN" || !contains(reasonCodes, "EXPECTATION_MISSING") {
			return errors.New("missing expectation requires UNKNOWN/EXPECTATION_MISSING")
		}
	}
	if verdict == "MATCHED" || verdict == "STALE" || verdict == "LEAKED" {
		if observation["process"] == nil || observation["executable"] == nil {
			return fmt.Errorf("verdict %s requires process and executable observations", verdict)
		}
	}
	if verdict == "MATCHED" && observation["artifact"] == nil {
		return errors.New("MATCHED requires an artifact observation")
	}
	if verdict == "NOT_RUNNING" && (observation["process"] != nil || observation["executable"] != nil || observation["artifact"] != nil) {
		return errors.New("NOT_RUNNING cannot contain process, executable, or artifact observations")
	}

	proofOrder := map[string]int{}
	for _, level := range decisions.ProofLevels {
		proofOrder[level.Name] = level.Order
	}
	type reasonRule struct {
		allowed map[string]bool
		minimum int
	}
	rules := map[string]reasonRule{}
	for _, reason := range decisions.ReasonCodes {
		minimum := decisions.DefaultMinimumProofLevel
		if override := decisions.MinimumProofOverrides[reason.Code]; override != "" {
			minimum = override
		}
		allowed := map[string]bool{}
		for _, allowedVerdict := range reason.AllowedVerdicts {
			allowed[allowedVerdict] = true
		}
		rules[reason.Code] = reasonRule{allowed: allowed, minimum: proofOrder[minimum]}
	}
	for _, code := range append(reasonCodes, limitations...) {
		rule, ok := rules[code]
		if !ok {
			return fmt.Errorf("unknown reason code %s", code)
		}
		if !rule.allowed[verdict] {
			return fmt.Errorf("reason code %s is not allowed for verdict %s", code, verdict)
		}
		if proofOrder[proofLevel] < rule.minimum {
			return fmt.Errorf("reason code %s requires a stronger proof level", code)
		}
	}
	return nil
}

func stringArray(value any) []string {
	raw := value.([]any)
	result := make([]string, len(raw))
	for index, item := range raw {
		result[index] = item.(string)
	}
	return result
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
