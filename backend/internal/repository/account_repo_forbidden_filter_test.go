package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func TestAccountListForbiddenFilterUsesGrokUsageSnapshot(t *testing.T) {
	var capturedSQL string
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(captureEntQueryMatcher{actual: &capturedSQL}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	driver := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	repo := newAccountRepositoryWithSQL(client, db, nil)

	mock.ExpectQuery("forbidden account count").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	total, err := repo.accountListFilteredQuery("", "", "forbidden", "", 0, "").Count(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.NoError(t, mock.ExpectationsWereMet())

	normalized := normalizeSQLWhitespace(capturedSQL)
	for _, fragment := range []string{"platform", "grok_usage_snapshot", "status_code", "deleted_at"} {
		require.Contains(t, strings.ToLower(normalized), fragment, "missing Forbidden filter fragment %q in SQL: %s", fragment, normalized)
	}
}
