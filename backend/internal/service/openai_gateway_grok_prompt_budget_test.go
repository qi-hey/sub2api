package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestApplyGrokResponsesPromptBudgetKeepsRequestUnderLimit(t *testing.T) {
	body := []byte(`{"model":"grok-4.5","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}]}`)

	updated, result, err := applyGrokResponsesPromptBudgetWithLimit(body, 100)
	require.NoError(t, err)
	require.Equal(t, body, updated)
	require.Zero(t, result.RemovedTurns)
}

func TestApplyGrokResponsesPromptBudgetKeepsStringInput(t *testing.T) {
	body := []byte(`{"model":"grok-4.6","input":"hello"}`)

	updated, result, err := applyGrokResponsesPromptBudgetWithLimit(body, 100)

	require.NoError(t, err)
	require.Equal(t, body, updated)
	require.Zero(t, result.RemovedTurns)
}

func TestApplyGrokResponsesPromptBudgetRemovesOldestCompleteToolTurn(t *testing.T) {
	body := []byte(`{
		"model":"grok-4.5",
		"input":[
			{"type":"message","role":"developer","content":[{"type":"input_text","text":"rules"}]},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"old question with removable details"}]},
			{"type":"function_call","call_id":"call_old","name":"read","arguments":"{}"},
			{"type":"function_call_output","call_id":"call_old","output":"old result"},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"latest question"}]}
		]
	}`)
	latestOnly := []byte(`{
		"model":"grok-4.5",
		"input":[
			{"type":"message","role":"developer","content":[{"type":"input_text","text":"rules"}]},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"latest question"}]}
		]
	}`)
	limit, err := estimateGrokResponsesPromptTokens(latestOnly)
	require.NoError(t, err)

	updated, result, err := applyGrokResponsesPromptBudgetWithLimit(body, limit)
	require.NoError(t, err)
	require.Equal(t, 1, result.RemovedTurns)
	require.Equal(t, 3, result.RemovedItems)
	require.LessOrEqual(t, result.EstimatedAfter, limit)
	require.False(t, gjson.GetBytes(updated, `input.#(call_id=="call_old")`).Exists())
	require.Contains(t, string(updated), "latest question")
	require.Contains(t, string(updated), "rules")
}

func TestApplyGrokResponsesPromptBudgetRemovesOldestToolSetWithinSingleUserTurn(t *testing.T) {
	body := []byte(`{
		"model":"grok-4.5",
		"input":[
			{"type":"message","role":"developer","content":[{"type":"input_text","text":"rules"}]},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"complete the task"}]},
			{"type":"function_call","call_id":"call_old","name":"read","arguments":"{}"},
			{"type":"function_call_output","call_id":"call_old","output":"old verbose result that should be removed"},
			{"type":"custom_tool_call","call_id":"call_new","name":"apply_patch","input":"patch"},
			{"type":"custom_tool_call_output","call_id":"call_new","output":"new result"},
			{"type":"message","role":"assistant","content":[{"type":"output_text","text":"latest state"}]}
		]
	}`)
	withoutOldToolSet := []byte(`{
		"model":"grok-4.5",
		"input":[
			{"type":"message","role":"developer","content":[{"type":"input_text","text":"rules"}]},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"complete the task"}]},
			{"type":"custom_tool_call","call_id":"call_new","name":"apply_patch","input":"patch"},
			{"type":"custom_tool_call_output","call_id":"call_new","output":"new result"},
			{"type":"message","role":"assistant","content":[{"type":"output_text","text":"latest state"}]}
		]
	}`)
	limit, err := estimateGrokResponsesPromptTokens(withoutOldToolSet)
	require.NoError(t, err)

	updated, result, err := applyGrokResponsesPromptBudgetWithLimit(body, limit)
	require.NoError(t, err)
	require.Zero(t, result.RemovedTurns)
	require.Equal(t, 1, result.RemovedToolSets)
	require.Equal(t, 2, result.RemovedItems)
	require.LessOrEqual(t, result.EstimatedAfter, limit)
	require.False(t, gjson.GetBytes(updated, `input.#(call_id=="call_old")`).Exists())
	require.True(t, gjson.GetBytes(updated, `input.#(call_id=="call_new")`).Exists())
	require.Contains(t, string(updated), "latest state")
}

