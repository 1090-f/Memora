package service

import (
	"encoding/json"
	"fmt"
	"strings"
)

type requiredFieldsSchema struct {
	Required []string `json:"required"`
}

func validateRequiredToolArguments(schema json.RawMessage, arguments map[string]any) error {
	if len(schema) == 0 {
		return nil
	}
	var parsed requiredFieldsSchema
	if err := json.Unmarshal(schema, &parsed); err != nil {
		return fmt.Errorf("invalid tool input schema: %w", err)
	}
	for _, field := range parsed.Required {
		value, exists := arguments[field]
		if !exists || isEmptyToolArgument(value) {
			return fmt.Errorf("tool argument %q is required", field)
		}
	}
	return nil
}

func isEmptyToolArgument(value any) bool {
	if value == nil {
		return true
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text) == ""
	}
	return false
}
