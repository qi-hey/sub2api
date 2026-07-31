package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGameWalletExchangeReplaysBeforeCheckingCurrentConfig(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now().UTC()
	expectGameWalletUserLock(mock, 7, "10.00000000", true)
	mock.ExpectQuery(regexp.QuoteMeta(gameWalletIdempotencyQuery)).
		WithArgs(int64(7), "request-1").
		WillReturnRows(sqlmock.NewRows(gameWalletIdempotencyColumns()).AddRow(
			91, 7, service.GameWalletDirectionBalanceToCredits, "1.00000000", 100,
			"100", "10.00000000", 300, "9.00000000", 400,
			"request-1", "same-hash", now,
		))
	mock.ExpectCommit()

	repo := NewGameWalletRepository(db)
	result, err := repo.Exchange(context.Background(), 7, service.GameWalletExchangeInput{
		Direction:      service.GameWalletDirectionBalanceToCredits,
		BalanceAmount:  "1",
		IdempotencyKey: "request-1",
		RequestHash:    "same-hash",
	}, service.GameWalletExchangeConfig{Enabled: false, Rate: "0", DailyLimit: "0", DailyLimitValid: true})

	require.NoError(t, err)
	require.True(t, result.IdempotentReplay)
	require.Equal(t, "10.00000000", result.BalanceBefore)
	require.Equal(t, int64(300), result.CreditsBefore)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGameWalletExchangeConflictingPayloadRollsBack(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	expectGameWalletUserLock(mock, 7, "10.00000000", true)
	mock.ExpectQuery(regexp.QuoteMeta(gameWalletIdempotencyQuery)).
		WithArgs(int64(7), "request-1").
		WillReturnRows(sqlmock.NewRows(gameWalletIdempotencyColumns()).AddRow(
			91, 7, service.GameWalletDirectionBalanceToCredits, "1.00000000", 100,
			"100", "10.00000000", 300, "9.00000000", 400,
			"request-1", "original-hash", time.Now().UTC(),
		))
	mock.ExpectRollback()

	repo := NewGameWalletRepository(db)
	_, err = repo.Exchange(context.Background(), 7, service.GameWalletExchangeInput{
		Direction:      service.GameWalletDirectionBalanceToCredits,
		BalanceAmount:  "2",
		IdempotencyKey: "request-1",
		RequestHash:    "different-hash",
	}, service.GameWalletExchangeConfig{})

	require.ErrorIs(t, err, service.ErrGameWalletIdempotencyConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGameWalletExchangeBalanceToCreditsIsAtomicAndAudited(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now().UTC()
	expectGameWalletUserLock(mock, 7, "10.00000000", true)
	expectNoGameWalletIdempotencyRecord(mock, 7, "request-2")
	expectGameWalletWalletLock(mock, 7, 300)
	mock.ExpectQuery(regexp.QuoteMeta(gameWalletBalanceToCreditsCalculationQuery)).
		WithArgs("1.25", "100").
		WillReturnRows(sqlmock.NewRows([]string{"credits"}).AddRow(125))
	mock.ExpectQuery(regexp.QuoteMeta(gameWalletDailyLimitQuery)).
		WithArgs(int64(7), "1.25", "5", gameWalletMaxDailyTransactions).
		WillReturnRows(sqlmock.NewRows([]string{"used", "amount_allowed", "count_allowed"}).AddRow("2.00000000", true, true))
	mock.ExpectQuery(regexp.QuoteMeta(gameWalletDeductBalanceQuery)).
		WithArgs(int64(7), "1.25").
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow("8.75000000"))
	mock.ExpectQuery(regexp.QuoteMeta(gameWalletAddCreditsQuery)).
		WithArgs(int64(7), int64(125)).
		WillReturnRows(sqlmock.NewRows([]string{"credits"}).AddRow(425))
	mock.ExpectQuery(regexp.QuoteMeta(gameWalletInsertTransactionQuery)).
		WithArgs(
			int64(7), service.GameWalletDirectionBalanceToCredits, "1.25", int64(125), "100",
			"10.00000000", int64(300), "8.75000000", int64(425), "request-2", "request-hash",
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(92, now))
	mock.ExpectCommit()

	repo := NewGameWalletRepository(db)
	result, err := repo.Exchange(context.Background(), 7, service.GameWalletExchangeInput{
		Direction:      service.GameWalletDirectionBalanceToCredits,
		BalanceAmount:  "1.25",
		IdempotencyKey: "request-2",
		RequestHash:    "request-hash",
	}, service.GameWalletExchangeConfig{
		Enabled: true, Rate: "100", RateConfigured: true, DailyLimit: "5", DailyLimitValid: true,
	})

	require.NoError(t, err)
	require.Equal(t, "10.00000000", result.BalanceBefore)
	require.Equal(t, int64(300), result.CreditsBefore)
	require.Equal(t, "8.75000000", result.BalanceAfter)
	require.Equal(t, int64(425), result.CreditsAfter)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGameWalletExchangeCreditsToBalanceDoesNotRechargeOrRebate(t *testing.T) {
	require.NotContains(t, gameWalletAddBalanceQuery, "total_recharged")

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now().UTC()
	expectGameWalletUserLock(mock, 7, "8.75000000", true)
	expectNoGameWalletIdempotencyRecord(mock, 7, "request-3")
	expectGameWalletWalletLock(mock, 7, 425)
	mock.ExpectQuery(regexp.QuoteMeta(gameWalletCreditsToBalanceCalculationQuery)).
		WithArgs(int64(125), "100").
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow("1.25000000"))
	mock.ExpectQuery(regexp.QuoteMeta(gameWalletDailyLimitQuery)).
		WithArgs(int64(7), "1.25000000", "0", gameWalletMaxDailyTransactions).
		WillReturnRows(sqlmock.NewRows([]string{"used", "amount_allowed", "count_allowed"}).AddRow("3.25000000", true, true))
	mock.ExpectQuery(regexp.QuoteMeta(gameWalletDeductCreditsQuery)).
		WithArgs(int64(7), int64(125)).
		WillReturnRows(sqlmock.NewRows([]string{"credits"}).AddRow(300))
	mock.ExpectQuery(regexp.QuoteMeta(gameWalletAddBalanceQuery)).
		WithArgs(int64(7), "1.25000000").
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow("10.00000000"))
	mock.ExpectQuery(regexp.QuoteMeta(gameWalletInsertTransactionQuery)).
		WithArgs(
			int64(7), service.GameWalletDirectionCreditsToBalance, "1.25000000", int64(125), "100",
			"8.75000000", int64(425), "10.00000000", int64(300), "request-3", "request-hash-3",
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(93, now))
	mock.ExpectCommit()

	repo := NewGameWalletRepository(db)
	result, err := repo.Exchange(context.Background(), 7, service.GameWalletExchangeInput{
		Direction:      service.GameWalletDirectionCreditsToBalance,
		CreditsAmount:  125,
		IdempotencyKey: "request-3",
		RequestHash:    "request-hash-3",
	}, service.GameWalletExchangeConfig{
		Enabled: true, Rate: "100", RateConfigured: true, DailyLimit: "0", DailyLimitValid: true,
	})

	require.NoError(t, err)
	require.Equal(t, "8.75000000", result.BalanceBefore)
	require.Equal(t, int64(425), result.CreditsBefore)
	require.Equal(t, "10.00000000", result.BalanceAfter)
	require.Equal(t, int64(300), result.CreditsAfter)
	require.NoError(t, mock.ExpectationsWereMet())
}

func expectGameWalletUserLock(mock sqlmock.Sqlmock, userID int64, balance string, nonNegative bool) {
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(gameWalletLockUserQuery)).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "non_negative"}).AddRow(balance, nonNegative))
}

