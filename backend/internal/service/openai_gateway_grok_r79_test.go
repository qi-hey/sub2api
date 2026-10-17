package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func r79OpaquePayload() string {
	payload := make([]byte, 89)
	payload[0] = 0x80
	return base64.URLEncoding.EncodeToString(payload)
}

func r79Agent(kind, author, recipient, text string, encrypted bool) map[string]any {
	parts := []any{map[string]any{"type": "input_text", "text": "Message Type: " + kind +
		"\nTask name: " + recipient + "\nSender: " + author + "\nPayload:\n" + text}}
	if encrypted {
		parts = append(parts, map[string]any{"type": "encrypted_content", "encrypted_content": r79OpaquePayload()})
	}
	return map[string]any{"type": "agent_message", "author": author, "recipient": recipient, "content": parts}
}

func r79ReplayBody(t *testing.T, items ...any) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"model": "grok-4.7", "input": items, "tools": []any{}, "reasoning": map[string]any{"effort": "xhigh"},
	})
	require.NoError(t, err)
	return body
}

func r79User() map[string]any {
	return map[string]any{"type": "message", "role": "user", "content": "CURRENT_USER_MARKER"}
}

func TestR79CompletedAgentHistoryPreservesReadableContent(t *testing.T) {
	body := r79ReplayBody(t,
		map[string]any{"type": "message", "role": "user", "content": "PRIOR_CONTEXT_MARKER"},
		r79Agent("MESSAGE", "/root/worker", "/root", "INTERMEDIATE_READABLE_MARKER", true),
		r79Agent("FINAL_ANSWER", "/root/worker", "/root", "FINAL_RESULT_MARKER", false),
		r79User())
	before := append([]byte(nil), body...)
	got, _, err := patchGrokResponsesBodyWithClientTools(body, "grok-4.7")
	require.NoError(t, err)
	require.Equal(t, before, body)
	require.Len(t, gjson.GetBytes(got, "input").Array(), 4)
	require.NotContains(t, string(got), r79OpaquePayload())
	require.Equal(t, grokCompletedAgentHistoryNotice, gjson.GetBytes(got, "input.1.content.1.text").String())
	for _, marker := range []string{"PRIOR_CONTEXT_MARKER", "INTERMEDIATE_READABLE_MARKER", "FINAL_RESULT_MARKER", "CURRENT_USER_MARKER"} {
		require.Contains(t, string(got), marker)
	}
	again, _, err := patchGrokResponsesBodyWithClientTools(got, "grok-4.7")
	require.NoError(t, err)
	require.JSONEq(t, string(got), string(again))

	// Switching back uses the original request, including its opaque payload.
	gpt, _, err := adaptResponsesClientToolsForFunctionUpstreamWithMapping(body, "OpenAI WS HTTP bridge", apicompat.ResponsesClientToolMapping{})
	require.NoError(t, err)
	require.Contains(t, string(gpt), r79OpaquePayload())
	require.NotContains(t, string(gpt), grokCompletedAgentHistoryNotice)
}

