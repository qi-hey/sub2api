package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	gameFruitFindIdempotencyQuery = `
SELECT id, user_id, bets, total_bet, stop_index, outcome, payout_credits,
       credits_before, credits_after, paytable_version, idempotency_key,
       request_hash, created_at
FROM game_fruit_spins
WHERE user_id = $1 AND idempotency_key = $2`

	gameFruitCountTodayQuery = `
SELECT COUNT(*) FROM game_fruit_spins
WHERE user_id = $1 AND created_at >= $2 AND created_at < $3`

	gameFruitInsertQuery = `
INSERT INTO game_fruit_spins (
    user_id, bets, total_bet, stop_index, outcome, payout_credits,
    credits_before, credits_after, paytable_version, bet_ledger_id,
    payout_ledger_id, idempotency_key, request_hash
) VALUES (
    $1, $2::jsonb, $3, $4, $5::jsonb, $6,
    $7, $8, $9, $10, $11, $12, $13
)
RETURNING id, created_at`
)

var _ service.GameFruitRepository = (*gameLoyaltyRepository)(nil)

type gameFruitQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (r *gameLoyaltyRepository) FindFruitSpin(ctx context.Context, userID int64, idempotencyKey string) (*service.FruitSpinResult, error) {
	return findFruitSpinIdempotency(ctx, r.db, userID, idempotencyKey)
}

func (r *gameLoyaltyRepository) FruitSpin(
	ctx context.Context,
	userID int64,
	input service.FruitSpinInput,
	outcome *service.FruitSpinResult,
) (*service.FruitSpinResult, error) {
	if outcome == nil || outcome.TotalBet != input.TotalBet || outcome.Bets != input.Bets {
		return nil, fmt.Errorf("fruit outcome does not match request")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin fruit spin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockGameLoyaltyUser(ctx, tx, userID); err != nil {
		return nil, err
	}
	if existing, err := findFruitSpinIdempotency(ctx, tx, userID, input.IdempotencyKey); err != nil {
		return nil, err
	} else if existing != nil {
		if existing.Bets != input.Bets || existing.TotalBet != input.TotalBet {
			return nil, service.ErrGameLoyaltyIdempotencyConflict
		}
		existing.IdempotentReplay = true
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit fruit spin replay: %w", err)
		}
		return existing, nil
	}

	if _, err := tx.ExecContext(ctx, gameLoyaltyEnsureQuery, userID); err != nil {
		return nil, fmt.Errorf("ensure fruit wallet: %w", err)
	}
	creditsBefore, _, err := lockGameLoyaltyWallet(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	dayStart := timezone.Today()
	dayEnd := dayStart.AddDate(0, 0, 1)
	var spinCount int64
	if err := tx.QueryRowContext(ctx, gameFruitCountTodayQuery, userID, dayStart, dayEnd).Scan(&spinCount); err != nil {
		return nil, fmt.Errorf("count daily fruit spins: %w", err)
	}
	if spinCount >= gameLoyaltyMaxDailySpins {
		return nil, service.ErrGameLoyaltySpinLimitExceeded
	}

	var creditsAfterBet int64
	if err := tx.QueryRowContext(ctx, gameLoyaltyDeductCreditsQuery, userID, input.TotalBet).Scan(&creditsAfterBet); errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrGameLoyaltyInsufficientCredits
	} else if err != nil {
		return nil, fmt.Errorf("deduct fruit bet: %w", err)
	}
	betMeta, _ := json.Marshal(map[string]any{"bets": input.Bets, "total_bet": input.TotalBet, "paytable_version": outcome.PaytableVersion})
	betLedgerID, _, err := insertLedger(
		ctx, tx, userID, service.GameLedgerEntryFruitBet, -input.TotalBet,
		creditsBefore, creditsAfterBet, "fruit_spin", nil,
		gameLoyaltyLedgerIdempotencyKey("fruit_bet", input.IdempotencyKey), input.RequestHash, string(betMeta),
	)
	if err != nil {
		return nil, err
	}

	creditsAfter := creditsAfterBet
	var payoutLedgerArg any
	if outcome.Payout > 0 {
		if err := tx.QueryRowContext(ctx, gameLoyaltyAddCreditsQuery, userID, outcome.Payout).Scan(&creditsAfter); errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrGameLoyaltyCreditsLimitExceeded
		} else if err != nil {
			return nil, fmt.Errorf("add fruit payout: %w", err)
		}
		payoutMeta, _ := json.Marshal(map[string]any{
			"stop_index": outcome.StopIndex, "outcome": outcome.Outcome,
			"payout": outcome.Payout, "paytable_version": outcome.PaytableVersion,
		})
		payoutLedgerID, _, err := insertLedger(
			ctx, tx, userID, service.GameLedgerEntryFruitPayout, outcome.Payout,
			creditsAfterBet, creditsAfter, "fruit_spin", nil,
			gameLoyaltyLedgerIdempotencyKey("fruit_payout", input.IdempotencyKey), input.RequestHash, string(payoutMeta),
		)
		if err != nil {
			return nil, err
		}
		payoutLedgerArg = payoutLedgerID
	}

	betsJSON, err := json.Marshal(input.Bets)
	if err != nil {
		return nil, fmt.Errorf("marshal fruit bets: %w", err)
	}
	outcomeJSON, err := json.Marshal(outcome.Outcome)
	if err != nil {
		return nil, fmt.Errorf("marshal fruit outcome: %w", err)
	}
	result := *outcome
	result.UserID = userID
	result.CreditsBefore = creditsBefore
	result.CreditsAfter = creditsAfter
	result.IdempotencyKey = input.IdempotencyKey
	if err := tx.QueryRowContext(
		ctx, gameFruitInsertQuery, userID, string(betsJSON), input.TotalBet,
		result.StopIndex, string(outcomeJSON), result.Payout, creditsBefore, creditsAfter,
		result.PaytableVersion, betLedgerID, payoutLedgerArg, input.IdempotencyKey, input.RequestHash,
	).Scan(&result.ID, &result.CreatedAt); err != nil {
		return nil, fmt.Errorf("insert fruit spin: %w", err)
	}

	localDate := input.LocalDate
	if localDate == "" {
		localDate = timezone.Today().Format("2006-01-02")
	}
	if _, err := tx.ExecContext(ctx, gameDailyScoreUpsertQuery, userID, localDate, service.GameIDMerge2048, result.Payout, result.CreatedAt); err != nil {
		return nil, fmt.Errorf("upsert fruit score: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit fruit spin: %w", err)
	}
	return &result, nil
}

func findFruitSpinIdempotency(ctx context.Context, queryer gameFruitQueryer, userID int64, key string) (*service.FruitSpinResult, error) {
	item := &service.FruitSpinResult{}
	var betsJSON, outcomeJSON []byte
	var requestHash string
	err := queryer.QueryRowContext(ctx, gameFruitFindIdempotencyQuery, userID, key).Scan(
		&item.ID, &item.UserID, &betsJSON, &item.TotalBet, &item.StopIndex, &outcomeJSON,
		&item.Payout, &item.CreditsBefore, &item.CreditsAfter, &item.PaytableVersion,
		&item.IdempotencyKey, &requestHash, &item.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find fruit spin idempotency: %w", err)
	}
	if err := json.Unmarshal(betsJSON, &item.Bets); err != nil {
		return nil, fmt.Errorf("decode fruit bets: %w", err)
	}
	if err := json.Unmarshal(outcomeJSON, &item.Outcome); err != nil {
		return nil, fmt.Errorf("decode fruit outcome: %w", err)
	}
	return item, nil
}