func expectGameWalletWalletLock(mock sqlmock.Sqlmock, userID int64, credits int64) {
	mock.ExpectExec(regexp.QuoteMeta(gameWalletEnsureQuery)).
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(gameWalletLockWalletQuery)).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "credits"}).AddRow(12, credits))
}

func TestGameWalletExchangeReplaysWhenCurrentBalanceIsNegative(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now().UTC()
	expectGameWalletUserLock(mock, 7, "-0.50000000", false)
	mock.ExpectQuery(regexp.QuoteMeta(gameWalletIdempotencyQuery)).
		WithArgs(int64(7), "request-negative-replay").
		WillReturnRows(sqlmock.NewRows(gameWalletIdempotencyColumns()).AddRow(
			94, 7, service.GameWalletDirectionBalanceToCredits, "1.00000000", 100,
			"100", "10.00000000", 300, "9.00000000", 400,
			"request-negative-replay", "same-hash", now,
		))
	mock.ExpectCommit()

	repo := NewGameWalletRepository(db)
	result, err := repo.Exchange(context.Background(), 7, service.GameWalletExchangeInput{
		Direction:      service.GameWalletDirectionBalanceToCredits,
		BalanceAmount:  "1",
		IdempotencyKey: "request-negative-replay",
		RequestHash:    "same-hash",
	}, service.GameWalletExchangeConfig{})

	require.NoError(t, err)
	require.True(t, result.IdempotentReplay)
	require.NoError(t, mock.ExpectationsWereMet())
}

func expectNoGameWalletIdempotencyRecord(mock sqlmock.Sqlmock, userID int64, key string) {
	mock.ExpectQuery(regexp.QuoteMeta(gameWalletIdempotencyQuery)).
		WithArgs(userID, key).
		WillReturnRows(sqlmock.NewRows(gameWalletIdempotencyColumns()))
}

func gameWalletIdempotencyColumns() []string {
	return []string{
		"id", "user_id", "direction", "balance_amount", "credits_amount",
		"exchange_rate", "balance_before", "credits_before", "balance_after", "credits_after",
		"idempotency_key", "request_hash", "created_at",
	}
}