func TestR79UnresolvedAgentContentStillFails(t *testing.T) {
	tests := []struct {
		name  string
		build func() []any
	}{
		{"no_final", func() []any {
			return []any{r79Agent("MESSAGE", "/root/a", "/root", "", true), r79User()}
		}},
		{"earlier_final", func() []any {
			return []any{r79Agent("FINAL_ANSWER", "/root/a", "/root", "done", false), r79Agent("MESSAGE", "/root/a", "/root", "", true), r79User()}
		}},
		{"different_author", func() []any {
			return []any{r79Agent("MESSAGE", "/root/a", "/root", "", true), r79Agent("FINAL_ANSWER", "/root/b", "/root", "done", false), r79User()}
		}},
		{"different_recipient", func() []any {
			return []any{r79Agent("MESSAGE", "/root/a", "/root", "", true), r79Agent("FINAL_ANSWER", "/root/a", "/other", "done", false), r79User()}
		}},
		{"empty_final", func() []any {
			return []any{r79Agent("MESSAGE", "/root/a", "/root", "", true), r79Agent("FINAL_ANSWER", "/root/a", "/root", " \n", false), r79User()}
		}},
		{"encrypted_final", func() []any {
			return []any{r79Agent("MESSAGE", "/root/a", "/root", "", true), r79Agent("FINAL_ANSWER", "/root/a", "/root", "partial", true), r79User()}
		}},
		{"task_reused", func() []any {
			return []any{r79Agent("MESSAGE", "/root/a", "/root", "", true),
				r79Agent("NEW_TASK", "/root", "/root/a", "next", false),
				r79Agent("FINAL_ANSWER", "/root/a", "/root", "unrelated", false), r79User()}
		}},
		{"new_encrypted_task", func() []any {
			return []any{r79Agent("NEW_TASK", "/root", "/root/a", "", true),
				r79Agent("FINAL_ANSWER", "/root/a", "/root", "done", false), r79User()}
		}},
		{"current_turn", func() []any {
			return []any{r79User(), r79Agent("MESSAGE", "/root/a", "/root", "", true), r79Agent("FINAL_ANSWER", "/root/a", "/root", "done", false)}
		}},
		{"missing_envelope", func() []any {
			item := r79Agent("MESSAGE", "/root/a", "/root", "", true)
			item["content"].([]any)[0].(map[string]any)["text"] = "an unrelated readable sibling"
			return []any{item, r79Agent("FINAL_ANSWER", "/root/a", "/root", "done", false), r79User()}
		}},
		{"ordinary_message", func() []any {
			item := r79Agent("MESSAGE", "/root/a", "/root", "", true)
			item["type"], item["role"] = "message", "user"
			return []any{item, r79Agent("FINAL_ANSWER", "/root/a", "/root", "done", false), r79User()}
		}},
		{"mismatched_envelope", func() []any {
			item := r79Agent("MESSAGE", "/root/a", "/root", "", true)
			item["author"] = "/root/b"
			return []any{item, r79Agent("FINAL_ANSWER", "/root/b", "/root", "done", false), r79User()}
		}},
		{"malformed_boundary", func() []any {
			malformed := r79Agent("MESSAGE", "/root/a", "/root", "", false)
			malformed["content"] = "unknown message"
			return []any{r79Agent("MESSAGE", "/root/a", "/root", "", true), malformed,
				r79Agent("FINAL_ANSWER", "/root/a", "/root", "done", false), r79User()}
		}},
		{"malformed_new_task_boundary", func() []any {
			malformed := r79Agent("NEW_TASK", "/root", "/root/a", "", false)
			malformed["content"] = "unknown task"
			return []any{r79Agent("MESSAGE", "/root/a", "/root", "", true), malformed,
				r79Agent("FINAL_ANSWER", "/root/a", "/root", "unrelated", false), r79User()}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := r79ReplayBody(t, tt.build()...)
			before := append([]byte(nil), body...)
			_, _, err := patchGrokResponsesBodyWithClientTools(body, "grok-4.7")
			require.ErrorContains(t, err, "opaque encrypted message content")
			var inputErr *grokModelInputError
			require.ErrorAs(t, err, &inputErr)
			require.Equal(t, before, body)
		})
	}
}

func TestR79MultipleIntermediateMessagesAndInterleavedAgents(t *testing.T) {
	first := r79Agent("MESSAGE", "/root/a", "/root", "first", true)
	first["content"].([]any)[0].(map[string]any)["text"] = strings.ReplaceAll(
		first["content"].([]any)[0].(map[string]any)["text"].(string), "\n", "\r\n")
	body := r79ReplayBody(t, first,
		r79Agent("MESSAGE", "/root/b", "/root", "other", true),
		r79Agent("MESSAGE", "/root/a", "/root", "second", true),
		r79Agent("FINAL_ANSWER", "/root/a", "/root", "RESULT_A", false),
		r79Agent("FINAL_ANSWER", "/root/b", "/root", "RESULT_B", false), r79User())
	got, _, err := patchGrokResponsesBodyWithClientTools(body, "grok-4.7")
	require.NoError(t, err)
	require.Equal(t, 3, strings.Count(string(got), grokCompletedAgentHistoryNotice))
	require.NotContains(t, string(got), r79OpaquePayload())
	require.Contains(t, string(got), "RESULT_A")
	require.Contains(t, string(got), "RESULT_B")
}

