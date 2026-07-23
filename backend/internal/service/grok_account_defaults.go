package service

import "maps"

import "strings"

const defaultGrokFallbackTargetModel = "grok-4.5"

var openAIToGrokFallbackModels = [...]string{
	"gpt-5.2",
	"gpt-5.4",
	"gpt-5.4-mini",
	"gpt-5.5",
	"gpt-5.6-luna",
	"gpt-5.6-sol",
	"gpt-5.6-terra",
}

var defaultGrokCreateModelMapping = buildDefaultGrokCreateModelMapping()

func buildDefaultGrokCreateModelMapping() map[string]any {
	mapping := map[string]any{
		"claude-opus-4-8": defaultGrokFallbackTargetModel,
		"grok-4.5":        defaultGrokFallbackTargetModel,
	}
	for _, model := range openAIToGrokFallbackModels {
		mapping[model] = defaultGrokFallbackTargetModel
	}
	return mapping
}

func IsOpenAIToGrokFallbackModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	for _, candidate := range openAIToGrokFallbackModels {
		if model == candidate {
			return true
		}
	}
	return false
}

func ApplyGrokCreateDefaults(credentials map[string]any) map[string]any {
	clone := maps.Clone(credentials)
	if clone == nil {
		clone = make(map[string]any)
	}
	mapping := maps.Clone(defaultGrokCreateModelMapping)
	switch explicit := clone["model_mapping"].(type) {
	case map[string]any:
		maps.Copy(mapping, explicit)
	case map[string]string:
		for source, target := range explicit {
			mapping[source] = target
		}
	}
	clone["model_mapping"] = mapping
	return clone
}
