package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestPatchGrokResponsesBodyRepairsAutomationUpdateMixedRootUnion(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"model":"grok-4.6",
		"input":"hello",
		"tools":[{
			"type":"namespace",
			"name":"mcp__codex_app",
			"tools":[{
				"type":"function",
				"name":"automation_update",
				"description":"Update an automation.",
				"parameters":{
					"anyOf":[
						{"properties":{"id":{"type":"string"},"mode":{"const":"view"}},"required":["id","mode"],"additionalProperties":false},
						{"type":"object","properties":{"kind":{"const":"heartbeat"},"mode":{"const":"create"}},"required":["kind","mode"]},
						{"type":"null"}
					]
				}
			}]
		}],
		"tool_choice":{"type":"function","namespace":"mcp__codex_app","name":"automation_update"}
	}`)

	patched, mapping, err := patchGrokResponsesBodyWithClientTools(body, "grok-4.6")

	require.NoError(t, err)
	require.True(t, json.Valid(patched))
	require.Equal(t, "automation_update", mapping.NamespaceTools["mcp__codex_app__automation_update"].Name)
	require.Equal(t, "mcp__codex_app", mapping.NamespaceTools["mcp__codex_app__automation_update"].Namespace)
	require.Equal(t, "mcp__codex_app__automation_update", gjson.GetBytes(patched, "tools.0.name").String())
	require.Equal(t, "object", gjson.GetBytes(patched, "tools.0.parameters.type").String())
	require.Len(t, gjson.GetBytes(patched, "tools.0.parameters.anyOf").Array(), 2)
	require.Equal(t, "object", gjson.GetBytes(patched, "tools.0.parameters.anyOf.0.type").String())
	require.Equal(t, "object", gjson.GetBytes(patched, "tools.0.parameters.anyOf.1.type").String())
	require.Equal(t, "mcp__codex_app__automation_update", gjson.GetBytes(patched, "tool_choice.name").String())
	require.False(t, gjson.GetBytes(patched, "tool_choice.namespace").Exists())
}

func TestPatchGrokResponsesBodyRepairsAutomationUpdateReferencedRootUnion(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"model":"grok-4.6",
		"input":"hello",
		"tools":[{
			"type":"namespace",
			"name":"mcp__codex_app",
			"tools":[{
				"type":"function",
				"name":"automation_update",
				"description":"Update an automation.",
				"parameters":{
					"type":"object",
					"oneOf":[
						{"$ref":"#/$defs/view"},
						{"$ref":"#/$defs/create"},
						{"$ref":"#/$defs/update"},
						{"$ref":"#/$defs/delete"}
					],
					"$defs":{
						"view":{
							"type":"object",
							"properties":{"mode":{"const":"view"},"id":{"type":"string"}},
							"required":["mode","id"],
							"additionalProperties":false
						},
						"create":{
							"oneOf":[
								{"$ref":"#/$defs/cronCreate"},
								{"$ref":"#/$defs/heartbeatCreate"}
							]
						},
						"cronCreate":{
							"type":"object",
							"properties":{
								"mode":{"const":"create"},
								"kind":{"const":"cron"},
								"notificationPolicy":{"$ref":"#/$defs/nullableNotification"}
							},
							"required":["mode","kind"],
							"additionalProperties":false
						},
						"heartbeatCreate":{
							"type":"object",
							"properties":{"mode":{"const":"create"},"kind":{"const":"heartbeat"}},
							"required":["mode","kind"],
							"additionalProperties":false
						},
						"update":{
							"oneOf":[
								{"$ref":"#/$defs/cronUpdate"},
								{"$ref":"#/$defs/heartbeatUpdate"}
							]
						},
						"cronUpdate":{
							"type":"object",
							"properties":{"mode":{"const":"update"},"kind":{"const":"cron"},"id":{"type":"string"}},
							"required":["mode","kind","id"],
							"additionalProperties":false
						},
						"heartbeatUpdate":{
							"type":"object",
							"properties":{"mode":{"const":"update"},"kind":{"const":"heartbeat"},"id":{"type":"string"}},
							"required":["mode","kind","id"],
							"additionalProperties":false
						},
						"delete":{
							"type":"object",
							"properties":{"mode":{"const":"delete"},"id":{"type":"string"}},
							"required":["mode","id"],
							"additionalProperties":false
						},
						"nullableNotification":{
							"anyOf":[
								{"type":"string","enum":["failed_runs_only"]},
								{"type":"null"}
							]
						}
					}
				}
			}]
		}]
	}`)

	patched, mapping, err := patchGrokResponsesBodyWithClientTools(body, "grok-4.6")

	require.NoError(t, err)
	require.True(t, json.Valid(patched))
	require.Equal(t, "automation_update", mapping.NamespaceTools["mcp__codex_app__automation_update"].Name)
	require.Equal(t, "mcp__codex_app", mapping.NamespaceTools["mcp__codex_app__automation_update"].Namespace)
	require.Equal(t, "object", gjson.GetBytes(patched, "tools.0.parameters.type").String())
	require.Len(t, gjson.GetBytes(patched, "tools.0.parameters.oneOf").Array(), 4)
	for index := range 4 {
		path := "tools.0.parameters.oneOf." + string(rune('0'+index))
		require.Equal(t, "object", gjson.GetBytes(patched, path+".type").String())
		require.False(t, gjson.GetBytes(patched, path+".$ref").Exists())
	}
	require.Len(t, gjson.GetBytes(patched, "tools.0.parameters.oneOf.1.oneOf").Array(), 2)
	require.Equal(t, "object", gjson.GetBytes(patched, "tools.0.parameters.oneOf.1.oneOf.0.type").String())
	require.Equal(t, "object", gjson.GetBytes(patched, "tools.0.parameters.oneOf.1.oneOf.1.type").String())
	require.Equal(t, "null", gjson.GetBytes(patched, "tools.0.parameters.$defs.nullableNotification.anyOf.1.type").String())
}