func TestR79ForwardCompletedAgentHistory(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "json", true: "stream"}[stream], func(t *testing.T) {
			body := r79ReplayBody(t,
				map[string]any{"type": "reasoning", "summary": []any{}, "encrypted_content": "native-grok-reasoning"},
				r79Agent("MESSAGE", "/root/a", "/root", "", true),
				r79Agent("FINAL_ANSWER", "/root/a", "/root", "FINAL_RESULT_MARKER", false), r79User())
			before := append([]byte(nil), body...)
			result := `{"id":"resp_r79","object":"response","status":"completed","model":"grok-4.7","output":[{"id":"msg_r79","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"FINAL_RESULT_MARKER"}]}],"usage":{"input_tokens":4,"output_tokens":2}}`
			contentType := "application/json"
			if stream {
				contentType = "text/event-stream"
				result = "data: {\"type\":\"response.completed\",\"response\":" + result + "}\n\n"
			}
			upstream := &httpUpstreamRecorder{responses: []*http.Response{{
				StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {contentType}},
				Body: io.NopCloser(strings.NewReader(result)),
			}}}
			svc := &OpenAIGatewayService{httpUpstream: upstream}
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
			account := &Account{ID: 79001, Platform: PlatformGrok, Type: AccountTypeAPIKey,
				Status: StatusActive, Schedulable: true, Concurrency: 2,
				Credentials: map[string]any{"api_key": "test-placeholder", "base_url": "https://api.x.ai/v1"}}
			forwarded, err := svc.forwardGrokResponses(context.Background(), c, account, body, "grok-4.7", stream, time.Now())
			require.NoError(t, err)
			require.NotNil(t, forwarded)
			require.Len(t, upstream.bodies, 1)
			require.NotContains(t, string(upstream.bodies[0]), r79OpaquePayload())
			require.Equal(t, "native-grok-reasoning", gjson.GetBytes(upstream.bodies[0], "input.0.encrypted_content").String())
			require.Contains(t, recorder.Body.String(), "FINAL_RESULT_MARKER")
			require.Equal(t, before, body)
		})
	}
}

func TestR79UnrecoverableInputHasCorrectErrorParameter(t *testing.T) {
	body := r79ReplayBody(t, r79Agent("MESSAGE", "/root/a", "/root", "", true), r79User())
	upstream := &httpUpstreamRecorder{}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	account := &Account{ID: 79001, Platform: PlatformGrok, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "test-placeholder", "base_url": "https://api.x.ai/v1"}}
	_, err := svc.forwardGrokResponses(context.Background(), c, account, body, "grok-4.7", false, time.Now())
	require.Error(t, err)
	require.Empty(t, upstream.bodies)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, "input", gjson.GetBytes(recorder.Body.Bytes(), "error.param").String())
}

