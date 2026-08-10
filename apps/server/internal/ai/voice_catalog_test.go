package ai

import (
	"testing"
)

const testVoiceJSONL = `{"style":"温柔女生","instruction":"请用温柔女生音色说","aliases":["温柔女生","female","gentle"]}
{"style":"播音男生","instruction":"请用播音男生音色说","aliases":["播音男生","broadcast","male-announcer"]}
`

func TestLoadVoiceCatalog_Valid(t *testing.T) {
	catalog, err := loadVoiceCatalog(testVoiceJSONL)
	if err != nil {
		t.Fatalf("loadVoiceCatalog error: %v", err)
	}
	if len(catalog.records) != 2 {
		t.Errorf("expected 2 records, got %d", len(catalog.records))
	}
}

func TestLoadVoiceCatalog_Empty(t *testing.T) {
	catalog, err := loadVoiceCatalog("")
	if err != nil {
		t.Fatalf("loadVoiceCatalog error: %v", err)
	}
	if len(catalog.records) != 0 {
		t.Errorf("expected 0 records, got %d", len(catalog.records))
	}
}

func TestLoadVoiceCatalog_InvalidJSON(t *testing.T) {
	_, err := loadVoiceCatalog(`{bad json}`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadVoiceCatalog_SkipsEmptyStyleOrInstruction(t *testing.T) {
	raw := `{"style":"","instruction":"some instruction","aliases":[]}
{"style":"valid","instruction":"","aliases":[]}`
	catalog, err := loadVoiceCatalog(raw)
	if err != nil {
		t.Fatalf("loadVoiceCatalog error: %v", err)
	}
	if len(catalog.records) != 0 {
		t.Errorf("expected 0 records (all filtered), got %d", len(catalog.records))
	}
}

func TestLoadVoiceCatalog_AliasMapping(t *testing.T) {
	catalog, err := loadVoiceCatalog(testVoiceJSONL)
	if err != nil {
		t.Fatalf("loadVoiceCatalog error: %v", err)
	}
	style, ok := catalog.aliasToStyle["female"]
	if !ok {
		t.Fatal("alias 'female' should map to a style")
	}
	if style != "温柔女生" {
		t.Errorf("alias 'female' mapped to %q, want 温柔女生", style)
	}
	style2, ok := catalog.aliasToStyle["broadcast"]
	if !ok {
		t.Fatal("alias 'broadcast' should map to a style")
	}
	if style2 != "播音男生" {
		t.Errorf("alias 'broadcast' mapped to %q, want 播音男生", style2)
	}
}

func TestLoadVoiceCatalog_OrderedAliasByLength(t *testing.T) {
	catalog, err := loadVoiceCatalog(testVoiceJSONL)
	if err != nil {
		t.Fatalf("loadVoiceCatalog error: %v", err)
	}
	// Longer aliases should come first
	for i := 0; i < len(catalog.orderedAlias)-1; i++ {
		if len(catalog.orderedAlias[i]) < len(catalog.orderedAlias[i+1]) {
			t.Errorf("orderedAlias not sorted by length desc at index %d: %q < %q",
				i, catalog.orderedAlias[i], catalog.orderedAlias[i+1])
		}
	}
}

func TestNormalizeVoiceAlias(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Female", "female"},
		{"  BROADCAST  ", "broadcast"},
		{"", ""},
		{"温柔女生", "温柔女生"},
	}
	for _, tc := range cases {
		got := normalizeVoiceAlias(tc.input)
		if got != tc.want {
			t.Errorf("normalizeVoiceAlias(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestNormalizeAliases(t *testing.T) {
	result := normalizeAliases("MyStyle", []string{"alias1", "ALIAS1", "alias2"})
	// Should contain normalized style + unique aliases
	if len(result) < 2 {
		t.Errorf("expected at least 2 entries, got %d", len(result))
	}
	if result[0] != "mystyle" {
		t.Errorf("first alias should be normalized style 'mystyle', got %q", result[0])
	}
}

func TestNormalizeAliases_DeduplicatesEmpty(t *testing.T) {
	result := normalizeAliases("x", []string{"", ""})
	// Should have "x" but not empty strings
	if len(result) != 1 {
		t.Errorf("expected 1 alias, got %d: %v", len(result), result)
	}
}
