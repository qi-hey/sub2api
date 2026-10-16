package service

import (
	"fmt"
	"strings"
)

type grokModelInputError struct {
	index int
	cause error
}

func (e *grokModelInputError) Error() string {
	return fmt.Sprintf("Grok input[%d]: %v", e.index, e.cause)
}

func (e *grokModelInputError) Unwrap() error { return e.cause }

const grokCompletedAgentHistoryNotice = "[Grok history compatibility: this earlier agent message contains OpenAI-encrypted content that Grok cannot decrypt. The readable FINAL_ANSWER from the same agent is retained below. The encrypted text was not recovered; use the readable result and conversation for context.]"

type grokAgentHistoryRoute struct {
	author    string
	recipient string
}

// A finished agent's intermediate message can contain an opaque payload even
// when its final answer is plaintext. Keep the envelope and final answer, and
// explicitly mark the unavailable intermediate text on Grok egress only.
// Active tasks, unmatched replies and ordinary encrypted messages still fail.
func recoverGrokCompletedAgentHistory(items []any) {
	lastUser := -1
	for i, raw := range items {
		item, ok := raw.(map[string]any)
		if ok && grokStringValue(item["role"]) == "user" &&
			(grokStringValue(item["type"]) == "message" || grokStringValue(item["type"]) == "") {
			lastUser = i
		}
	}
	pending := make(map[grokAgentHistoryRoute][]map[string]any)
	for i, raw := range items {
		if i >= lastUser {
			break
		}
		item, ok := raw.(map[string]any)
		if !ok || grokStringValue(item["type"]) != "agent_message" {
			continue
		}
		route := grokAgentHistoryRoute{
			author:    strings.TrimSpace(grokStringValue(item["author"])),
			recipient: strings.TrimSpace(grokStringValue(item["recipient"])),
		}
		if route.author == "" || route.recipient == "" || route.author == route.recipient {
			continue
		}
		kind, payload, opaque, valid := grokAgentHistoryEnvelope(item, route)
		if !valid {
			// Do not match across a message whose envelope cannot be interpreted.
			delete(pending, route)
			delete(pending, grokAgentHistoryRoute{author: route.recipient, recipient: route.author})
			continue
		}
		switch kind {
		case "NEW_TASK":
			delete(pending, route)
			delete(pending, grokAgentHistoryRoute{author: route.recipient, recipient: route.author})
		case "MESSAGE":
			if opaque {
				pending[route] = append(pending[route], item)
			}
		case "FINAL_ANSWER":
			if !opaque && strings.TrimSpace(payload) != "" {
				for _, earlier := range pending[route] {
					parts, _ := earlier["content"].([]any)
					for j, value := range parts {
						part, ok := value.(map[string]any)
						if ok && grokStringValue(part["type"]) == "encrypted_content" &&
							isGrokOpaqueMessageCiphertext(grokStringValue(part["encrypted_content"])) {
							parts[j] = map[string]any{"type": "input_text", "text": grokCompletedAgentHistoryNotice}
						}
					}
				}
			}
			delete(pending, route)
		default:
			delete(pending, route)
		}
	}
}

func grokAgentHistoryEnvelope(item map[string]any, route grokAgentHistoryRoute) (kind, payload string, opaque, valid bool) {
	parts, ok := item["content"].([]any)
	if !ok {
		return "", "", false, false
	}
	var text strings.Builder
	for _, value := range parts {
		part, ok := value.(map[string]any)
		if !ok {
			return "", "", false, false
		}
		switch grokStringValue(part["type"]) {
		case "input_text", "output_text", "text":
			text.WriteString(grokStringValue(part["text"]))
		case "encrypted_content":
			value, ok := part["encrypted_content"].(string)
			if !ok || !isGrokOpaqueMessageCiphertext(value) {
				// A legacy plaintext carrier already works in the normal adapter.
				// Do not use a partial final answer to replace opaque history.
				return "", "", false, false
			}
			opaque = true
		default:
			return "", "", false, false
		}
	}
	header, payload, found := strings.Cut(strings.ReplaceAll(text.String(), "\r\n", "\n"), "\nPayload:\n")
	if !found {
		return "", "", opaque, false
	}
	lines := strings.Split(header, "\n")
	if len(lines) != 3 || lines[1] != "Task name: "+route.recipient || lines[2] != "Sender: "+route.author {
		return "", "", opaque, false
	}
	kind, found = strings.CutPrefix(lines[0], "Message Type: ")
	return kind, payload, opaque, found
}