// Private replay fixtures are deliberately external to the repository.
func TestR79PrivateHistoryReplay(t *testing.T) {
	path := os.Getenv("SUB2API_GROK_REPLAY_FILE")
	if path == "" {
		t.Skip("private replay not configured")
	}
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	before := append([]byte(nil), body...)
	got, _, err := patchGrokResponsesBodyWithClientTools(body, "grok-4.7")
	require.NoError(t, err)
	require.True(t, bytes.Equal(before, body), "source history must remain unchanged")
	var original, patched map[string]any
	require.NoError(t, json.Unmarshal(body, &original))
	require.NoError(t, json.Unmarshal(got, &patched))
	// Compare text content in memory without displaying private messages.
	outputStrings := make(map[string]int)
	var visit func(any, func(string))
	visit = func(value any, take func(string)) {
		switch v := value.(type) {
		case string:
			take(v)
		case []any:
			for _, x := range v {
				visit(x, take)
			}
		case map[string]any:
			for _, x := range v {
				visit(x, take)
			}
		}
	}
	visit(patched["input"], func(s string) { outputStrings[s]++ })
	readableCount, opaqueCount := 0, 0
	for _, raw := range original["input"].([]any) {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if kind := grokStringValue(item["type"]); kind != "message" && kind != "agent_message" {
			continue
		}
		if content, ok := item["content"].(string); ok && strings.TrimSpace(content) != "" {
			require.True(t, outputStrings[content] > 0, "readable message missing")
			readableCount++
		}
		parts, _ := item["content"].([]any)
		for _, rawPart := range parts {
			part, ok := rawPart.(map[string]any)
			if !ok {
				continue
			}
			if text := grokStringValue(part["text"]); strings.TrimSpace(text) != "" {
				require.True(t, outputStrings[text] > 0, "readable content part missing")
				readableCount++
			}
			if encrypted := grokStringValue(part["encrypted_content"]); isGrokOpaqueMessageCiphertext(encrypted) {
				require.False(t, bytes.Contains(got, []byte(encrypted)), "opaque agent payload reached Grok")
				opaqueCount++
			}
		}
	}
	require.Positive(t, opaqueCount, "fixture must exercise genuine encrypted history")
	require.Equal(t, opaqueCount, outputStrings[grokCompletedAgentHistoryNotice])
	t.Logf("readable messages/parts preserved: %d; opaque agent parts marked: %d; original bytes unchanged: true", readableCount, opaqueCount)
}

func TestR79PrivateFullForwardAndEncryptedReasoningRetry(t *testing.T) {
	path := os.Getenv("SUB2API_GROK_REPLAY_FILE")
	if path == "" {
		t.Skip("private replay not configured")
	}
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	before := append([]byte(nil), body...)
	result := `{"id":"resp_recovery","object":"response","status":"completed","model":"grok-4.7","output":[{"id":"msg_recovery","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"REPLAY_OK"}]}],"usage":{"input_tokens":4,"output_tokens":2}}`
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		{StatusCode: http.StatusBadRequest, Header: http.Header{"Content-Type": {"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{"error":{"code":"invalid_encrypted_content","message":"Could not decrypt the provided encrypted_content."}}`))},
		{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}},
			Body: io.NopCloser(strings.NewReader("data: {\"type\":\"response.completed\",\"response\":" + result + "}\n\n"))},
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	account := &Account{ID: 79002, Platform: PlatformGrok, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 2,
		Credentials: map[string]any{"api_key": "test-placeholder", "base_url": "https://api.x.ai/v1"}}
	forwarded, err := svc.forwardGrokResponses(context.Background(), c, account, body, "grok-4.7", true, time.Now())
	require.NoError(t, err)
	require.NotNil(t, forwarded)
	require.Len(t, upstream.bodies, 2, "one bounded reasoning-decode retry")
	require.True(t, bytes.Equal(before, body), "source history changed")
	require.Contains(t, recorder.Body.String(), "REPLAY_OK")
	sourceItems := gjson.GetBytes(body, "input").Array()
	sentItems := gjson.GetBytes(upstream.bodies[1], "input").Array()
	var wantContent, gotContent any
	require.NoError(t, json.Unmarshal([]byte(sourceItems[len(sourceItems)-1].Get("content").Raw), &wantContent))
	require.NoError(t, json.Unmarshal([]byte(sentItems[len(sentItems)-1].Get("content").Raw), &gotContent))
	wantJSON, _ := json.Marshal(wantContent)
	gotJSON, _ := json.Marshal(gotContent)
	require.True(t, bytes.Equal(wantJSON, gotJSON), "latest user message or image changed")
	for _, sent := range upstream.bodies {
		agentAnswers := 0
		for _, item := range gjson.GetBytes(sent, "input").Array() {
			if strings.Contains(item.Get("content").Raw, "Message Type: FINAL_ANSWER") {
				agentAnswers++
			}
		}
		require.Positive(t, agentAnswers, "readable final answer missing")
		require.Contains(t, string(sent), grokCompletedAgentHistoryNotice)
	}
	t.Logf("private full forward completed; one encrypted-reasoning retry; latest text/image and final agent answer retained")
}
