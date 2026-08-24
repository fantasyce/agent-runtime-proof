package canonical

import (
	"encoding/json"
	"testing"
)

func TestMarshalSortsObjectProperties(t *testing.T) {
	got, err := Marshal(map[string]any{"b": int64(1), "a": int64(2)})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"a":2,"b":1}` {
		t.Fatalf("canonical JSON = %s", got)
	}
}

func TestMarshalUsesUTF16PropertyOrder(t *testing.T) {
	got, err := Marshal(map[string]any{"\ue000": int64(1), "\U00010000": int64(2)})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "{\"𐀀\":2,\"\":1}" {
		t.Fatalf("canonical JSON = %s", got)
	}
}

func TestMarshalRejectsUnsafeInteger(t *testing.T) {
	if _, err := Marshal(json.Number("9007199254740992")); err == nil {
		t.Fatal("unsafe integer accepted")
	}
}

func TestMarshalRejectsFloats(t *testing.T) {
	if _, err := Marshal(1.5); err == nil {
		t.Fatal("floating-point value accepted")
	}
}
