package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIResponsesAllowsFreshGrokFallback(t *testing.T) {
	require.True(t, openAIResponsesAllowsFreshGrokFallback([]byte(`{"model":"gpt-5.4","input":"hello"}`)))
	require.True(t, openAIResponsesAllowsFreshGrokFallback([]byte(`{"type":"response.create","model":"gpt-5.4"}`)))
	require.True(t, openAIResponsesAllowsFreshGrokFallback([]byte(`{"model":"gpt-5.4","input":[{"role":"developer","content":"rules"},{"role":"user","content":"hello"}]}`)))
	require.False(t, openAIResponsesAllowsFreshGrokFallback([]byte(`{"model":"gpt-5.4"}`)))
	require.False(t, openAIResponsesAllowsFreshGrokFallback([]byte(`{"model":"gpt-5.4","previous_response_id":"resp_1","input":"next"}`)))
	require.False(t, openAIResponsesAllowsFreshGrokFallback([]byte(`{"model":"gpt-5.4","input":[{"role":"user","content":"hello"},{"role":"assistant","content":"answer"},{"role":"user","content":"next"}]}`)))
	require.False(t, openAIResponsesAllowsFreshGrokFallback([]byte(`{"model":"gpt-5.4","input":[{"type":"function_call_output","call_id":"call_1","output":"ok"}]}`)))
}

func TestOpenAIChatAllowsFreshGrokFallback(t *testing.T) {
	require.True(t, openAIChatAllowsFreshGrokFallback([]byte(`{"messages":[{"role":"system","content":"rules"},{"role":"user","content":"hello"}]}`)))
	require.False(t, openAIChatAllowsFreshGrokFallback([]byte(`{"messages":[{"role":"user","content":"hello"},{"role":"assistant","content":"answer"},{"role":"user","content":"next"}]}`)))
	require.False(t, openAIChatAllowsFreshGrokFallback([]byte(`{"messages":[{"role":"tool","content":"result"}]}`)))
}

func TestAnthropicMessagesAllowFreshGrokFallback(t *testing.T) {
	require.True(t, anthropicMessagesAllowFreshGrokFallback([]byte(`{"messages":[{"role":"user","content":"hello"}]}`)))
	require.False(t, anthropicMessagesAllowFreshGrokFallback([]byte(`{"messages":[{"role":"assistant","content":"answer"},{"role":"user","content":"next"}]}`)))
	require.False(t, anthropicMessagesAllowFreshGrokFallback([]byte(`{"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"ok"}]}]}`)))
}

func TestOpenAIToGrokFallbackRouteBlocksFreshSwitchWithHistory(t *testing.T) {
	route := newOpenAIToGrokFallbackRoute(openAIToGrokRouteTestAPIKey("openai", false), nil, "gpt-5.4")
	require.True(t, route.canRestoreContinuity())
	route.setFreshFallbackAllowed(false)
	require.False(t, route.canFallback())
	require.True(t, route.canRestoreContinuity())
}