func TestSanitizeGrokResponsesToolsDropsOnlyPrimitiveParameterTool(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"tools":[
			{"type":"function","name":"bad","parameters":{"oneOf":[{"type":"string"},{"type":"null"}]}},
			{"type":"function","name":"good","parameters":{"type":"object","properties":{"q":{"type":"string"}}}}
		],
		"tool_choice":{"type":"function","name":"bad"},
		"parallel_tool_calls":true
	}`)

	patched, err := sanitizeGrokResponsesTools(body)

	require.NoError(t, err)
	require.Len(t, gjson.GetBytes(patched, "tools").Array(), 1)
	require.Equal(t, "good", gjson.GetBytes(patched, "tools.0.name").String())
	require.False(t, gjson.GetBytes(patched, "tool_choice").Exists())
	require.True(t, gjson.GetBytes(patched, "parallel_tool_calls").Bool())
}

func TestSanitizeGrokResponsesToolsNormalizesObjectNullableTypeArray(t *testing.T) {
	t.Parallel()

	body := []byte(`{"tools":[{"type":"function","name":"lookup","parameters":{"type":["object","null"],"properties":{"q":{"type":"string"}}}}]}`)

	patched, err := sanitizeGrokResponsesTools(body)

	require.NoError(t, err)
	require.Equal(t, "object", gjson.GetBytes(patched, "tools.0.parameters.type").String())
	require.Equal(t, "string", gjson.GetBytes(patched, "tools.0.parameters.properties.q.type").String())
}

func TestSanitizeGrokResponsesToolsLeavesValidFunctionSchemaUnchanged(t *testing.T) {
	t.Parallel()

	body := []byte(`{"tools":[{"type":"function","name":"lookup","parameters":{"type":"object","properties":{"q":{"type":"string"}}}}]}`)

	patched, err := sanitizeGrokResponsesTools(body)

	require.NoError(t, err)
	require.Equal(t, string(body), string(patched))
}
