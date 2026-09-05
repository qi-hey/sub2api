package service

import "maps"

const defaultOpenAIAstraModel = "gpt-6-astra"

// ApplyOpenAICreateDefaults adds model defaults only to official OpenAI
// OAuth-like accounts. OpenAI-compatible API key accounts may point at other
// providers, so their model mappings must remain untouched.
func ApplyOpenAICreateDefaults(platform, accountType string, credentials map[string]any) map[string]any {
	if platform != PlatformOpenAI || (accountType != AccountTypeOAuth && accountType != AccountTypeSetupToken) {
		return credentials
	}

	clone := maps.Clone(credentials)
	if clone == nil {
		clone = make(map[string]any)
	}

	var mapping map[string]any
	switch explicit := clone["model_mapping"].(type) {
	case map[string]any:
		mapping = maps.Clone(explicit)
	case map[string]string:
		mapping = make(map[string]any, len(explicit))
		for source, target := range explicit {
			mapping[source] = target
		}
	}
	// Missing/empty mappings mean "allow every model". Turning that into a
	// one-entry map would accidentally restrict the account to Astra only.
	if len(mapping) == 0 {
		return clone
	}
	if _, exists := mapping[defaultOpenAIAstraModel]; !exists {
		mapping[defaultOpenAIAstraModel] = defaultOpenAIAstraModel
	}
	clone["model_mapping"] = mapping
	return clone
}
