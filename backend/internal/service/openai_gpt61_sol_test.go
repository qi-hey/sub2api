package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGPT61SolKeepsSeparateIdentityAndReasoningLimits(t *testing.T) {
	require.Equal(t, "gpt-6.1-sol", normalizeKnownOpenAICodexModel("openai/gpt-6.1-sol-max"))
	require.Equal(t, "gpt-6.1-sol", getNormalizedCodexModel("gpt-6.1-sol"))
	require.True(t, openai.IsGPT61SolModelSpelling("gpt-6.1-sol"))
	require.False(t, openai.IsGPT61SolModelSpelling("gpt-6-sol"))
	require.False(t, openai.IsGPT61SolModelSpelling("gpt-6.1-sol-none"))
	require.True(t, isOpenAIGPT6Model("gpt-6.1-sol"))
	require.True(t, configuredCodexSupportsPriorityServiceTier("gpt-6.1-sol"))
	require.Equal(t, "max", normalizeOpenAIReasoningEffortForModel("max", "gpt-6.1-sol"))

	levels := configuredCodexGPTReasoningLevels("gpt-6.1-sol")
	require.Len(t, levels, 6)
	require.Equal(t, "low", levels[0].Effort)
	require.Equal(t, "max", levels[4].Effort)
	require.Equal(t, "ultra", levels[5].Effort)

	model := newConfiguredCodexModelDescriptor("gpt-6.1-sol")
	require.Equal(t, "medium", *model.DefaultReasoningLevel)
	require.Equal(t, "xhigh", *model.MultiAgentReasoningEffort)
	require.Equal(t, "v2", model.MultiAgentVersion)
	require.Equal(t, levels, model.SupportedReasoningLevels)
	require.Equal(t, []configuredCodexServiceTier{{
		ID: "priority", Name: "Fast", Description: "Priority processing for lower latency.",
	}}, model.ServiceTiers)
	require.Equal(t, int64(configuredCodexGPT56MaxContext), model.MaxContextWindow)

	legacy := newConfiguredCodexModelDescriptor("gpt-6-sol")
	require.Nil(t, legacy.MultiAgentReasoningEffort)
	require.NotContains(t, effortsFromConfiguredCodexLevels(legacy.SupportedReasoningLevels), "ultra")
}

func TestGPT61SolResponsesKeepsModelAndMaxEffort(t *testing.T) {
	body := []byte(`{"model":"gpt-6.1-sol","input":"ping","reasoning":{"effort":"max"},"temperature":0.2}`)
	out, changed, err := normalizeGPT6ResponsesSampling(body, "gpt-6.1-sol")
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "gpt-6.1-sol", gjson.GetBytes(out, "model").String())
	require.Equal(t, "max", gjson.GetBytes(out, "reasoning.effort").String())
	require.False(t, gjson.GetBytes(out, "temperature").Exists())
}

func TestGPT61SolFallbackPriceDistinctFromGPT6Sol(t *testing.T) {
	require.Equal(t, 2e-6, openAIGPT61SolFallbackPricing.InputCostPerToken)
	require.Equal(t, 10e-6, openAIGPT61SolFallbackPricing.OutputCostPerToken)
	require.Equal(t, 0.1e-6, openAIGPT61SolFallbackPricing.CacheReadInputTokenCost)
	require.Equal(t, 0.2e-6, openAIGPT6SolFallbackPricing.CacheReadInputTokenCost)
}
