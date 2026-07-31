package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	gameWalletEnsureQuery = `
INSERT INTO game_wallets (user_id)
SELECT id FROM users WHERE id = $1 AND deleted_at IS NULL
ON CONFLICT (user_id) DO NOTHING`

	gameWalletSnapshotQuery = `
SELECT u.id,
       u.balance::text,
       w.credits,
       COALESCE(d.daily_exchanged, 0)::text,
       w.created_at,
       w.updated_at
FROM users u
JOIN game_wallets w ON w.user_id = u.id
LEFT JOIN LATERAL (
    SELECT SUM(t.balance_amount) AS daily_exchanged
    FROM game_wallet_transactions t
    WHERE t.user_id = u.id
      AND t.created_at >= date_trunc('day', CURRENT_TIMESTAMP)
      AND t.created_at < date_trunc('day', CURRENT_TIMESTAMP) + INTERVAL '1 day'
) d ON TRUE
WHERE u.id = $1 AND u.deleted_at IS NULL`

	gameWalletLockUserQuery = `
SELECT balance::text, balance >= 0
FROM users
WHERE id = $1 AND deleted_at IS NULL
FOR UPDATE`

	gameWalletLockWalletQuery = `
SELECT id, credits
FROM game_wallets
WHERE user_id = $1
FOR UPDATE`

	gameWalletIdempotencyQuery = `
SELECT id, user_id, direction, balance_amount::text, credits_amount,
       exchange_rate::text, balance_before::text, credits_before,
       balance_after::text, credits_after,
       idempotency_key, request_hash, created_at
FROM game_wallet_transactions
WHERE user_id = $1 AND idempotency_key = $2`

	gameWalletDailyLimitQuery = `
SELECT COALESCE(SUM(balance_amount), 0)::text,
	   ($3::numeric = 0 OR COALESCE(SUM(balance_amount), 0) + $2::numeric <= $3::numeric),
	   COUNT(*) < $4::bigint
FROM game_wallet_transactions
WHERE user_id = $1
  AND created_at >= date_trunc('day', CURRENT_TIMESTAMP)
  AND created_at < date_trunc('day', CURRENT_TIMESTAMP) + INTERVAL '1 day'`

	gameWalletBalanceToCreditsCalculationQuery = `
SELECT CASE
         WHEN converted = trunc(converted)
          AND converted BETWEEN 1 AND 9223372036854775807
         THEN converted::bigint
       END
FROM (SELECT $1::numeric * $2::bigint AS converted) calculation`

	gameWalletCreditsToBalanceCalculationQuery = `
SELECT CASE
         WHEN converted = trunc(converted, 8)
          AND converted > 0
          AND converted <= 999999999999.99999999
         THEN converted::numeric(20, 8)::text
       END
FROM (SELECT $1::numeric / $2::bigint AS converted) calculation`

	gameWalletDeductBalanceQuery = `
UPDATE users
SET balance = balance - $2::numeric, updated_at = NOW()
WHERE id = $1 AND balance >= $2::numeric
RETURNING balance::text`

	gameWalletAddBalanceQuery = `
UPDATE users
SET balance = balance + $2::numeric, updated_at = NOW()
WHERE id = $1 AND balance <= 999999999999.99999999 - $2::numeric
RETURNING balance::text`

	gameWalletAddCreditsQuery = `
UPDATE game_wallets
SET credits = credits + $2::bigint, updated_at = NOW()
WHERE user_id = $1 AND credits <= 9223372036854775807::bigint - $2::bigint
RETURNING credits`

	gameWalletDeductCreditsQuery = `
UPDATE game_wallets
SET credits = credits - $2::bigint, updated_at = NOW()
WHERE user_id = $1 AND credits >= $2::bigint
RETURNING credits`

	gameWalletInsertTransactionQuery = `
INSERT INTO game_wallet_transactions (
    user_id, direction, balance_amount, credits_amount, exchange_rate,
    balance_before, credits_before, balance_after, credits_after,
    idempotency_key, request_hash
) VALUES ($1, $2, $3::numeric, $4, $5::bigint, $6::numeric, $7, $8::numeric, $9, $10, $11)
RETURNING id, created_at`

	gameWalletListTransactionsQuery = `
SELECT id, user_id, direction, balance_amount::text, credits_amount,
       exchange_rate::text, balance_before::text, credits_before,
       balance_after::text, credits_after,
       idempotency_key, created_at
FROM game_wallet_transactions
WHERE user_id = $1 AND ($3::bigint = 0 OR id < $3::bigint)
ORDER BY id DESC
LIMIT $2`
)

type gameWalletRepository struct {
	db *sql.DB
}

