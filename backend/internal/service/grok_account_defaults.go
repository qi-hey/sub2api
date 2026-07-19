package service

import "maps"

var defaultGrokCreateModelMapping = map[string]any{
	"claude-opus-4-8": "grok-4.5",
	"gpt-5.4":         "grok-4.5",
	"grok-4.5":        "grok-4.5",
}

func ApplyGrokCreateDefaults(credentials map[string]any) map[string]any {
	if _, explicit := credentials["model_mapping"]; explicit {
		return credentials
	}

	clone := maps.Clone(credentials)
	if clone == nil {
		clone = make(map[string]any)
	}
	clone["model_mapping"] = maps.Clone(defaultGrokCreateModelMapping)
	return clone
}
