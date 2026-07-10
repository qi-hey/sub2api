package service

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

const openAIPassthroughOutboundShapeDebugKey = "openai_passthrough_outbound_shape_debug"

func shouldForceOpenAIPassthroughUpstreamStream(account *Account, model string, clientStream bool) bool {
	if clientStream {
		return false
	}
	return shouldUseAnyRouterOpenAIPassthroughCodexShape(account, model)
}

func shouldUseAnyRouterOpenAIPassthroughCodexShape(account *Account, model string) bool {
	if account == nil || account.Type != AccountTypeAPIKey || !account.IsPoolMode() {
		return false
	}
	baseURL := strings.ToLower(strings.TrimSpace(account.GetOpenAIBaseURL()))
	if !strings.Contains(baseURL, "anyrouter.top") {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(model), "gpt-5.5")
}

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

func ensureAnyRouterOpenAIPassthroughCodexBody(c *gin.Context, body []byte) ([]byte, bool, error) {
	if len(body) == 0 {
		return body, false, nil
	}
	var reqBody map[string]any
	if err := json.Unmarshal(body, &reqBody); err != nil {
		return body, false, err
	}

	changed := ensureOpenAICodexEncryptedReasoningInclude(reqBody)
	if promptCacheKey, _ := reqBody["prompt_cache_key"].(string); strings.TrimSpace(promptCacheKey) == "" {
		reqBody["prompt_cache_key"] = resolveOpenAIPassthroughPromptCacheKey(c)
		changed = true
	}
	for _, field := range []string{
		"metadata",
		"previous_response_id",
		"prompt_cache_retention",
		"truncation",
	} {
		if _, ok := reqBody[field]; ok {
			delete(reqBody, field)
			changed = true
		}
	}
	if trimOpenAIEncryptedReasoningItems(reqBody) {
		changed = true
	}
	if !changed {
		return body, false, nil
	}
	updated, err := marshalOpenAIUpstreamJSON(reqBody)
	if err != nil {
		return body, false, err
	}
	return updated, true, nil
}

func ensureOpenAICodexEncryptedReasoningInclude(reqBody map[string]any) bool {
	if len(reqBody) == 0 {
		return false
	}
	const encrypted = "reasoning.encrypted_content"
	switch existing := reqBody["include"].(type) {
	case nil:
		reqBody["include"] = []any{encrypted}
		return true
	case []any:
		for _, v := range existing {
			if s, ok := v.(string); ok && s == encrypted {
				return false
			}
		}
		reqBody["include"] = append(existing, encrypted)
		return true
	case []string:
		for _, v := range existing {
			if v == encrypted {
				return false
			}
		}
		reqBody["include"] = append(existing, encrypted)
		return true
	default:
		return false
	}
}

func resolveOpenAIPassthroughPromptCacheKey(c *gin.Context) string {
	if c != nil {
		for _, key := range []string{
			"prompt_cache_key",
			"session_id",
			"conversation_id",
			"x-client-request-id",
			"x-request-id",
		} {
			if value := strings.TrimSpace(c.GetHeader(key)); value != "" {
				return value
			}
		}
	}
	return uuid.NewString()
}

func shouldFailoverOpenAIPassthroughPoolModeResponse(statusCode int, body []byte) (bool, bool) {
	if len(body) == 0 {
		return false, false
	}
	upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(body))
	lowerMsg := strings.ToLower(upstreamMsg)
	lowerBody := strings.ToLower(string(body))

	if statusCode == http.StatusBadRequest {
		if strings.Contains(lowerMsg, "invalid codex request") ||
			strings.Contains(lowerMsg, "\u65e0\u6548\u4ee3\u7801\u8bf7\u6c42") ||
			strings.Contains(lowerBody, "invalid_codex_request") ||
			strings.Contains(lowerBody, "invalid codex request") ||
			strings.Contains(lowerBody, "invalid_responses_request") ||
			strings.Contains(lowerBody, "\u65e0\u6548_responses_request") {
			return true, true
		}
	}

	if statusCode >= http.StatusInternalServerError && statusCode <= http.StatusGatewayTimeout {
		if strings.Contains(lowerBody, "get_channel_failed") ||
			strings.Contains(lowerMsg, "selected model is at capacity") ||
			strings.Contains(lowerMsg, "\u8d1f\u8f7d\u5df2\u7ecf\u8fbe\u5230\u4e0a\u9650") ||
			strings.Contains(lowerMsg, "model") && strings.Contains(lowerMsg, "capacity") ||
			strings.Contains(lowerMsg, "load") && strings.Contains(lowerMsg, "limit") ||
			isOpenAITransientProcessingError(statusCode, upstreamMsg, body) {
			return true, true
		}
	}

	return false, false
}

func buildOpenAIPassthroughOutboundShapeDebug(account *Account, req *http.Request, body []byte) string {
	if account == nil || req == nil || !shouldUseAnyRouterOpenAIPassthroughCodexShape(account, gjson.GetBytes(body, "model").String()) {
		return ""
	}
	shape := map[string]any{
		"account_id":     account.ID,
		"account_name":   account.Name,
		"method":         req.Method,
		"url_path":       "",
		"accept":         req.Header.Get("accept"),
		"content_type":   req.Header.Get("content-type"),
		"user_agent":     req.Header.Get("user-agent"),
		"originator":     req.Header.Get("originator"),
		"version":        req.Header.Get("version"),
		"openai_beta":    req.Header.Get("openai-beta"),
		"content_length": req.ContentLength,
		"body_bytes":     len(body),
	}
	if req.URL != nil {
		shape["url_path"] = req.URL.Path
	}
	for key, value := range openAIPassthroughBodyShape(body) {
		shape[key] = value
	}
	encoded, err := json.Marshal(shape)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func openAIPassthroughBodyShape(body []byte) map[string]any {
	out := map[string]any{"json_valid": false}
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return out
	}
	out["json_valid"] = true
	out["model"] = gjson.GetBytes(body, "model").String()
	stream := gjson.GetBytes(body, "stream")
	out["stream_exists"] = stream.Exists()
	out["stream_type"] = stream.Type.String()
	out["stream_value"] = stream.Bool()
	input := gjson.GetBytes(body, "input")
	out["input_exists"] = input.Exists()
	out["input_type"] = input.Type.String()
	if input.IsArray() {
		out["input_count"] = len(input.Array())
	}
	topKeys := make([]string, 0)
	gjson.ParseBytes(body).ForEach(func(key, value gjson.Result) bool {
		topKeys = append(topKeys, key.String())
		return true
	})
	sort.Strings(topKeys)
	out["top_level_keys"] = topKeys
	for _, key := range []string{
		"include",
		"instructions",
		"metadata",
		"parallel_tool_calls",
		"previous_response_id",
		"prompt_cache_key",
		"prompt_cache_retention",
		"reasoning",
		"service_tier",
		"store",
		"text",
		"tools",
		"truncation",
	} {
		out["has_"+key] = gjson.GetBytes(body, key).Exists()
	}
	return out
}

func openAIPassthroughOutboundShapeDebugFromContext(c *gin.Context) string {
	if c == nil {
		return ""
	}
	v, ok := c.Get(openAIPassthroughOutboundShapeDebugKey)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func mergeOpenAIPassthroughOutboundShapeDetail(detail string, c *gin.Context) string {
	shape := openAIPassthroughOutboundShapeDebugFromContext(c)
	if shape == "" {
		return detail
	}
	payload := map[string]any{
		"outbound_shape": json.RawMessage(shape),
	}
	if strings.TrimSpace(detail) != "" {
		payload["upstream_error_body"] = detail
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return detail
	}
	return string(encoded)
}
