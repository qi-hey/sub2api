package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGrokOpenAIFallbackMappingMigrationContract(t *testing.T) {
	content, err := FS.ReadFile("186_backfill_grok_openai_fallback_mappings.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	for _, model := range []string{
		"gpt-5.2", "gpt-5.4", "gpt-5.4-mini", "gpt-5.5",
		"gpt-5.6-luna", "gpt-5.6-sol", "gpt-5.6-terra",
	} {
		require.Contains(t, sql, `"`+model+`": "grok-4.5"`)
	}
	require.Contains(t, sql, "'::jsonb || COALESCE(credentials->'model_mapping', '{}'::jsonb)")
	require.Contains(t, sql, "WHERE platform = 'grok' AND deleted_at IS NULL")
}
