package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestR78ForwardGrok47NormalizesDelegationWithoutMutatingOriginal(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{true: "stream", false: "json"}[stream], func(t *testing.T) {
			body := []byte(`{"model":"grok-4.7","stream":false,"input":[
				{"type":"agent_message","author":"/root","recipient":"/root/worker","content":[
					{"type":"input_text","text":"Message Type: NEW_TASK\nPayload:\n"},
					{"type":"encrypted_content","encrypted_content":"R78_KEEP_TASK"}
				]},
				{"type":"function_call_output","name":"send_message_to_thread","namespace":"codex_app","output":"R78_KEEP_REPLY"}
			],"tools":[]}`)
			if stream {
				body = bytes.Replace(body, []byte(`"stream":false`), []byte(`"stream":true`), 1)
			}
			original := append([]byte(nil), body...)
			result := `{"id":"resp_r78","object":"response","status":"completed","model":"grok-4.7","output":[{"id":"msg_r78","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"R78_OK"}]}],"usage":{"input_tokens":4,"output_tokens":2}}`
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
			account := &Account{ID: 78001, Platform: PlatformGrok, Type: AccountTypeAPIKey,
				Status: StatusActive, Schedulable: true, Concurrency: 2,
				Credentials: map[string]any{"api_key": "test-placeholder", "base_url": "https://api.x.ai/v1"}}
			forwarded, err := svc.forwardGrokResponses(context.Background(), c, account, body, "grok-4.7", stream, time.Now())
			require.NoError(t, err)
			require.NotNil(t, forwarded)
			require.Len(t, upstream.bodies, 1)
			require.Equal(t, original, body)
			sent := upstream.bodies[0]
			require.Equal(t, "user", gjson.GetBytes(sent, "input.0.role").String())
			require.Equal(t, "R78_KEEP_TASK", gjson.GetBytes(sent, "input.0.content.1.text").String())
			require.Contains(t, gjson.GetBytes(sent, "input.1.content.0.text").String(), "R78_KEEP_REPLY")
			require.Equal(t, http.StatusOK, recorder.Code)
		})
	}
}

