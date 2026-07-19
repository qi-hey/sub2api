package handler

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyGroupIDsJSONDistinguishesOmittedAndExplicitEmpty(t *testing.T) {
	var createOmitted CreateAPIKeyRequest
	require.NoError(t, json.Unmarshal([]byte(`{"name":"legacy"}`), &createOmitted))
	require.Nil(t, createOmitted.GroupIDs)

	var createEmpty CreateAPIKeyRequest
	require.NoError(t, json.Unmarshal([]byte(`{"name":"invalid","group_ids":[]}`), &createEmpty))
	require.NotNil(t, createEmpty.GroupIDs)
	require.Empty(t, createEmpty.GroupIDs)

	var omitted UpdateAPIKeyRequest
	require.NoError(t, json.Unmarshal([]byte(`{"name":"unchanged bindings"}`), &omitted))
	require.Nil(t, omitted.GroupIDs)

	var empty UpdateAPIKeyRequest
	require.NoError(t, json.Unmarshal([]byte(`{"group_ids":[]}`), &empty))
	require.NotNil(t, empty.GroupIDs)
	require.Empty(t, empty.GroupIDs)
}
