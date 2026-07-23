package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGrokConcurrencyMigrationContract(t *testing.T) {
	content, err := FS.ReadFile("187_enforce_grok_concurrency.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "WHERE platform = 'grok' AND deleted_at IS NULL")
	require.Contains(t, sql, "concurrency <= 0 OR concurrency > 2")
	require.Contains(t, sql, "CHECK (deleted_at IS NOT NULL OR platform <> 'grok' OR concurrency BETWEEN 1 AND 2)")
	require.Contains(t, sql, "VALIDATE CONSTRAINT accounts_grok_concurrency_safe")
}
