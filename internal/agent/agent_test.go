package agent

import (
	"encoding/json"
	"testing"
)

// --- getInt ---

func TestGetInt_Float64(t *testing.T) {
	p := map[string]any{"id": float64(42)}
	v, ok := getInt(p, "id")
	if !ok || v != 42 {
		t.Errorf("expected (42, true), got (%d, %v)", v, ok)
	}
}

func TestGetInt_Int64(t *testing.T) {
	p := map[string]any{"id": int64(99)}
	v, ok := getInt(p, "id")
	if !ok || v != 99 {
		t.Errorf("expected (99, true), got (%d, %v)", v, ok)
	}
}

func TestGetInt_JSONNumber(t *testing.T) {
	p := map[string]any{"id": json.Number("7")}
	v, ok := getInt(p, "id")
	if !ok || v != 7 {
		t.Errorf("expected (7, true), got (%d, %v)", v, ok)
	}
}

func TestGetInt_JSONNumberInvalid(t *testing.T) {
	p := map[string]any{"id": json.Number("not_a_number")}
	_, ok := getInt(p, "id")
	if ok {
		t.Error("expected false for invalid json.Number")
	}
}

func TestGetInt_MissingKey(t *testing.T) {
	p := map[string]any{}
	v, ok := getInt(p, "id")
	if ok || v != 0 {
		t.Errorf("expected (0, false), got (%d, %v)", v, ok)
	}
}

func TestGetInt_WrongType(t *testing.T) {
	p := map[string]any{"id": "hello"}
	_, ok := getInt(p, "id")
	if ok {
		t.Error("expected false for string value")
	}
}

func TestGetInt_NilMap(t *testing.T) {
	v, ok := getInt(nil, "id")
	if ok || v != 0 {
		t.Errorf("expected (0, false), got (%d, %v)", v, ok)
	}
}

// --- getString ---

func TestGetString_Present(t *testing.T) {
	p := map[string]any{"key": "value"}
	v, ok := getString(p, "key")
	if !ok || v != "value" {
		t.Errorf("expected (value, true), got (%s, %v)", v, ok)
	}
}

func TestGetString_MissingKey(t *testing.T) {
	p := map[string]any{}
	v, ok := getString(p, "key")
	if ok || v != "" {
		t.Errorf("expected ('', false), got (%s, %v)", v, ok)
	}
}

func TestGetString_WrongType(t *testing.T) {
	p := map[string]any{"key": 123}
	_, ok := getString(p, "key")
	if ok {
		t.Error("expected false for non-string value")
	}
}

func TestGetString_EmptyString(t *testing.T) {
	p := map[string]any{"key": ""}
	v, ok := getString(p, "key")
	if !ok || v != "" {
		t.Errorf("expected ('', true), got (%s, %v)", v, ok)
	}
}

// --- truncate ---

func TestTruncate_Short(t *testing.T) {
	got := truncate("hello", 10)
	if got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
}

func TestTruncate_Exact(t *testing.T) {
	got := truncate("hello", 5)
	if got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
}

func TestTruncate_Long(t *testing.T) {
	got := truncate("hello world", 5)
	if got != "hello..." {
		t.Errorf("expected 'hello...', got %q", got)
	}
}

func TestTruncate_Empty(t *testing.T) {
	got := truncate("", 5)
	if got != "" {
		t.Errorf("expected '', got %q", got)
	}
}
