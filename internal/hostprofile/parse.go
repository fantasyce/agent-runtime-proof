package hostprofile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/tailscale/hujson"
)

const (
	maximumConfigDepth   = 128
	maximumConfigValues  = 4096
	maximumConfigServers = 128
	maximumConfigArgs    = 128
	maximumScalarBytes   = 8192
)

var errInvalidHostConfig = errors.New("invalid host configuration")

type RawBinding struct {
	HostID     string
	SourceID   string
	ServerName string
	Command    string
	Args       []string
}

func parseConfig(profile Profile, source ConfigSource, document []byte) ([]RawBinding, error) {
	if source.MaximumBytes <= 0 || int64(len(document)) > source.MaximumBytes || bytes.IndexByte(document, 0) >= 0 {
		return nil, errInvalidHostConfig
	}
	var root any
	var err error
	switch source.Format {
	case "json":
		root, err = decodeStrictJSON(document)
	case "jsonc":
		standard, standardErr := hujson.Standardize(document)
		if standardErr != nil {
			err = standardErr
		} else {
			root, err = decodeStrictJSON(standard)
		}
	case "toml":
		var value map[string]any
		err = toml.Unmarshal(document, &value)
		root = value
		if err == nil {
			err = validateDecodedTree(root, 0, new(int))
		}
	case "yaml":
		root, err = decodeStrictYAML(document)
	default:
		err = errInvalidHostConfig
	}
	if err != nil {
		return nil, fmt.Errorf("%w: parse data", errInvalidHostConfig)
	}
	bindings, err := extractBindings(profile.HostID, source, root)
	if err != nil {
		return nil, fmt.Errorf("%w: project bindings", errInvalidHostConfig)
	}
	return bindings, nil
}

func extractBindings(hostID string, source ConfigSource, root any) ([]RawBinding, error) {
	switch source.Dialect {
	case "mcp-servers":
		object, err := objectAt(root)
		if err != nil {
			return nil, err
		}
		return bindingsFromServerMap(hostID, source.SourceID, object["mcpServers"], false)
	case "vscode-servers":
		object, err := objectAt(root)
		if err != nil {
			return nil, err
		}
		return bindingsFromServerMap(hostID, source.SourceID, object["servers"], false)
	case "opencode-v2":
		object, err := objectAt(root)
		if err != nil {
			return nil, err
		}
		mcp, err := objectAt(object["mcp"])
		if err != nil {
			return nil, err
		}
		return bindingsFromServerMap(hostID, source.SourceID, mcp["servers"], true)
	case "codex-toml":
		object, err := objectAt(root)
		if err != nil {
			return nil, err
		}
		return bindingsFromServerMap(hostID, source.SourceID, object["mcp_servers"], false)
	case "dsh-cordis":
		return bindingsFromCordis(hostID, source.SourceID, root)
	case "generic-only":
		return nil, nil
	default:
		return nil, errInvalidHostConfig
	}
}

func bindingsFromServerMap(hostID, sourceID string, value any, commandArray bool) ([]RawBinding, error) {
	servers, err := objectAt(value)
	if err != nil {
		return nil, err
	}
	if len(servers) > maximumConfigServers {
		return nil, errInvalidHostConfig
	}
	result := make([]RawBinding, 0, len(servers))
	for serverName, raw := range servers {
		if err := validateSafeIdentifier(serverName); err != nil {
			return nil, err
		}
		server, err := objectAt(raw)
		if err != nil {
			return nil, err
		}
		if _, remote := server["url"]; remote {
			continue
		}
		if kind, ok := server["type"].(string); ok && kind != "stdio" && kind != "local" {
			continue
		}
		if err := validateSecretMaps(server); err != nil {
			return nil, err
		}
		var command string
		var args []string
		if commandArray {
			commandLine, err := stringSlice(server["command"])
			if err != nil || len(commandLine) == 0 {
				return nil, errInvalidHostConfig
			}
			command, args = commandLine[0], append([]string{}, commandLine[1:]...)
		} else {
			var ok bool
			command, ok = server["command"].(string)
			if !ok {
				return nil, errInvalidHostConfig
			}
			args, err = optionalStringSlice(server["args"])
			if err != nil {
				return nil, err
			}
		}
		if err := validateDirectCommand(command, args); err != nil {
			return nil, err
		}
		result = append(result, RawBinding{HostID: hostID, SourceID: sourceID, ServerName: serverName, Command: command, Args: append([]string{}, args...)})
	}
	return result, nil
}