func auditGrokPatch(t *testing.T, body string) []byte {
	t.Helper()
	ingress, _, err := normalizeOpenAIResponsesLegacyIngress([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	ingress, _, err = sanitizeOpenAIResponsesToolSchemasForPlatform(ingress, PlatformGrok)
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := patchGrokResponsesBodyWithClientTools(ingress, "grok-4.6")
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestAuditGrokNestedEncryptedContent(t *testing.T) {
	out := auditGrokPatch(t, `{"model":"grok-4.6","input":[
		{"type":"message","role":"user","content":[
			{"type":"input_text","text":"Task envelope"},
			{"type":"encrypted_content","encrypted_content":"AUDIT_TASK_BODY"}
		]}
	]}`)
	if gjson.GetBytes(out, "input.0.content.1.type").String() == "encrypted_content" {
		t.Error("nested encrypted_content is still forwarded unchanged")
	}
	if !strings.Contains(string(out), "AUDIT_TASK_BODY") {
		t.Error("readable task body was lost")
	}
}

func TestAuditGrokDelegatedTaskRole(t *testing.T) {
	out := auditGrokPatch(t, `{"model":"grok-4.6","input":[
		{"type":"agent_message","author":"/root","recipient":"/root/worker","content":[
			{"type":"input_text","text":"Message Type: NEW_TASK\nTask name: /root/worker\nSender: /root\nPayload:\nAUDIT_TASK"}
		]}
	]}`)
	if role := gjson.GetBytes(out, "input.0.role").String(); role != "user" {
		t.Errorf("delegated NEW_TASK role = %q; task input became assistant history", role)
	}
}

func TestAuditGrokChatContentAliases(t *testing.T) {
	out := auditGrokPatch(t, `{"model":"grok-4.6","input":[
		{"role":"user","content":[
			{"type":"text","text":"AUDIT_IMAGE_QUESTION"},
			{"type":"image_url","image_url":{"url":"https://example.invalid/image.png","detail":"low"}}
		]}
	]}`)
	if typ := gjson.GetBytes(out, "input.0.content.0.type").String(); typ != "input_text" {
		t.Errorf("Chat text part was not converted: %q", typ)
	}
	if typ := gjson.GetBytes(out, "input.0.content.1.type").String(); typ != "input_image" {
		t.Errorf("Chat image part was not converted: %q", typ)
	}
}

func TestAuditGrokNamedExternalToolInput(t *testing.T) {
	out := auditGrokPatch(t, `{"model":"grok-4.6","input":[
		{"type":"function_call_output","name":"send_message_to_thread","namespace":"codex_app",
		 "output":"AUDIT_DELEGATED_REQUEST"}
	]}`)
	item := gjson.GetBytes(out, "input.0")
	if item.Get("type").String() == "function_call_output" &&
		item.Get("call_id").String() != "" &&
		!gjson.GetBytes(out, `input.#(type=="function_call")`).Exists() {
		t.Errorf("standalone named input became orphan output: call_id=%s name_present=%v namespace_present=%v",
			item.Get("call_id").String(), item.Get("name").Exists(), item.Get("namespace").Exists())
	}
}

func TestAuditGrokCustomToolOutputImages(t *testing.T) {
	for _, tools := range []string{`[]`, `[{"type":"custom","name":"inspect_image"}]`} {
		t.Run(tools, func(t *testing.T) {
			body := `{"model":"grok-4.6","tools":` + tools + `,"input":[
				{"type":"custom_tool_call","call_id":"call_img","name":"inspect_image","input":"inspect"},
				{"type":"custom_tool_call_output","call_id":"call_img","output":[
					{"type":"input_text","text":"AUDIT_IMAGE_RESULT"},
					{"type":"input_image","image_url":"https://example.invalid/result.png"}
				]}
			]}`
			out := auditGrokPatch(t, body)
			imageFound := false
			for _, item := range gjson.GetBytes(out, "input").Array() {
				for _, part := range item.Get("content").Array() {
					if part.Get("type").String() == "input_image" {
						imageFound = true
					}
				}
			}
			if !imageFound {
				t.Error("custom tool image result was stringified, with no visible input_image")
			}
		})
	}
}

func TestAuditGrokPreservesToolSchemaPropertyNames(t *testing.T) {
	out := auditGrokPatch(t, `{"model":"grok-4.6","input":"AUDIT","tools":[
		{"type":"function","name":"configure_network","parameters":{
			"type":"object",
			"properties":{"external_web_access":{"type":"boolean"}},
			"required":["external_web_access"],
			"additionalProperties":false
		}}
	]}`)
	if !gjson.GetBytes(out, "tools.0.parameters.properties.external_web_access").Exists() {
		t.Error("required property external_web_access was deleted from the function schema")
	}
	if gjson.GetBytes(out, "tools.0.parameters.required.0").String() != "external_web_access" {
		t.Error("required list changed unexpectedly")
	}
}

func TestAuditGrokUnionSchemaSemantics(t *testing.T) {
	schema := `{"oneOf":[
		{"anyOf":[
			{"type":"object","properties":{"mode":{"const":"create"},"a":{"type":"string"}},"required":["mode","a"]},
			{"type":"object","properties":{"mode":{"const":"create"},"b":{"type":"string"}},"required":["mode","b"]}
		]},
		{"type":"object","properties":{"mode":{"const":"delete"}},"required":["mode"]}
	]}`
	out := auditGrokPatch(t, `{"model":"grok-4.6","input":"AUDIT","tools":[
		{"type":"function","name":"configure","parameters":`+schema+`}
	]}`)
	fixture := map[string]json.RawMessage{
		"original":   json.RawMessage(schema),
		"normalized": json.RawMessage(gjson.GetBytes(out, "tools.0.parameters").Raw),
		"arguments":  json.RawMessage(`{"mode":"create","a":"A","b":"B"}`),
	}
	encoded, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("AUDIT_UNION_VALIDATION=" + string(encoded))
	if gjson.GetBytes(out, "tools.0.parameters.oneOf.#").Int() == 3 &&
		!gjson.GetBytes(out, "tools.0.parameters.oneOf.0.anyOf").Exists() {
		t.Error("nested anyOf was flattened into outer oneOf, changing exclusivity")
	}
}

func TestAuditGrokFlatStringError(t *testing.T) {
	body := []byte(`{"error":"Failed to deserialize: unknown content type encrypted_content"}`)
	if got := extractGrokUpstreamErrorMessage(body); got == "" {
		t.Error("flat xAI error string is discarded by the displayed error extractor")
	}
}

func TestR78OpenAISharedAdapterUnchanged(t *testing.T) {
	body := []byte(`{"model":"gpt-6-astra","service_tier":"priority","reasoning":{"effort":"ultra"},
		"input":[{"type":"function_call_output","name":"send_message_to_thread","namespace":"codex_app","output":"task"},
		{"type":"custom_tool_call_output","call_id":"call_image","output":[{"type":"input_image","image_url":"https://example.invalid/i.png"}]}],
		"tools":[]}`)
	var original map[string]any
	require.NoError(t, decodeOpenAIJSONUseNumber(body, &original))
	_, _, err := apicompat.AdaptResponsesClientToolsWithInheritedMapping(original, apicompat.ResponsesClientToolMapping{})
	require.NoError(t, err)
	lowerGrokResponsesCustomToolHistory(original["input"])
	want, err := marshalOpenAIUpstreamJSON(original)
	require.NoError(t, err)
	got, _, err := adaptResponsesClientToolsForFunctionUpstreamWithMapping(body, "OpenAI WS HTTP bridge", apicompat.ResponsesClientToolMapping{})
	require.NoError(t, err)
	require.JSONEq(t, string(want), string(got))
	require.Equal(t, "priority", gjson.GetBytes(got, "service_tier").String())
	require.Equal(t, "ultra", gjson.GetBytes(got, "reasoning.effort").String())
}

func TestR78OpaqueMessageFailsExplicitlyWithoutDroppingHistory(t *testing.T) {
	cipher := make([]byte, 73)
	cipher[0] = 0x80
	encoded := base64.URLEncoding.EncodeToString(cipher)
	body, err := json.Marshal(map[string]any{
		"input": []any{map[string]any{"type": "message", "role": "user", "content": []any{
			map[string]any{"type": "encrypted_content", "encrypted_content": encoded},
		}}},
	})
	require.NoError(t, err)
	before := append([]byte(nil), body...)
	_, _, err = patchGrokResponsesBodyWithClientTools(body, "grok-4.7")
	require.ErrorContains(t, err, "opaque encrypted message content")
	require.Equal(t, before, body)
}

func TestR78Grok47AndNativeHistoryPreserved(t *testing.T) {
	body := []byte(`{"model":"grok-4.6","reasoning":{"effort":"xhigh"},"input":[
		{"type":"reasoning","summary":[{"type":"summary_text","text":"keep"}],"encrypted_content":"native-state"},
		{"type":"message","role":"user","content":"continue"}
	]}`)
	before := append([]byte(nil), body...)
	got, _, err := patchGrokResponsesBodyWithClientTools(body, "grok-4.7")
	require.NoError(t, err)
	require.Equal(t, "grok-4.7", gjson.GetBytes(got, "model").String())
	require.Equal(t, "xhigh", gjson.GetBytes(got, "reasoning.effort").String())
	require.Equal(t, "native-state", gjson.GetBytes(got, "input.0.encrypted_content").String())
	require.Equal(t, before, body)
}

func TestAuditGrokSyntheticIDsAvoidExistingIDs(t *testing.T) {
	out := auditGrokPatch(t, `{"model":"grok-4.6","input":[
		{"type":"function_call","call_id":"grok_replay_call_1","name":"first","arguments":"{}"},
		{"type":"function_call_output","call_id":"grok_replay_call_1","output":"first result"},
		{"type":"function_call","name":"second","arguments":"{}"},
		{"type":"function_call_output","output":"second result"}
	]}`)
	first := gjson.GetBytes(out, "input.0.call_id").String()
	second := gjson.GetBytes(out, "input.2.call_id").String()
	if first == second {
		t.Errorf("generated call_id collides with an existing ID: %q", first)
	}
}

func TestAuditGrokSupportedBaseline(t *testing.T) {
	out := auditGrokPatch(t, `{"model":"grok-4.6","reasoning":{"effort":"xhigh"},"input":[
		{"type":"message","role":"user","content":[{"type":"input_text","text":"AUDIT_KEEP"}]},
		{"type":"function_call","call_id":"call_ok","name":"lookup","arguments":"{}"},
		{"type":"function_call_output","call_id":"call_ok","output":"AUDIT_RESULT"},
		{"type":"message","role":"user","content":"continue"}
	],"tools":[{"type":"function","name":"lookup","parameters":{"type":"object","properties":{}}}]}`)
	if gjson.GetBytes(out, "reasoning.effort").String() != "xhigh" ||
		gjson.GetBytes(out, "input.1.call_id").String() != "call_ok" ||
		gjson.GetBytes(out, "input.2.output").String() != "AUDIT_RESULT" {
		t.Fatal("supported baseline unexpectedly changed")
	}
}
