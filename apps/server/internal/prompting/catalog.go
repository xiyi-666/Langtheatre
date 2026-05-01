package prompting

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
)

type TextRecord struct {
	Key  string `json:"key"`
	Text string `json:"text"`
}

type Catalog struct {
	records   map[string]TextRecord
	templates map[string]*template.Template
}

func Load(raw string) (Catalog, error) {
	catalog := Catalog{
		records:   make(map[string]TextRecord),
		templates: make(map[string]*template.Template),
	}
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record TextRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return Catalog{}, err
		}
		record.Key = strings.TrimSpace(record.Key)
		record.Text = strings.TrimSpace(record.Text)
		if record.Key == "" || record.Text == "" {
			continue
		}
		tmpl, err := template.New(record.Key).Option("missingkey=error").Parse(record.Text)
		if err != nil {
			return Catalog{}, fmt.Errorf("parse prompt %s: %w", record.Key, err)
		}
		catalog.records[record.Key] = record
		catalog.templates[record.Key] = tmpl
	}
	if err := scanner.Err(); err != nil {
		return Catalog{}, err
	}
	return catalog, nil
}

func (c Catalog) MustText(key string) string {
	record, ok := c.records[key]
	if !ok {
		panic("prompt key not found: " + key)
	}
	return record.Text
}

func (c Catalog) MustRender(key string, data any) string {
	tmpl, ok := c.templates[key]
	if !ok {
		panic("prompt key not found: " + key)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		panic(fmt.Sprintf("render prompt %s: %v", key, err))
	}
	return buf.String()
}

func (c Catalog) Keys() []string {
	keys := make([]string, 0, len(c.records))
	for key := range c.records {
		keys = append(keys, key)
	}
	return keys
}
