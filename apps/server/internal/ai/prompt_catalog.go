package ai

import (
	_ "embed"
	"log"

	"github.com/linguaquest/server/internal/prompting"
)

//go:embed prompt_templates.jsonl
var embeddedPrompts string

var aiPromptCatalog = mustLoadAIPromptCatalog()

func mustLoadAIPromptCatalog() prompting.Catalog {
	catalog, err := prompting.Load(embeddedPrompts)
	if err != nil {
		log.Fatalf("load ai prompt catalog: %v", err)
	}
	return catalog
}
