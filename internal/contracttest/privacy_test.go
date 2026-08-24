package contracttest

import (
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

type privacyRegistry struct {
	SchemaVersion string `json:"schema_version"`
	Rules         []struct {
		FieldPattern string   `json:"field_pattern"`
		Class        string   `json:"class"`
		Projection   string   `json:"projection"`
		ThreatIDs    []string `json:"threat_ids"`
	} `json:"rules"`
}

type threatRegistry struct {
	SchemaVersion string `json:"schema_version"`
	Threats       []struct {
		ID            string `json:"id"`
		Title         string `json:"title"`
		Precondition  string `json:"precondition"`
		Impact        string `json:"impact"`
		Prevention    string `json:"prevention"`
		Detection     string `json:"detection"`
		ResidualRisk  string `json:"residual_risk"`
		RequiredPhase string `json:"required_phase"`
	} `json:"threats"`
}

func TestPrivacyClassificationIsExhaustive(t *testing.T) {
	documents := loadContractSchemas(t)
	wantFields := []string{}
	for _, root := range []struct {
		name string
		id   string
	}{
		{"expectation", schemaBaseURL + "agent-runtime-expectation-1.0.schema.json"},
		{"proof", schemaBaseURL + "agent-runtime-proof-1.0.schema.json"},
		{"fixture", schemaBaseURL + "agent-runtime-fixture-1.0.schema.json"},
	} {
		collectSchemaFields(t, documents, root.id, documents[root.id], root.name, map[string]bool{}, &wantFields)
	}
	sort.Strings(wantFields)
	wantFields = slices.Compact(wantFields)

	registry := decodeViaJSON[privacyRegistry](t, loadJSON(t, "contracts/privacy-registry.json"))
	if registry.SchemaVersion != "agent-runtime-privacy-registry/1.0" {
		t.Fatalf("schema_version = %q", registry.SchemaVersion)
	}
	type compiledRule struct {
		pattern *regexp.Regexp
		index   int
	}
	compiledRules := make([]compiledRule, 0, len(registry.Rules))
	seenPatterns := map[string]bool{}
	allowedClasses := map[string]bool{"PUBLIC": true, "SAFE_IDENTIFIER": true, "HASH_ONLY": true, "LOCAL_EXPLICIT_ONLY": true, "PROHIBITED": true}
	for index, rule := range registry.Rules {
		if seenPatterns[rule.FieldPattern] {
			t.Errorf("duplicate privacy rule %s", rule.FieldPattern)
		}
		seenPatterns[rule.FieldPattern] = true
		pattern, err := regexp.Compile(rule.FieldPattern)
		if err != nil {
			t.Errorf("invalid field pattern %q: %v", rule.FieldPattern, err)
			continue
		}
		compiledRules = append(compiledRules, compiledRule{pattern: pattern, index: index})
		if !allowedClasses[rule.Class] {
			t.Errorf("pattern %s has unknown class %s", rule.FieldPattern, rule.Class)
		}
		if strings.TrimSpace(rule.Projection) == "" || len(rule.ThreatIDs) == 0 {
			t.Errorf("pattern %s lacks projection or threat IDs", rule.FieldPattern)
		}
	}
	matchCounts := make([]int, len(registry.Rules))
	for _, field := range wantFields {
		matches := 0
		for _, rule := range compiledRules {
			if rule.pattern.MatchString(field) {
				matches++
				matchCounts[rule.index]++
			}
		}
		if matches != 1 {
			t.Errorf("field %s matches %d privacy rules, want exactly 1", field, matches)
		}
	}
	for index, count := range matchCounts {
		if count == 0 {
			t.Errorf("privacy pattern %s matches no schema fields", registry.Rules[index].FieldPattern)
		}
	}
}

func TestThreatRegistryResolvesPrivacyReferences(t *testing.T) {
	privacy := decodeViaJSON[privacyRegistry](t, loadJSON(t, "contracts/privacy-registry.json"))
	threats := decodeViaJSON[threatRegistry](t, loadJSON(t, "contracts/threat-registry.json"))
	if threats.SchemaVersion != "agent-runtime-threat-registry/1.0" {
		t.Fatalf("schema_version = %q", threats.SchemaVersion)
	}
	known := map[string]bool{}
	for _, threat := range threats.Threats {
		if known[threat.ID] {
			t.Errorf("duplicate threat ID %s", threat.ID)
		}
		known[threat.ID] = true
		for label, value := range map[string]string{
			"title": threat.Title, "precondition": threat.Precondition, "impact": threat.Impact,
			"prevention": threat.Prevention, "detection": threat.Detection,
			"residual_risk": threat.ResidualRisk, "required_phase": threat.RequiredPhase,
		} {
			if strings.TrimSpace(value) == "" {
				t.Errorf("threat %s has empty %s", threat.ID, label)
			}
		}
	}
	for _, rule := range privacy.Rules {
		for _, threatID := range rule.ThreatIDs {
			if !known[threatID] {
				t.Errorf("pattern %s references unknown threat %s", rule.FieldPattern, threatID)
			}
		}
	}
}

func TestPublicSchemasForbidSensitivePropertyNames(t *testing.T) {
	prohibited := regexp.MustCompile(`(?i)^(environment|env_value|argv|content|transcript|password|secret|token|cookie|private_key)$`)
	for id, schema := range loadContractSchemas(t) {
		walkPropertyNames(schema, func(name string) {
			if prohibited.MatchString(name) {
				t.Errorf("schema %s defines prohibited property %q", id, name)
			}
		})
	}
}

func loadContractSchemas(t *testing.T) map[string]map[string]any {
	t.Helper()
	documents := map[string]map[string]any{}
	for _, path := range []string{
		"schemas/agent-runtime-expectation-1.0.schema.json",
		"schemas/agent-runtime-proof-1.0.schema.json",
		"schemas/agent-runtime-fixture-1.0.schema.json",
	} {
		document := loadJSON(t, path).(map[string]any)
		documents[document["$id"].(string)] = document
	}
	return documents
}

func collectSchemaFields(t *testing.T, documents map[string]map[string]any, currentID string, node any, path string, refStack map[string]bool, fields *[]string) {
	t.Helper()
	object, ok := node.(map[string]any)
	if !ok {
		*fields = append(*fields, path)
		return
	}
	if reference, ok := object["$ref"].(string); ok {
		refID, target := resolveSchemaReference(t, documents, currentID, reference)
		stackKey := refID + reference + "@" + path
		if refStack[stackKey] {
			t.Fatalf("cyclic schema reference %s", reference)
		}
		refStack[stackKey] = true
		collectSchemaFields(t, documents, refID, target, path, refStack, fields)
		delete(refStack, stackKey)
		return
	}
	if alternatives, ok := object["oneOf"].([]any); ok {
		for _, alternative := range alternatives {
			alternativeObject, _ := alternative.(map[string]any)
			if alternativeObject["type"] == "null" {
				continue
			}
			collectSchemaFields(t, documents, currentID, alternative, path, refStack, fields)
		}
		return
	}
	if properties, ok := object["properties"].(map[string]any); ok {
		propertyNames := make([]string, 0, len(properties))
		for name := range properties {
			propertyNames = append(propertyNames, name)
		}
		sort.Strings(propertyNames)
		for _, name := range propertyNames {
			collectSchemaFields(t, documents, currentID, properties[name], path+"."+name, refStack, fields)
		}
		return
	}
	if items, ok := object["items"]; ok {
		collectSchemaFields(t, documents, currentID, items, path+"[]", refStack, fields)
		return
	}
	if object["type"] == "object" && object["additionalProperties"] == true {
		*fields = append(*fields, path+".*")
		return
	}
	*fields = append(*fields, path)
}

func resolveSchemaReference(t *testing.T, documents map[string]map[string]any, currentID, reference string) (string, any) {
	t.Helper()
	documentID := currentID
	fragment := ""
	if strings.HasPrefix(reference, "#") {
		fragment = strings.TrimPrefix(reference, "#")
	} else {
		parts := strings.SplitN(reference, "#", 2)
		documentID = parts[0]
		if len(parts) == 2 {
			fragment = parts[1]
		}
	}
	document, ok := documents[documentID]
	if !ok {
		t.Fatalf("unknown schema document %s", documentID)
	}
	var node any = document
	if fragment == "" {
		return documentID, node
	}
	for _, escapedToken := range strings.Split(strings.TrimPrefix(fragment, "/"), "/") {
		token := strings.ReplaceAll(strings.ReplaceAll(escapedToken, "~1", "/"), "~0", "~")
		object, ok := node.(map[string]any)
		if !ok {
			t.Fatalf("reference %s traverses non-object at %s", reference, token)
		}
		node, ok = object[token]
		if !ok {
			t.Fatalf("reference %s missing token %s", reference, token)
		}
	}
	return documentID, node
}

func walkPropertyNames(node any, visit func(string)) {
	switch value := node.(type) {
	case map[string]any:
		if properties, ok := value["properties"].(map[string]any); ok {
			for name := range properties {
				visit(name)
			}
		}
		for _, child := range value {
			walkPropertyNames(child, visit)
		}
	case []any:
		for _, child := range value {
			walkPropertyNames(child, visit)
		}
	}
}
