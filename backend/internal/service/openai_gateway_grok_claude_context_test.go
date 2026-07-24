package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/stretchr/testify/require"
)

func TestShouldScaleGrokClaudeClientUsage_IsNarrowlyScoped(t *testing.T) {
	grok := &Account{Platform: PlatformGrok}
	require.True(t, shouldScaleGrokClaudeClientUsage(grok, "claude-opus-4-8", "grok-4.5"))
	require.False(t, shouldScaleGrokClaudeClientUsage(grok, "grok-4.5", "grok-4.5"))
	require.False(t, shouldScaleGrokClaudeClientUsage(grok, "claude-opus-4-8", "grok-4.3"))
	require.False(t, shouldScaleGrokClaudeClientUsage(&Account{Platform: PlatformAnthropic}, "claude-opus-4-8", "grok-4.5"))
}

func TestScaleGrokClaudeClientResponsesUsage_DoesNotMutateBillingUsage(t *testing.T) {
	original := &apicompat.ResponsesUsage{
		InputTokens:              440000,
		OutputTokens:             23,
		CacheCreationInputTokens: 1000,
		InputTokensDetails:       &apicompat.ResponsesInputTokensDetails{CachedTokens: 400000},
	}

	scaled := scaleGrokClaudeClientResponsesUsage(original)

	require.Equal(t, 968000, scaled.InputTokens)
	require.Equal(t, 880000, scaled.InputTokensDetails.CachedTokens)
	require.Equal(t, 2200, scaled.CacheCreationInputTokens)
	require.Equal(t, 23, scaled.OutputTokens)
	require.Equal(t, 440000, original.InputTokens)
	require.Equal(t, 400000, original.InputTokensDetails.CachedTokens)
}
