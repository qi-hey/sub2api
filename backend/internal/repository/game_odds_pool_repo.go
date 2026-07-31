package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	gameOddsPoolEnsureQuery = `
INSERT INTO game_odds_pools (
    game_id, algorithm_version, current_profile,
    period_started_at, period_ends_at
) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (game_id) DO NOTHING`

	gameOddsPoolReadQuery = `
	SELECT game_id, algorithm_version, current_profile,
	       period_started_at, period_ends_at,
	       total_bet_credits, total_payout_credits, total_rounds,
	       period_bet_credits, period_payout_credits, period_rounds
FROM game_odds_pools
WHERE game_id = $1`

	gameOddsPoolLockQuery = gameOddsPoolReadQuery + ` FOR UPDATE`

	gameOddsPeriodInsertQuery = `
INSERT INTO game_odds_periods (
    game_id, algorithm_version, profile, next_profile,
    period_started_at, period_ends_at,
    bet_credits, payout_credits, rounds,
    total_bet_credits, total_payout_credits
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (game_id, period_started_at) DO NOTHING`

	gameOddsPoolRotateQuery = `
UPDATE game_odds_pools
SET algorithm_version = $2,
    current_profile = $3,
    period_started_at = $4,
    period_ends_at = $5,
    period_bet_credits = 0,
    period_payout_credits = 0,
    period_rounds = 0,
    updated_at = NOW()
WHERE game_id = $1`
)

var _ service.GameOddsPoolRepository = (*gameLoyaltyRepository)(nil)

func (r *gameLoyaltyRepository) ResolveGameOddsPolicy(
	ctx context.Context,
	gameID string,
	now time.Time,
) (*service.GameOddsPolicy, error) {
	gameID, ok := service.NormalizeGameOddsPoolID(gameID)
	if !ok {
		return nil, fmt.Errorf("invalid game odds pool id")
	}
	now = now.UTC()
	periodStart := now.Truncate(service.GameOddsPeriod)
	periodEnd := periodStart.Add(service.GameOddsPeriod)
	if _, err := r.db.ExecContext(
		ctx, gameOddsPoolEnsureQuery, gameID, service.GameOddsAlgorithmVersion,
		service.GameOddsProfileStandard, periodStart, periodEnd,
	); err != nil {
		return nil, fmt.Errorf("ensure game odds pool: %w", err)
	}

	policy, err := readGameOddsPolicy(ctx, r.db, gameID, gameOddsPoolReadQuery)
	if err != nil {
		return nil, err
	}
	if now.Before(policy.PeriodEndsAt) {
		return policy, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin game odds pool rotation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	locked, err := readGameOddsPolicy(ctx, tx, gameID, gameOddsPoolLockQuery)
	if err != nil {
		return nil, err
	}
	if now.Before(locked.PeriodEndsAt) {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit game odds pool concurrent rotation: %w", err)
		}
		return locked, nil
	}

	nextProfile := service.SelectNextGameOddsProfile(locked.Profile, locked.Metrics)
	if _, err := tx.ExecContext(
		ctx, gameOddsPeriodInsertQuery,
		locked.GameID, locked.AlgorithmVersion, locked.Profile, nextProfile,
		locked.PeriodStartedAt, locked.PeriodEndsAt,
		locked.Metrics.PeriodBetCredits, locked.Metrics.PeriodPayoutCredits,
		locked.Metrics.PeriodRounds, locked.Metrics.TotalBetCredits,
		locked.Metrics.TotalPayoutCredits,
	); err != nil {
		return nil, fmt.Errorf("audit game odds period: %w", err)
	}
	if _, err := tx.ExecContext(
		ctx, gameOddsPoolRotateQuery, gameID, service.GameOddsAlgorithmVersion,
		nextProfile, periodStart, periodEnd,
	); err != nil {
		return nil, fmt.Errorf("rotate game odds pool: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit game odds pool rotation: %w", err)
	}

	locked.Profile = nextProfile
	locked.AlgorithmVersion = service.GameOddsAlgorithmVersion
	locked.PeriodStartedAt = periodStart
	locked.PeriodEndsAt = periodEnd
	locked.Metrics.PeriodBetCredits = 0
	locked.Metrics.PeriodPayoutCredits = 0
	locked.Metrics.PeriodRounds = 0
	return locked, nil
}

type gameOddsPolicyQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func readGameOddsPolicy(
	ctx context.Context,
	queryer gameOddsPolicyQueryer,
	gameID, query string,
) (*service.GameOddsPolicy, error) {
	policy := &service.GameOddsPolicy{}
	var rawProfile string
	err := queryer.QueryRowContext(ctx, query, gameID).Scan(
		&policy.GameID, &policy.AlgorithmVersion, &rawProfile,
		&policy.PeriodStartedAt, &policy.PeriodEndsAt,
		&policy.Metrics.TotalBetCredits, &policy.Metrics.TotalPayoutCredits,
		&policy.Metrics.TotalRounds,
		&policy.Metrics.PeriodBetCredits, &policy.Metrics.PeriodPayoutCredits,
		&policy.Metrics.PeriodRounds,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("game odds pool %q was not created", gameID)
	}
	if err != nil {
		return nil, fmt.Errorf("read game odds pool: %w", err)
	}
	policy.Profile = service.NormalizeGameOddsProfile(service.GameOddsProfile(rawProfile))
	return policy, nil
}
