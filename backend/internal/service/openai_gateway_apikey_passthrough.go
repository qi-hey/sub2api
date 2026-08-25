package service

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func ensureOpenAIAPIKeyPassthroughCodexHeaders(c *gin.Context, req *http.Request) {
	if req == nil {
		return
	}
	userAgent := req.Header.Get("user-agent")
	originator := req.Header.Get("originator")
	if c != nil {
		if userAgent == "" {
			userAgent = c.GetHeader("user-agent")
		}
		if originator == "" {
			originator = c.GetHeader("originator")
		}
	}
	if !openai.IsCodexOfficialClientByHeaders(userAgent, originator) {
		return
	}
	if req.Header.Get("originator") == "" {
		req.Header.Set("originator", "codex_cli_rs")
	}
	if req.Header.Get("version") == "" {
		if parsed, ok := openai.ParseCodexEngineVersion(userAgent); ok && strings.TrimSpace(parsed) != "" {
			req.Header.Set("version", strings.TrimSpace(parsed))
		} else {
			req.Header.Set("version", codexCLIVersion)
		}
	}
	if req.Header.Get("openai-beta") == "" {
		req.Header.Set("openai-beta", "responses=experimental")
	}
}

func ensureOpenAIAPIKeyPassthroughCodexBody(body []byte) ([]byte, bool, error) {
	if len(body) == 0 || !gjson.GetBytes(body, "reasoning").Exists() {
		return body, false, nil
	}
	var reqBody map[string]any
	if err := json.Unmarshal(body, &reqBody); err != nil {
		return body, false, err
	}
	if !ensureCodexReasoningInclude(reqBody) {
		return body, false, nil
	}
	updated, err := marshalOpenAIUpstreamJSON(reqBody)
	if err != nil {
		return body, false, err
	}
	return updated, true, nil
}
