package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration231DefaultsExistingOpenAIOAuthLikeAccountsToBalanced(t *testing.T) {
	content, err := FS.ReadFile("231_default_existing_openai_codex_fingerprint_balanced.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "platform = 'openai'")
	require.Contains(t, sql, "type IN ('oauth', 'setup-token')")
	require.Contains(t, sql, "'{codex_fingerprint_mode}'")
	require.Contains(t, sql, "'\"session\"'::jsonb")
	require.Contains(t, sql, "'{codex_fingerprint_seed}'")
	require.Contains(t, sql, "gen_random_uuid()::text")
	require.Contains(t, sql, "deleted_at IS NULL")
	require.NotContains(t, sql, "type = 'apikey'")
}