func TestApplyGrokResponsesPromptBudgetRejectsSplitToolPair(t *testing.T) {
	body := []byte(`{
		"model":"grok-4.5",
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"old"}]},
			{"type":"function_call","call_id":"call_split","name":"read","arguments":"{}"},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"latest"}]},
			{"type":"function_call_output","call_id":"call_split","output":"result"}
		]
	}`)

	_, _, err := applyGrokResponsesPromptBudgetWithLimit(body, 1)
	var budgetErr *GrokPromptBudgetError
	require.ErrorAs(t, err, &budgetErr)
	require.Contains(t, budgetErr.Reason, "split a tool call")
}

func TestApplyGrokResponsesPromptBudgetRejectsOversizedLatestTurn(t *testing.T) {
	body := []byte(`{"model":"grok-4.5","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"only latest turn"}]}]}`)

	_, _, err := applyGrokResponsesPromptBudgetWithLimit(body, 1)
	var budgetErr *GrokPromptBudgetError
	require.True(t, errors.As(err, &budgetErr))
	require.Contains(t, err.Error(), "Start a new session or compact")
}

func TestEstimateGrokResponsesPromptTokenBreakdownMatchesFullEstimate(t *testing.T) {
	body := []byte(`{
		"model":"grok-4.6",
		"instructions":"follow the rules",
		"input":[
			{"type":"message","role":"developer","content":[{"type":"input_text","text":"rules"}]},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"question"}]},
			{"type":"function_call","call_id":"call_1","name":"read","arguments":"{\"path\":\"README.md\"}"},
			{"type":"function_call_output","call_id":"call_1","output":"result"}
		],
		"tools":[
			{"type":"function","name":"read","description":"read a file","parameters":{"type":"object","properties":{"path":{"type":"string"}}}}
		],
		"tool_choice":"auto"
	}`)

	want, err := estimateGrokResponsesPromptTokens(body)
	require.NoError(t, err)
	breakdown, err := estimateGrokResponsesPromptTokenBreakdown(body)
	require.NoError(t, err)

	require.Equal(t, want, breakdown.TotalTokens)
	require.Len(t, breakdown.ItemTokens, 4)
	require.Equal(t, breakdown.FixedTokens+sumGrokPromptItemTokens(breakdown.ItemTokens), breakdown.TotalTokens)
}

func TestApplyGrokResponsesPromptBudgetLargeToolSchemaCompletesQuickly(t *testing.T) {
	const (
		oldTurns       = 150
		turnsToRemove  = 140
		maxTrimRuntime = 5 * time.Second
	)

	input := make([]any, 0, 2+oldTurns*2)
	input = append(input, map[string]any{
		"type": "message",
		"role": "developer",
		"content": []any{
			map[string]any{"type": "input_text", "text": "keep the system rules"},
		},
	})
	for i := 0; i < oldTurns; i++ {
		input = append(input,
			map[string]any{
				"type": "message",
				"role": "user",
				"content": []any{
					map[string]any{"type": "input_text", "text": fmt.Sprintf("old question %03d with removable context", i)},
				},
			},
			map[string]any{
				"type": "message",
				"role": "assistant",
				"content": []any{
					map[string]any{"type": "output_text", "text": fmt.Sprintf("old answer %03d with removable context", i)},
				},
			},
		)
	}
	input = append(input, map[string]any{
		"type": "message",
		"role": "user",
		"content": []any{
			map[string]any{"type": "input_text", "text": "latest question must remain"},
		},
	})

	body, err := json.Marshal(map[string]any{
		"model": "grok-4.6",
		"input": input,
		"tools": []any{
			map[string]any{
				"type":        "function",
				"name":        "large_schema_tool",
				"description": strings.Repeat("abcdefghijklmnopqrstuvwxyz0123456789 ", 48_000),
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"payload": map[string]any{"type": "string"},
					},
				},
			},
		},
	})
	require.NoError(t, err)
	require.Greater(t, len(body), 1_500_000)

	breakdown, err := estimateGrokResponsesPromptTokenBreakdown(body)
	require.NoError(t, err)
	firstRemovedIndex := 1
	lastRemovedIndex := firstRemovedIndex + turnsToRemove*2
	budget := breakdown.TotalTokens - sumGrokPromptItemTokens(breakdown.ItemTokens[firstRemovedIndex:lastRemovedIndex])

	start := time.Now()
	updated, result, err := applyGrokResponsesPromptBudgetWithLimit(body, budget)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.Equal(t, turnsToRemove, result.RemovedTurns)
	require.LessOrEqual(t, result.EstimatedAfter, budget)
	require.Contains(t, string(updated), "latest question must remain")
	require.Less(t, elapsed, maxTrimRuntime, "large Grok prompts must not recount the fixed tool schema once per removed turn")
}
