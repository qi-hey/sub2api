package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGPT6SolLunaOpenAIDefaultsMigrationContract(t *testing.T) {
	content, err := FS.ReadFile("241_add_gpt6_sol_luna_openai_defaults.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, `"gpt-6-sol": "gpt-6-sol"`)
	require.Contains(t, sql, `"gpt-6-luna": "gpt-6-luna"`)
	require.Contains(t, sql, "platform = 'openai'")
	require.Contains(t, sql, "type IN ('oauth', 'setup-token')")
	require.Contains(t, sql, "deleted_at IS NULL")
	require.Contains(t, sql, "credentials->'model_mapping' <> '{}'::jsonb")
	require.Contains(t, sql, "credentials->'model_mapping'->>'gpt-6-sol'")
	require.Contains(t, sql, "credentials->'model_mapping'->>'gpt-6-luna'")
	require.Contains(t, sql, "'::jsonb ||")
}
