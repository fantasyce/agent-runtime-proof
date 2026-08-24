package canonical

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const maxSafeInteger int64 = 9007199254740991

func Marshal(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode canonical input: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var normalized any
	if err := decoder.Decode(&normalized); err != nil {
		return nil, fmt.Errorf("decode canonical input: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, errors.New("canonical input contains trailing JSON")
	}
	var output bytes.Buffer
	if err := appendValue(&output, normalized); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func appendValue(output *bytes.Buffer, value any) error {
	switch typed := value.(type) {
	case nil:
		output.WriteString("null")
	case bool:
		if typed {
			output.WriteString("true")
		} else {
			output.WriteString("false")
		}
	case string:
		return appendString(output, typed)
	case json.Number:
		return appendInteger(output, typed.String())
	case []any:
		output.WriteByte('[')
		for index, item := range typed {
			if index > 0 {
				output.WriteByte(',')
			}
			if err := appendValue(output, item); err != nil {
				return err
			}
		}
		output.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		slices.SortFunc(keys, compareUTF16)
		output.WriteByte('{')
		for index, key := range keys {
			if index > 0 {
				output.WriteByte(',')
			}
			if err := appendString(output, key); err != nil {
				return err
			}
			output.WriteByte(':')
			if err := appendValue(output, typed[key]); err != nil {
				return err
			}
		}
		output.WriteByte('}')
	default:
		return fmt.Errorf("unsupported canonical JSON type %T", value)
	}
	return nil
}

func appendInteger(output *bytes.Buffer, value string) error {
	if strings.ContainsAny(value, ".eE") || value == "-0" || strings.HasPrefix(value, "+") {
		return fmt.Errorf("canonical JSON number %q is not an integer", value)
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < -maxSafeInteger || parsed > maxSafeInteger {
		return fmt.Errorf("canonical JSON integer %q is outside the I-JSON safe range", value)
	}
	if strconv.FormatInt(parsed, 10) != value {
		return fmt.Errorf("canonical JSON integer %q is not minimally encoded", value)
	}
	output.WriteString(value)
	return nil
}

func appendString(output *bytes.Buffer, value string) error {
	if !utf8.ValidString(value) {
		return errors.New("canonical JSON string is not valid UTF-8")
	}
	output.WriteByte('"')
	for _, character := range value {
		switch character {
		case '"', '\\':
			output.WriteByte('\\')
			output.WriteRune(character)
		case '\b':
			output.WriteString(`\b`)
		case '\t':
			output.WriteString(`\t`)
		case '\n':
			output.WriteString(`\n`)
		case '\f':
			output.WriteString(`\f`)
		case '\r':
			output.WriteString(`\r`)
		default:
			if character < 0x20 {
				fmt.Fprintf(output, `\u%04x`, character)
			} else {
				output.WriteRune(character)
			}
		}
	}
	output.WriteByte('"')
	return nil
}

func compareUTF16(left, right string) int {
	return slices.Compare(utf16.Encode([]rune(left)), utf16.Encode([]rune(right)))
}
