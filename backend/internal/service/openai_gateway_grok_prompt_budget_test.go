package service

import (
	"errors"
	"testing"

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
