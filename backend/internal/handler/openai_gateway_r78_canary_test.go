package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

func TestR78CanaryDelegationPassesHTTPIngress(t *testing.T) {
	body := []byte(`{"model":"grok-4.7","input":[
		{"type":"agent_message","author":"/root","recipient":"/root/worker","content":[
			{"type":"input_text","text":"Message Type: NEW_TASK\nPayload:\n"},
			{"type":"encrypted_content","encrypted_content":"R78-GROK47-OK"}
		]},
		{"type":"function_call_output","name":"send_message_to_thread","namespace":"codex_app",
		 "output":"<codex_delegation><source_thread_id>r78-fixture</source_thread_id><input>Return the delegated task marker exactly.</input></codex_delegation>"}
	]}`)
	normalized, changed := normalizeCodexDelegationBootstrap(body)
	require.True(t, changed)
	require.Equal(t, "R78-GROK47-OK", gjson.GetBytes(normalized, "input.0.content.1.encrypted_content").String())
	require.Equal(t, "user", gjson.GetBytes(normalized, "input.1.role").String())
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	handler := &OpenAIGatewayHandler{}
	require.True(t, handler.validateFunctionCallOutputRequest(c, normalized, zap.NewNop()))
	require.False(t, c.Writer.Written())
}