func bindingsFromCordis(hostID, sourceID string, root any) ([]RawBinding, error) {
	rows, ok := root.([]any)
	if !ok {
		return nil, errInvalidHostConfig
	}
	result := []RawBinding{}
	for _, rawRow := range rows {
		row, err := objectAt(rawRow)
		if err != nil {
			return nil, err
		}
		insert, ok := row["insert"].([]any)
		if !ok {
			return nil, errInvalidHostConfig
		}
		for _, rawPlugin := range insert {
			plugin, err := objectAt(rawPlugin)
			if err != nil {
				return nil, err
			}
			if plugin["name"] != "@deepseek-ai/dsh-mcp-client" {
				continue
			}
			config, err := objectAt(plugin["config"])
			if err != nil || config["transport"] != "stdio" {
				continue
			}
			if err := validateSecretMaps(config); err != nil {
				return nil, err
			}
			serverName, nameOK := config["serverName"].(string)
			command, commandOK := config["command"].(string)
			args, argsErr := optionalStringSlice(config["args"])
			if !nameOK || !commandOK || argsErr != nil || validateSafeIdentifier(serverName) != nil || validateDirectCommand(command, args) != nil {
				return nil, errInvalidHostConfig
			}
			result = append(result, RawBinding{HostID: hostID, SourceID: sourceID, ServerName: serverName, Command: command, Args: append([]string{}, args...)})
			if len(result) > maximumConfigServers {
				return nil, errInvalidHostConfig
			}
		}
	}
	return result, nil
}

func validateDirectCommand(command string, args []string) error {
	if command == "" || len(command) > maximumScalarBytes || unsafeDynamic(command) || len(args) > maximumConfigArgs {
		return errInvalidHostConfig
	}
	base := strings.ToLower(path.Base(strings.ReplaceAll(command, "\\", "/")))
	switch strings.TrimSuffix(base, ".exe") {
	case "sh", "bash", "zsh", "fish", "cmd", "powershell", "pwsh":
		return errInvalidHostConfig
	}
	for _, argument := range args {
		if argument == "" || len(argument) > maximumScalarBytes || unsafeDynamic(argument) {
			return errInvalidHostConfig
		}
	}
	return nil
}

func unsafeDynamic(value string) bool {
	return strings.ContainsRune(value, 0) || strings.Contains(value, "${") || strings.Contains(value, "$(") || strings.Contains(value, "`")
}

func validateSecretMaps(server map[string]any) error {
	for _, name := range []string{"env", "environment", "headers", "http_headers"} {
		value, exists := server[name]
		if !exists {
			continue
		}
		values, ok := value.(map[string]any)
		if !ok {
			return errInvalidHostConfig
		}
		for key, raw := range values {
			if validateSafeIdentifier(key) != nil {
				return errInvalidHostConfig
			}
			text, ok := raw.(string)
			if !ok || len(text) > maximumScalarBytes {
				return errInvalidHostConfig
			}
		}
	}
	return nil
}

func validateSafeIdentifier(value string) error {
	if value == "" || len(value) > 128 {
		return errInvalidHostConfig
	}
	for index, character := range value {
		if !((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '.' || character == '_' || character == '-') || (index == 0 && (character == '.' || character == '_' || character == '-')) {
			return errInvalidHostConfig
		}
	}
	return nil
}

func optionalStringSlice(value any) ([]string, error) {
	if value == nil {
		return nil, nil
	}
	return stringSlice(value)
}

func stringSlice(value any) ([]string, error) {
	raw, ok := value.([]any)
	if !ok || len(raw) > maximumConfigArgs {
		return nil, errInvalidHostConfig
	}
	result := make([]string, len(raw))
	for index, item := range raw {
		text, ok := item.(string)
		if !ok {
			return nil, errInvalidHostConfig
		}
		result[index] = text
	}
	return result, nil
}

func objectAt(value any) (map[string]any, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return nil, errInvalidHostConfig
	}
	return object, nil
}

func decodeStrictJSON(document []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()
	count := 0
	value, err := decodeJSONValue(decoder, 0, &count)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errInvalidHostConfig
	}
	return value, nil
}

func decodeJSONValue(decoder *json.Decoder, depth int, count *int) (any, error) {
	if depth > maximumConfigDepth || *count >= maximumConfigValues {
		return nil, errInvalidHostConfig
	}
	*count++
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if text, ok := token.(string); ok && len(text) > maximumScalarBytes {
		return nil, errInvalidHostConfig
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
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
			if !ok || len(key) > maximumScalarBytes {
				return nil, errInvalidHostConfig
			}
			if _, exists := object[key]; exists {
				return nil, errInvalidHostConfig
			}
			value, err := decodeJSONValue(decoder, depth+1, count)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return nil, errInvalidHostConfig
		}
		return object, nil
	case '[':
		array := []any{}
		for decoder.More() {
			value, err := decodeJSONValue(decoder, depth+1, count)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return nil, errInvalidHostConfig
		}
		return array, nil
	default:
		return nil, errInvalidHostConfig
	}
}

func validateDecodedTree(value any, depth int, count *int) error {
	if depth > maximumConfigDepth || *count >= maximumConfigValues {
		return errInvalidHostConfig
	}
	*count++
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if len(key) > maximumScalarBytes {
				return errInvalidHostConfig
			}
			if err := validateDecodedTree(child, depth+1, count); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range typed {
			if err := validateDecodedTree(child, depth+1, count); err != nil {
				return err
			}
		}
	case string:
		if len(typed) > maximumScalarBytes {
			return errInvalidHostConfig
		}
	}
	return nil
}
