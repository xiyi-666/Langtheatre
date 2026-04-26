package ai

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"log"
	"slices"
	"sort"
	"strings"
)

//go:embed voice_styles.jsonl
var embeddedVoiceStyles string

type voiceStyleRecord struct {
	Style       string   `json:"style"`
	Instruction string   `json:"instruction"`
	Aliases     []string `json:"aliases"`
}

type voiceCatalog struct {
	records      []voiceStyleRecord
	aliasToStyle map[string]string
	orderedAlias []string
}

var defaultVoiceCatalog = mustLoadVoiceCatalog()

func mustLoadVoiceCatalog() voiceCatalog {
	catalog, err := loadVoiceCatalog(embeddedVoiceStyles)
	if err != nil {
		log.Fatalf("load voice catalog: %v", err)
	}
	return catalog
}

func loadVoiceCatalog(raw string) (voiceCatalog, error) {
	catalog := voiceCatalog{
		records:      make([]voiceStyleRecord, 0, 8),
		aliasToStyle: make(map[string]string),
		orderedAlias: make([]string, 0, 16),
	}
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record voiceStyleRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return voiceCatalog{}, err
		}
		record.Style = strings.TrimSpace(record.Style)
		record.Instruction = strings.TrimSpace(record.Instruction)
		if record.Style == "" || record.Instruction == "" {
			continue
		}
		record.Aliases = normalizeAliases(record.Style, record.Aliases)
		catalog.records = append(catalog.records, record)
		for _, alias := range record.Aliases {
			if _, exists := catalog.aliasToStyle[alias]; !exists {
				catalog.orderedAlias = append(catalog.orderedAlias, alias)
			}
			catalog.aliasToStyle[alias] = record.Style
		}
	}
	if err := scanner.Err(); err != nil {
		return voiceCatalog{}, err
	}
	// Prefer more specific aliases first (e.g. "soft-female" before "female").
	sort.SliceStable(catalog.orderedAlias, func(i, j int) bool {
		return len(catalog.orderedAlias[i]) > len(catalog.orderedAlias[j])
	})
	return catalog, nil
}

func normalizeAliases(style string, aliases []string) []string {
	normalized := make([]string, 0, len(aliases)+1)
	normalized = append(normalized, normalizeVoiceAlias(style))
	for _, alias := range aliases {
		candidate := normalizeVoiceAlias(alias)
		if candidate == "" || slices.Contains(normalized, candidate) {
			continue
		}
		normalized = append(normalized, candidate)
	}
	return normalized
}

func normalizeVoiceAlias(input string) string {
	return strings.ToLower(strings.TrimSpace(input))
}

func normalizeVoiceStyleFromCatalog(input string) string {
	normalized := normalizeVoiceAlias(input)
	if normalized == "" {
		return ""
	}
	if style, ok := defaultVoiceCatalog.aliasToStyle[normalized]; ok {
		return style
	}
	for _, alias := range defaultVoiceCatalog.orderedAlias {
		style := defaultVoiceCatalog.aliasToStyle[alias]
		if alias != "" && strings.Contains(normalized, alias) {
			return style
		}
	}
	return ""
}

func voiceStyleInstructionFromCatalog(style string) string {
	for _, record := range defaultVoiceCatalog.records {
		if record.Style == style {
			return record.Instruction
		}
	}
	return ""
}

func maintainedVoiceStyles() []string {
	styles := make([]string, 0, len(defaultVoiceCatalog.records))
	for _, record := range defaultVoiceCatalog.records {
		styles = append(styles, record.Style)
	}
	return styles
}
