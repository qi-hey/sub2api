package service

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
)

const (
	grokClaudeContextScaleNumerator   = 11
	grokClaudeContextScaleDenominator = 5
)

func shouldScaleGrokClaudeClientUsage(account *Account, originalModel, upstreamModel string) bool {
	return account != nil &&
		account.Platform == PlatformGrok &&
		strings.EqualFold(strings.TrimSpace(originalModel), "claude-opus-4-8") &&
		strings.EqualFold(strings.TrimSpace(upstreamModel), grokDefaultResponsesModel)
}

// scaleGrokClaudeClientInputTokens reports the 500k Grok window in the 1M
// Claude Opus coordinate space. The small extra margin makes Claude Code
// compact before the translated request reaches xAI's hard prompt limit.
func scaleGrokClaudeClientInputTokens(tokens int) int {
	if tokens <= 0 {
		return tokens
	}
	return (tokens*grokClaudeContextScaleNumerator + grokClaudeContextScaleDenominator - 1) /
		grokClaudeContextScaleDenominator
}

// ScaleGrokClaudeClientInputTokens keeps the local count_tokens response in
// the same client-visible coordinate space as streamed Messages usage.
func ScaleGrokClaudeClientInputTokens(tokens int) int {
	return scaleGrokClaudeClientInputTokens(tokens)
}

func scaleGrokClaudeClientResponsesUsage(usage *apicompat.ResponsesUsage) *apicompat.ResponsesUsage {
	if usage == nil {
		return nil
	}
	scaled := *usage
	scaled.InputTokens = scaleGrokClaudeClientInputTokens(usage.InputTokens)
	scaled.CacheCreationInputTokens = scaleGrokClaudeClientInputTokens(usage.CacheCreationInputTokens)
	if usage.InputTokensDetails != nil {
		details := *usage.InputTokensDetails
		details.CachedTokens = scaleGrokClaudeClientInputTokens(usage.InputTokensDetails.CachedTokens)
		scaled.InputTokensDetails = &details
	}
	return &scaled
}

func grokClaudeClientResponsesEvent(event *apicompat.ResponsesStreamEvent) *apicompat.ResponsesStreamEvent {
	if event == nil {
		return nil
	}
	scaled := *event
	scaled.Usage = scaleGrokClaudeClientResponsesUsage(event.Usage)
	if event.Response != nil {
		response := *event.Response
		response.Usage = scaleGrokClaudeClientResponsesUsage(event.Response.Usage)
		scaled.Response = &response
	}
	return &scaled
}
