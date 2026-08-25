package hostprofile

import (
	"bytes"
	"errors"
	"io"

	"gopkg.in/yaml.v3"
)

func decodeStrictYAML(document []byte) (any, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(document))
	var root yaml.Node
	if err := decoder.Decode(&root); err != nil {
		return nil, err
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errInvalidHostConfig
	}
	if len(root.Content) != 1 {
		return nil, errInvalidHostConfig
	}
	count := 0
	return decodeYAMLNode(root.Content[0], 0, &count)
}

func decodeYAMLNode(node *yaml.Node, depth int, count *int) (any, error) {
	if node == nil || depth > maximumConfigDepth || *count >= maximumConfigValues || node.Anchor != "" || node.Kind == yaml.AliasNode {
		return nil, errInvalidHostConfig
	}
	*count++
	switch node.Kind {
	case yaml.MappingNode:
		if node.Tag != "!!map" && node.Tag != "tag:yaml.org,2002:map" {
			return nil, errInvalidHostConfig
		}
		if len(node.Content)%2 != 0 {
			return nil, errInvalidHostConfig
		}
		result := map[string]any{}
		for index := 0; index < len(node.Content); index += 2 {
			keyNode := node.Content[index]
			if keyNode.Kind != yaml.ScalarNode || (keyNode.Tag != "!!str" && keyNode.Tag != "tag:yaml.org,2002:str") || keyNode.Value == "<<" || len(keyNode.Value) > maximumScalarBytes {
				return nil, errInvalidHostConfig
			}
			if _, exists := result[keyNode.Value]; exists {
				return nil, errInvalidHostConfig
			}
			value, err := decodeYAMLNode(node.Content[index+1], depth+1, count)
			if err != nil {
				return nil, err
			}
			result[keyNode.Value] = value
		}
		return result, nil
	case yaml.SequenceNode:
		if node.Tag != "!!seq" && node.Tag != "tag:yaml.org,2002:seq" {
			return nil, errInvalidHostConfig
		}
		result := make([]any, len(node.Content))
		for index, child := range node.Content {
			value, err := decodeYAMLNode(child, depth+1, count)
			if err != nil {
				return nil, err
			}
			result[index] = value
		}
		return result, nil
	case yaml.ScalarNode:
		if len(node.Value) > maximumScalarBytes {
			return nil, errInvalidHostConfig
		}
		switch node.Tag {
		case "!!str", "tag:yaml.org,2002:str":
			return node.Value, nil
		case "!!bool", "tag:yaml.org,2002:bool":
			return node.Value == "true", nil
		case "!!null", "tag:yaml.org,2002:null":
			return nil, nil
		default:
			return nil, errInvalidHostConfig
		}
	default:
		return nil, errInvalidHostConfig
	}
}
