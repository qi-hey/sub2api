package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	grokResponsesPromptHardLimit = 500_000
	grokResponsesPromptReserve   = 32_000
	grokResponsesPromptBudget    = grokResponsesPromptHardLimit - grokResponsesPromptReserve
)

type grokPromptBudgetResult struct {
	EstimatedBefore int
	EstimatedAfter  int
	RemovedTurns    int
	RemovedItems    int
	RemovedToolSets int
}

type GrokPromptBudgetError struct {
	Estimated int
	Budget    int
	Reason    string
}

func (e *GrokPromptBudgetError) Error() string {
	reason := strings.TrimSpace(e.Reason)
	if reason == "" {
		reason = "the latest turn alone exceeds the safe prompt budget"
	}
	return fmt.Sprintf(
		"Grok context is too large (%d estimated input tokens; safe limit %d): %s. Start a new session or compact the conversation",
		e.Estimated,
		e.Budget,
		reason,
	)
}

func applyGrokResponsesPromptBudget(body []byte) ([]byte, grokPromptBudgetResult, error) {
	return applyGrokResponsesPromptBudgetWithLimit(body, grokResponsesPromptBudget)
}

func applyGrokResponsesPromptBudgetWithLimit(body []byte, budget int) ([]byte, grokPromptBudgetResult, error) {
	result := grokPromptBudgetResult{}
	if budget <= 0 {
		return body, result, fmt.Errorf("invalid Grok prompt budget: %d", budget)
	}

	estimated, err := estimateGrokResponsesPromptTokens(body)
	if err != nil {
		return body, result, fmt.Errorf("estimate Grok prompt tokens: %w", err)
	}
	result.EstimatedBefore = estimated
	result.EstimatedAfter = estimated
	if estimated <= budget {
		return body, result, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var requestBody map[string]any
	if err := decoder.Decode(&requestBody); err != nil {
		return body, result, fmt.Errorf("decode Grok prompt for context reduction: %w", err)
	}
	items, ok := requestBody["input"].([]any)
	if !ok || len(items) == 0 {
		return body, result, &GrokPromptBudgetError{
			Estimated: estimated,
			Budget:    budget,
			Reason:    "the request has no safely removable complete turns",
		}
	}

	for result.EstimatedAfter > budget {
		turns := grokPromptTurns(items)
		removed := 0
		if len(turns) > 1 && grokPromptTurnHasSelfContainedToolPairs(items[turns[0].start:turns[0].end]) {
			oldest := turns[0]
			removed = oldest.end - oldest.start
			items = append(items[:oldest.start], items[oldest.end:]...)
			result.RemovedTurns++
		} else if removable := grokOldestRemovableToolSet(items); len(removable) > 0 {
			filtered := make([]any, 0, len(items)-len(removable))
			for i, item := range items {
				if _, drop := removable[i]; drop {
					removed++
					continue
				}
				filtered = append(filtered, item)
			}
			items = filtered
			result.RemovedToolSets++
		} else {
			return body, result, &GrokPromptBudgetError{
				Estimated: result.EstimatedAfter,
				Budget:    budget,
				Reason:    "the latest turn and fixed instructions cannot be reduced because that would split a tool call from its result",
			}
		}

		requestBody["input"] = items
		result.RemovedItems += removed

		rebuilt, marshalErr := marshalOpenAIUpstreamJSON(requestBody)
		if marshalErr != nil {
			return body, result, fmt.Errorf("encode reduced Grok prompt: %w", marshalErr)
		}
		estimated, err = estimateGrokResponsesPromptTokens(rebuilt)
		if err != nil {
			return body, result, fmt.Errorf("estimate reduced Grok prompt tokens: %w", err)
		}
		result.EstimatedAfter = estimated
		body = rebuilt
	}

	return body, result, nil
}

// grokOldestRemovableToolSet returns one complete, non-latest tool-call set.
// Claude Code can run many tool cycles under a single user message, so turn
// trimming alone cannot reduce those sessions. Calls and outputs are removed
// together while newer context is always retained.
func grokOldestRemovableToolSet(items []any) map[int]struct{} {
	callIndexes := make(map[string][]int)
	outputIndexes := make(map[string][]int)
	collectingCalls := false
	finishedCalls := false
	for i, item := range items {
		itemMap, _ := item.(map[string]any)
		typ, _ := itemMap["type"].(string)
		callID, _ := itemMap["call_id"].(string)
		callID = strings.TrimSpace(callID)
		switch strings.ToLower(strings.TrimSpace(typ)) {
		case "function_call", "custom_tool_call":
			if finishedCalls {
				continue
			}
			if callID != "" {
				collectingCalls = true
				callIndexes[callID] = append(callIndexes[callID], i)
			}
		case "function_call_output", "custom_tool_call_output":
			if !collectingCalls || callID == "" {
				continue
			}
			finishedCalls = true
			if _, ok := callIndexes[callID]; ok {
				outputIndexes[callID] = append(outputIndexes[callID], i)
			}
		default:
			if collectingCalls && !finishedCalls {
				return nil
			}
		}
	}
	if len(callIndexes) == 0 {
		return nil
	}
	removable := make(map[int]struct{})
	lastRemoved := -1
	for id, calls := range callIndexes {
		outputs := outputIndexes[id]
		if len(calls) != len(outputs) {
			return nil
		}
		for _, index := range calls {
			removable[index] = struct{}{}
			if index > lastRemoved {
				lastRemoved = index
			}
		}
		for _, index := range outputs {
			removable[index] = struct{}{}
			if index > lastRemoved {
				lastRemoved = index
			}
		}
	}
	for i := lastRemoved + 1; i < len(items); i++ {
		itemMap, _ := items[i].(map[string]any)
		typ, _ := itemMap["type"].(string)
		switch strings.ToLower(strings.TrimSpace(typ)) {
		case "function_call_output", "custom_tool_call_output":
			continue
		default:
			return removable
		}
	}
	return nil
}

func estimateGrokResponsesPromptTokens(body []byte) (int, error) {
	var req openAIInputTokensCountRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return 0, err
	}
	if strings.TrimSpace(req.Model) == "" {
		return 0, fmt.Errorf("model is required")
	}
	return estimateOpenAIInputTokens(req)
}

type grokPromptTurn struct {
	start int
	end   int
}

func grokPromptTurns(items []any) []grokPromptTurn {
	starts := make([]int, 0)
	for i, item := range items {
		if grokPromptItemRole(item) == "user" {
			starts = append(starts, i)
		}
	}
	turns := make([]grokPromptTurn, 0, len(starts))
	for i, start := range starts {
		end := len(items)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		turns = append(turns, grokPromptTurn{start: start, end: end})
	}
	return turns
}

func grokPromptItemRole(item any) string {
	itemMap, _ := item.(map[string]any)
	role, _ := itemMap["role"].(string)
	return strings.ToLower(strings.TrimSpace(role))
}

func grokPromptTurnHasSelfContainedToolPairs(items []any) bool {
	calls := make(map[string]int)
	outputs := make(map[string]int)
	for _, item := range items {
		itemMap, _ := item.(map[string]any)
		typ, _ := itemMap["type"].(string)
		callID, _ := itemMap["call_id"].(string)
		callID = strings.TrimSpace(callID)
		if callID == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(typ)) {
		case "function_call", "custom_tool_call":
			calls[callID]++
		case "function_call_output", "custom_tool_call_output":
			outputs[callID]++
		}
	}
	for id, count := range calls {
		if outputs[id] != count {
			return false
		}
	}
	for id, count := range outputs {
		if calls[id] != count {
			return false
		}
	}
	return true
}
