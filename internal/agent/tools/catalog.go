package tools

import (
	"regexp"
	"strings"

	"github.com/1090-f/Memora/internal/contracts"
)

const (
	CapabilityWebFetch        = "web.fetch"
	CapabilityWebSearch       = "web.search"
	CapabilityKnowledgeSearch = "knowledge.search"
	CapabilityDocumentRead    = "document.read"
)

var invalidToolAliasChar = regexp.MustCompile("[^a-zA-Z0-9_-]+")

func CatalogSpecs(specs []contracts.ToolSpec) []contracts.ToolSpec {
	result := make([]contracts.ToolSpec, len(specs))
	shortCounts := make(map[string]int)
	for _, spec := range specs {
		shortCounts[ShortToolName(spec.Name)]++
	}
	for i, spec := range specs {
		shortName := ShortToolName(spec.Name)
		if spec.Alias == "" {
			spec.Alias = shortName
			if shortCounts[shortName] > 1 {
				prefix := spec.SourceID
				if len(prefix) > 8 {
					prefix = prefix[:8]
				}
				spec.Alias = prefix + "_" + shortName
			}
			spec.Alias = invalidToolAliasChar.ReplaceAllString(spec.Alias, "_")
		}
		if len(spec.Capabilities) == 0 {
			spec.Capabilities = InferToolCapabilities(spec)
		}
		result[i] = spec
	}
	return result
}

func ShortToolName(name string) string {
	if index := strings.LastIndex(name, "::"); index >= 0 {
		return name[index+2:]
	}
	return name
}

func InferToolCapabilities(spec contracts.ToolSpec) []string {
	text := strings.ToLower(ShortToolName(spec.Name) + " " + spec.Alias + " " + spec.Description)
	var capabilities []string
	add := func(capability string) {
		for _, existing := range capabilities {
			if existing == capability {
				return
			}
		}
		capabilities = append(capabilities, capability)
	}
	if strings.Contains(text, "fetch_url") || strings.Contains(text, "fetch url") || strings.Contains(text, "web fetch") {
		add(CapabilityWebFetch)
	}
	if strings.Contains(text, "search") {
		add(CapabilityWebSearch)
	}
	if strings.Contains(text, "knowledge_search") {
		add(CapabilityKnowledgeSearch)
	}
	if strings.Contains(text, "document_read") {
		add(CapabilityDocumentRead)
	}
	return capabilities
}

func HasCapability(spec contracts.ToolSpec, capability string) bool {
	for _, item := range spec.Capabilities {
		if item == capability {
			return true
		}
	}
	return false
}
