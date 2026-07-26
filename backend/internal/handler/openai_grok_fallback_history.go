package handler

import (
	"bytes"
	"encoding/json"
	"strings"
)

type fallbackHistoryMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// A fresh provider fallback is allowed only before any assistant/tool history
// exists. Provider continuity is handled separately by the route-owner marker.
func openAIResponsesAllowsFreshGrokFallback(body []byte) bool {
	var req struct {
		Type               string          `json:"type"`
		PreviousResponseID string          `json:"previous_response_id"`
		Input              json.RawMessage `json:"input"`
	}
	if json.Unmarshal(body, &req) != nil || strings.TrimSpace(req.PreviousResponseID) != "" {
		return false
	}
	trimmed := bytes.TrimSpace(req.Input)
	if len(trimmed) == 0 {
		// A Responses WebSocket v2 connection may start with an empty
		// response.create frame. With no previous_response_id, it has no
		// provider-owned history and is safe to route as a fresh request.
		return strings.EqualFold(strings.TrimSpace(req.Type), "response.create")
	}
	var text string
	if json.Unmarshal(trimmed, &text) == nil {
		return strings.TrimSpace(text) != ""
	}
	var items []map[string]any
	if json.Unmarshal(trimmed, &items) != nil {
		return false
	}
	userMessages := 0
	for _, item := range items {
		typ := strings.ToLower(strings.TrimSpace(stringValue(item["type"])))
		role := strings.ToLower(strings.TrimSpace(stringValue(item["role"])))
		if typ != "" && typ != "message" {
			return false
		}
		switch role {
		case "developer", "system":
		case "user":
			userMessages++
		default:
			return false
		}
	}
	return userMessages == 1
}

func openAIChatAllowsFreshGrokFallback(body []byte) bool {
	var req struct {
		Messages []fallbackHistoryMessage `json:"messages"`
	}
	if json.Unmarshal(body, &req) != nil {
		return false
	}
	return messagesAllowFreshGrokFallback(req.Messages, false)
}

func anthropicMessagesAllowFreshGrokFallback(body []byte) bool {
	var req struct {
		Messages []fallbackHistoryMessage `json:"messages"`
	}
	if json.Unmarshal(body, &req) != nil {
		return false
	}
	return messagesAllowFreshGrokFallback(req.Messages, true)
}

func messagesAllowFreshGrokFallback(messages []fallbackHistoryMessage, rejectToolResults bool) bool {
	userMessages := 0
	for _, message := range messages {
		switch strings.ToLower(strings.TrimSpace(message.Role)) {
		case "system", "developer":
		case "user":
			userMessages++
			if rejectToolResults && bytes.Contains(message.Content, []byte(`"tool_result"`)) {
				return false
			}
		default:
			return false
		}
	}
	return userMessages == 1
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}
