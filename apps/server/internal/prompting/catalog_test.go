package prompting

import (
	"sort"
	"testing"
)

const testJSONL = `{"key":"greeting","text":"Hello, {{.Name}}!"}
{"key":"simple","text":"Plain text prompt."}
`

func TestLoad_ValidJSONL(t *testing.T) {
	catalog, err := Load(testJSONL)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	keys := catalog.Keys()
	sort.Strings(keys)
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
	if keys[0] != "greeting" || keys[1] != "simple" {
		t.Errorf("unexpected keys: %v", keys)
	}
}

func TestLoad_SkipsBlankLines(t *testing.T) {
	raw := "\n\n{\"key\":\"a\",\"text\":\"hello\"}\n\n"
	catalog, err := Load(raw)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(catalog.Keys()) != 1 {
		t.Errorf("expected 1 key, got %d", len(catalog.Keys()))
	}
}

func TestLoad_SkipsEmptyKeyOrText(t *testing.T) {
	raw := `{"key":"","text":"hello"}
{"key":"valid","text":""}
{"key":"ok","text":"world"}`
	catalog, err := Load(raw)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	keys := catalog.Keys()
	if len(keys) != 1 || keys[0] != "ok" {
		t.Errorf("expected only 'ok', got %v", keys)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	_, err := Load(`{invalid json}`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestMustText(t *testing.T) {
	catalog, _ := Load(testJSONL)
	text := catalog.MustText("simple")
	if text != "Plain text prompt." {
		t.Errorf("MustText returned %q, want %q", text, "Plain text prompt.")
	}
}

func TestMustText_Panics(t *testing.T) {
	catalog, _ := Load(testJSONL)
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for missing key")
		}
	}()
	catalog.MustText("nonexistent")
}

func TestMustRender(t *testing.T) {
	catalog, _ := Load(testJSONL)
	result := catalog.MustRender("greeting", map[string]string{"Name": "World"})
	if result != "Hello, World!" {
		t.Errorf("MustRender returned %q, want %q", result, "Hello, World!")
	}
}

func TestMustRender_PanicsOnMissingKey(t *testing.T) {
	catalog, _ := Load(testJSONL)
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for missing key")
		}
	}()
	catalog.MustRender("nonexistent", nil)
}

func TestMustRender_PanicsOnMissingData(t *testing.T) {
	catalog, _ := Load(testJSONL)
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for missing template data")
		}
	}()
	// "greeting" template uses {{.Name}} which requires Name in data
	catalog.MustRender("greeting", map[string]string{})
}
