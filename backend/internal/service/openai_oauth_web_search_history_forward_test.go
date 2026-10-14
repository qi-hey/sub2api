//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// TestForward_OAuthWebSearchHistoryDeclaresTool drives the Codex compaction
// shape (web_search_call history, tools:[]) through Forward and asserts the
// body reaching chatgpt.com declares web_search (#7927): top-level for the
// standard wire, input additional_tools for Responses Lite.
func TestForward_OAuthWebSearchHistoryDeclaresTool(t *testing.T) {
	for _, tc := range []struct {
		name        string
		passthrough bool
		lite        bool
		fingerprint bool
	}{
		{"transform", false, false, false},
		{"passthrough", true, false, false},
		{"transform lite", false, true, false},
		{"passthrough lite", true, true, false},
		{"balanced transform", false, false, true},
		{"balanced passthrough", true, false, true},
		{"balanced transform lite", false, true, true},
		{"balanced passthrough lite", true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			s := newAstraOAuthSetup(t, tc.passthrough)
			if tc.lite {
				s.c.Request.Header.Set(responsesLiteHeader, "true")
			}
			var expectedIDs *codexFingerprintIDs
			if tc.fingerprint {
				if s.account.Extra == nil {
					s.account.Extra = make(map[string]any)
				}
				s.account.Extra[codexFingerprintModeExtraKey] = "session"
				s.account.Extra[codexFingerprintSeedExtraKey] = userSuppliedCodexFingerprintSeed
				s.c.Request.Header.Set("session-id", "r77-history-regression")
				expectedIDs = resolveCodexFingerprintIDsFromRequest(s.account, s.c.Request.Header)
				require.NotNil(t, expectedIDs)
			}
			inner := `{"id":"resp_test","model":"gpt-5.5","output":[],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`
			s.upstream.resp = &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:       io.NopCloser(strings.NewReader(codexCompletedSSE(inner))),
			}
			body, err := sjson.SetRawBytes([]byte(openAIWebSearchHistoryCompactionBody), "input.-1", []byte(`{"type":"compaction_trigger"}`))
			require.NoError(t, err)

			result, err := s.svc.Forward(context.Background(), s.c, s.account, body)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, s.upstream.lastReq)
			require.Equal(t, astraProCodexResponsesURL, s.upstream.lastReq.URL.String())
			if expectedIDs != nil {
				require.Equal(t, expectedIDs.installationID, s.upstream.lastReq.Header.Get("x-codex-installation-id"))
				require.Equal(t, expectedIDs.sessionID, s.upstream.lastReq.Header.Get("session-id"))
				require.Equal(t, expectedIDs.threadID, s.upstream.lastReq.Header.Get("thread-id"))
				require.Equal(t, userSuppliedCodexFingerprintSeed, s.account.Extra[codexFingerprintSeedExtraKey])
				require.Equal(t, "session", s.account.Extra[codexFingerprintModeExtraKey])
			}

			forwarded := s.upstream.lastBody
			require.Equal(t, "web_search_call", gjson.GetBytes(forwarded, "input.1.type").String())
			require.Equal(t, "none", gjson.GetBytes(forwarded, "tool_choice").String())
			items := gjson.GetBytes(forwarded, "input").Array()
			require.Equal(t, "compaction_trigger", items[len(items)-1].Get("type").String())
			if !tc.lite {
				tools := gjson.GetBytes(forwarded, "tools").Array()
				require.Len(t, tools, 1)
				require.Equal(t, "web_search", tools[0].Get("type").String())
				return
			}
			require.False(t, gjsonToolsContainWebSearch(gjson.GetBytes(forwarded, "tools")))
			additional := items[len(items)-2]
			require.Equal(t, "additional_tools", additional.Get("type").String())
			require.Equal(t, "web_search", additional.Get("tools.0.type").String())
		})
	}
}