const gameWalletMaxDailyTransactions = 1000

var _ service.GameWalletRepository = (*gameWalletRepository)(nil)

func NewGameWalletRepository(db *sql.DB) service.GameWalletRepository {
	return &gameWalletRepository{db: db}
}

func (r *gameWalletRepository) GetSnapshot(ctx context.Context, userID int64) (*service.GameWalletSnapshot, error) {
	if _, err := r.db.ExecContext(ctx, gameWalletEnsureQuery, userID); err != nil {
		return nil, fmt.Errorf("ensure game wallet: %w", err)
	}
	snapshot := &service.GameWalletSnapshot{}
	err := r.db.QueryRowContext(ctx, gameWalletSnapshotQuery, userID).Scan(
		&snapshot.UserID,
		&snapshot.AccountBalance,
		&snapshot.Credits,
		&snapshot.DailyExchanged,
		&snapshot.WalletCreatedAt,
		&snapshot.WalletUpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrGameWalletUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get game wallet snapshot: %w", err)
	}
	return snapshot, nil
}

func (r *gameWalletRepository) Exchange(ctx context.Context, userID int64, input service.GameWalletExchangeInput, cfg service.GameWalletExchangeConfig) (*service.GameWalletTransaction, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin game wallet exchange: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var lockedBalance string
	var balanceIsNonNegative bool
	if err := tx.QueryRowContext(ctx, gameWalletLockUserQuery, userID).Scan(&lockedBalance, &balanceIsNonNegative); errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrGameWalletUserNotFound
	} else if err != nil {
		return nil, fmt.Errorf("lock game wallet user: %w", err)
	}
	existing, requestHash, err := findGameWalletIdempotencyRecord(ctx, tx, userID, input.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if requestHash != input.RequestHash {
			return nil, service.ErrGameWalletIdempotencyConflict
		}
		existing.IdempotentReplay = true
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit game wallet replay: %w", err)
		}
		return existing, nil
	}
	if !balanceIsNonNegative {
		return nil, service.ErrGameWalletAccountOverdrawn
	}
	if _, err := tx.ExecContext(ctx, gameWalletEnsureQuery, userID); err != nil {
		return nil, fmt.Errorf("ensure locked game wallet: %w", err)
	}
	var walletID, lockedCredits int64
	if err := tx.QueryRowContext(ctx, gameWalletLockWalletQuery, userID).Scan(&walletID, &lockedCredits); err != nil {
		return nil, fmt.Errorf("lock game wallet: %w", err)
	}

	if !cfg.Enabled {
		return nil, service.ErrGameWalletDisabled
	}
	if !cfg.RateConfigured || cfg.Rate == "0" {
		return nil, service.ErrGameWalletRateNotConfigured
	}
	if !cfg.DailyLimitValid {
		return nil, service.ErrGameWalletConfigInvalid
	}

	result := &service.GameWalletTransaction{
		UserID:         userID,
		Direction:      input.Direction,
		ExchangeRate:   cfg.Rate,
		BalanceBefore:  lockedBalance,
		CreditsBefore:  lockedCredits,
		IdempotencyKey: input.IdempotencyKey,
	}

	switch input.Direction {
	case service.GameWalletDirectionBalanceToCredits:
		result.BalanceAmount = input.BalanceAmount
		var credits sql.NullInt64
		if err := tx.QueryRowContext(ctx, gameWalletBalanceToCreditsCalculationQuery, input.BalanceAmount, cfg.Rate).Scan(&credits); err != nil {
			return nil, fmt.Errorf("calculate game wallet credits: %w", err)
		}
		if !credits.Valid {
			return nil, service.ErrGameWalletAmountNotConvertible
		}
		result.CreditsAmount = credits.Int64
		if err := enforceGameWalletDailyLimit(ctx, tx, userID, result.BalanceAmount, cfg.DailyLimit); err != nil {
			return nil, err
		}
		if err := tx.QueryRowContext(ctx, gameWalletDeductBalanceQuery, userID, result.BalanceAmount).Scan(&result.BalanceAfter); errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrGameWalletInsufficientBalance
		} else if err != nil {
			return nil, fmt.Errorf("deduct account balance for game wallet: %w", err)
		}
		if err := tx.QueryRowContext(ctx, gameWalletAddCreditsQuery, userID, result.CreditsAmount).Scan(&result.CreditsAfter); errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrGameWalletCreditsLimitExceeded
		} else if err != nil {
			return nil, fmt.Errorf("add game wallet credits: %w", err)
		}

	case service.GameWalletDirectionCreditsToBalance:
		result.CreditsAmount = input.CreditsAmount
		var balanceAmount sql.NullString
		if err := tx.QueryRowContext(ctx, gameWalletCreditsToBalanceCalculationQuery, input.CreditsAmount, cfg.Rate).Scan(&balanceAmount); err != nil {
			return nil, fmt.Errorf("calculate game wallet balance: %w", err)
		}
		if !balanceAmount.Valid {
			return nil, service.ErrGameWalletAmountNotConvertible
		}
		result.BalanceAmount = balanceAmount.String
		if err := enforceGameWalletDailyLimit(ctx, tx, userID, result.BalanceAmount, cfg.DailyLimit); err != nil {
			return nil, err
		}
		if err := tx.QueryRowContext(ctx, gameWalletDeductCreditsQuery, userID, result.CreditsAmount).Scan(&result.CreditsAfter); errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrGameWalletInsufficientCredits
		} else if err != nil {
			return nil, fmt.Errorf("deduct game wallet credits: %w", err)
		}
		// This deliberately updates only users.balance. It must not touch
		// users.total_recharged or call any recharge/affiliate path.
		if err := tx.QueryRowContext(ctx, gameWalletAddBalanceQuery, userID, result.BalanceAmount).Scan(&result.BalanceAfter); errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrGameWalletBalanceLimitExceeded
		} else if err != nil {
			return nil, fmt.Errorf("add account balance from game wallet: %w", err)
		}

	default:
		return nil, service.ErrGameWalletInvalidDirection
	}

	if err := tx.QueryRowContext(
		ctx,
		gameWalletInsertTransactionQuery,
		userID,
		result.Direction,
		result.BalanceAmount,
		result.CreditsAmount,
		result.ExchangeRate,
		result.BalanceBefore,
		result.CreditsBefore,
		result.BalanceAfter,
		result.CreditsAfter,
		result.IdempotencyKey,
		input.RequestHash,
	).Scan(&result.ID, &result.CreatedAt); err != nil {
		return nil, fmt.Errorf("insert game wallet transaction: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit game wallet exchange: %w", err)
	}
	return result, nil
}

