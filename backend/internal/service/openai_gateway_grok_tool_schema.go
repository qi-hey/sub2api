package service

import (
	"encoding/json"
	"strings"
)

var grokObjectSchemaKeywords = map[string]struct{}{
	"properties":            {},
	"required":              {},
	"additionalProperties":  {},
	"patternProperties":     {},
	"dependentRequired":     {},
	"dependentSchemas":      {},
	"dependencies":          {},
	"propertyNames":         {},
	"minProperties":         {},
	"maxProperties":         {},
	"unevaluatedProperties": {},
}

// normalizeGrokFunctionToolSchema makes a function tool parameter root
// acceptable to xAI. Function arguments are JSON objects, so primitive/null
// branches in a root oneOf/anyOf cannot be used by the caller and may be
// removed without changing any callable object variant. If no object variant
// remains, the caller drops only that incompatible tool.
func normalizeGrokFunctionToolSchema(raw json.RawMessage) ([]byte, bool, bool, error) {
	var tool map[string]any
	if err := decodeOpenAIJSONUseNumber(raw, &tool); err != nil {
		return nil, false, false, err
	}

	parameters, exists := tool["parameters"]
	if !exists || parameters == nil {
		tool["parameters"] = emptyGrokObjectSchema()
		encoded, err := marshalOpenAIUpstreamJSON(tool)
		return encoded, true, true, err
	}

	schema, ok := parameters.(map[string]any)
	if !ok {
		return nil, true, false, nil
	}
	changed, supported := normalizeGrokObjectSchema(schema, 0)
	if !supported {
		return nil, true, false, nil
	}
	if !changed {
		return raw, false, true, nil
	}
	tool["parameters"] = schema
	encoded, err := marshalOpenAIUpstreamJSON(tool)
	return encoded, true, true, err
}

func normalizeGrokObjectSchema(schema map[string]any, depth int) (bool, bool) {
	if schema == nil || depth > openAIResponsesObjectUnionMaxDepth {
		return false, false
	}

	changed := false
	unionSeen := false
	for _, keyword := range []string{"oneOf", "anyOf"} {
		value, exists := schema[keyword]
		if !exists {
			continue
		}
		unionSeen = true
		branches, ok := value.([]any)
		if !ok || len(branches) == 0 {
			return changed, false
		}

		objectBranches := make([]any, 0, len(branches))
		for _, branch := range branches {
			normalized, branchChanged, compatible := normalizeGrokObjectSchemaBranch(branch, depth+1)
			if !compatible {
				changed = true
				continue
			}
			if branchChanged {
				changed = true
			}
			objectBranches = append(objectBranches, normalized)
		}
		if len(objectBranches) == 0 {
			return changed, false
		}
		if len(objectBranches) != len(branches) {
			changed = true
		}
		schema[keyword] = objectBranches
	}

	switch schemaType := schema["type"].(type) {
	case nil:
		if !unionSeen && schemaExplicitlyExcludesObjects(schema) {
			return changed, false
		}
		schema["type"] = "object"
		changed = true
	case string:
		if strings.TrimSpace(schemaType) != "object" {
			if !unionSeen {
				return changed, false
			}
			schema["type"] = "object"
			changed = true
		}
	case []any:
		if !grokSchemaTypeArrayContainsObject(schemaType) {
			return changed, false
		}
		schema["type"] = "object"
		changed = true
	default:
		schema["type"] = "object"
		changed = true
	}
	return changed, true
}

func normalizeGrokObjectSchemaBranch(branch any, depth int) (any, bool, bool) {
	switch typed := branch.(type) {
	case bool:
		if !typed {
			return nil, false, false
		}
		return emptyGrokObjectSchema(), true, true
	case map[string]any:
		changed, supported := normalizeGrokObjectSchema(typed, depth)
		return typed, changed, supported
	default:
		return nil, false, false
	}
}

func grokSchemaTypeArrayContainsObject(types []any) bool {
	for _, raw := range types {
		if value, ok := raw.(string); ok && strings.TrimSpace(value) == "object" {
			return true
		}
	}
	return false
}

func schemaExplicitlyExcludesObjects(schema map[string]any) bool {
	if value, exists := schema["const"]; exists {
		if _, ok := value.(map[string]any); !ok {
			return true
		}
	}
	if rawEnum, exists := schema["enum"]; exists {
		values, ok := rawEnum.([]any)
		if !ok || len(values) == 0 {
			return true
		}
		for _, value := range values {
			if _, ok := value.(map[string]any); ok {
				return false
			}
		}
		return true
	}
	if len(schema) == 0 {
		return false
	}
	for keyword := range grokObjectSchemaKeywords {
		if _, exists := schema[keyword]; exists {
			return false
		}
	}
	// Unknown annotations and composition keywords do not prove a primitive
	// root. Restrict them to object rather than dropping a potentially usable
	// function tool.
	return false
}

func emptyGrokObjectSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}
