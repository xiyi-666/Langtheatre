package service

import (
	_ "embed"
	"log"

	"github.com/linguaquest/server/internal/prompting"
)

//go:embed prompt_templates.jsonl
var embeddedServicePrompts string

var servicePromptCatalog = mustLoadServicePromptCatalog()

func mustLoadServicePromptCatalog() prompting.Catalog {
	catalog, err := prompting.Load(embeddedServicePrompts)
	if err != nil {
		log.Fatalf("load service prompt catalog: %v", err)
	}
	return catalog
}