func (r *gameWalletRepository) ListTransactions(ctx context.Context, userID int64, limit int, beforeID int64) ([]service.GameWalletTransaction, error) {
	rows, err := r.db.QueryContext(ctx, gameWalletListTransactionsQuery, userID, limit, beforeID)
	if err != nil {
		return nil, fmt.Errorf("list game wallet transactions: %w", err)
	}
	defer rows.Close()

	items := make([]service.GameWalletTransaction, 0, limit)
	for rows.Next() {
		var item service.GameWalletTransaction
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.Direction,
			&item.BalanceAmount,
			&item.CreditsAmount,
			&item.ExchangeRate,
			&item.BalanceBefore,
			&item.CreditsBefore,
			&item.BalanceAfter,
			&item.CreditsAfter,
			&item.IdempotencyKey,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan game wallet transaction: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate game wallet transactions: %w", err)
	}
	return items, nil
}

func findGameWalletIdempotencyRecord(ctx context.Context, tx *sql.Tx, userID int64, key string) (*service.GameWalletTransaction, string, error) {
	item := &service.GameWalletTransaction{}
	var requestHash string
	err := tx.QueryRowContext(ctx, gameWalletIdempotencyQuery, userID, key).Scan(
		&item.ID,
		&item.UserID,
		&item.Direction,
		&item.BalanceAmount,
		&item.CreditsAmount,
		&item.ExchangeRate,
		&item.BalanceBefore,
		&item.CreditsBefore,
		&item.BalanceAfter,
		&item.CreditsAfter,
		&item.IdempotencyKey,
		&requestHash,
		&item.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("query game wallet idempotency record: %w", err)
	}
	return item, requestHash, nil
}

func enforceGameWalletDailyLimit(ctx context.Context, tx *sql.Tx, userID int64, balanceAmount, dailyLimit string) error {
	var used string
	var amountAllowed, transactionCountAllowed bool
	if err := tx.QueryRowContext(
		ctx,
		gameWalletDailyLimitQuery,
		userID,
		balanceAmount,
		dailyLimit,
		gameWalletMaxDailyTransactions,
	).Scan(&used, &amountAllowed, &transactionCountAllowed); err != nil {
		return fmt.Errorf("check game wallet daily limit: %w", err)
	}
	if !transactionCountAllowed {
		return service.ErrGameWalletDailyTransactionLimitExceeded
	}
	if !amountAllowed {
		return service.ErrGameWalletDailyLimitExceeded.WithMetadata(map[string]string{
			"daily_exchanged": used,
			"daily_limit":     dailyLimit,
		})
	}
	return nil
}
