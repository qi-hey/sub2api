package service

import (
	"context"
	"log/slog"
	"math/big"
	"regexp"
	"strings"
	"time"
)

const (
	GameOddsPoolFruit = "fruit_machine"
	GameOddsPoolSlot  = "five_reel_slot"

	GameOddsAlgorithmVersion = "global-pool-v2"
	GameOddsPeriod           = 15 * time.Minute

	GameOddsPeriodMinRounds       int64 = 20
	GameOddsPeriodMinBetCredits   int64 = 100_000
	GameOddsLifetimeMinRounds     int64 = 100
	GameOddsLifetimeMinBetCredits int64 = 1_000_000
)

type GameOddsProfile string

const (
	GameOddsProfileStandard GameOddsProfile = "standard"
	GameOddsProfileCooling  GameOddsProfile = "cooling"
	GameOddsProfileTight    GameOddsProfile = "tight"
)

var gameOddsPoolIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

type GameOddsMetrics struct {
	TotalBetCredits     int64
	TotalPayoutCredits  int64
	TotalRounds         int64
	PeriodBetCredits    int64
	PeriodPayoutCredits int64
	PeriodRounds        int64
}

type GameOddsPolicy struct {
	GameID           string
	Profile          GameOddsProfile
	AlgorithmVersion string
	PeriodStartedAt  time.Time
	PeriodEndsAt     time.Time
	Metrics          GameOddsMetrics
}

// GameOddsPoolRepository is optional so existing test doubles and external
// implementations continue to use the unchanged standard odds profile.
type GameOddsPoolRepository interface {
	ResolveGameOddsPolicy(ctx context.Context, gameID string, now time.Time) (*GameOddsPolicy, error)
}

func NormalizeGameOddsPoolID(gameID string) (string, bool) {
	gameID = strings.ToLower(strings.TrimSpace(gameID))
	return gameID, gameOddsPoolIDPattern.MatchString(gameID)
}

func NormalizeGameOddsProfile(profile GameOddsProfile) GameOddsProfile {
	switch profile {
	case GameOddsProfileStandard, GameOddsProfileCooling, GameOddsProfileTight:
		return profile
	default:
		return GameOddsProfileStandard
	}
}

// SelectNextGameOddsProfile evaluates one game pool only. It never receives a
// user ID, so account history cannot influence an individual player's odds.
// Changes are limited to one level per period to avoid abrupt oscillation.
func SelectNextGameOddsProfile(current GameOddsProfile, metrics GameOddsMetrics) GameOddsProfile {
	current = NormalizeGameOddsProfile(current)
	desired := current

	periodReady := metrics.PeriodRounds >= GameOddsPeriodMinRounds &&
		metrics.PeriodBetCredits >= GameOddsPeriodMinBetCredits
	if periodReady {
		switch {
		case ratioGreater(metrics.PeriodPayoutCredits, metrics.PeriodBetCredits, 115, 100):
			desired = GameOddsProfileTight
		case ratioGreater(metrics.PeriodPayoutCredits, metrics.PeriodBetCredits, 98, 100):
			desired = GameOddsProfileCooling
		case ratioLess(metrics.PeriodPayoutCredits, metrics.PeriodBetCredits, 82, 100):
			desired = GameOddsProfileStandard
		}
	}

	lifetimeReady := metrics.TotalRounds >= GameOddsLifetimeMinRounds &&
		metrics.TotalBetCredits >= GameOddsLifetimeMinBetCredits
	if lifetimeReady {
		switch {
		case ratioGreater(metrics.TotalPayoutCredits, metrics.TotalBetCredits, 108, 100):
			desired = maxGameOddsProfile(desired, GameOddsProfileTight)
		case ratioGreater(metrics.TotalPayoutCredits, metrics.TotalBetCredits, 96, 100):
			desired = maxGameOddsProfile(desired, GameOddsProfileCooling)
		case ratioLess(metrics.TotalPayoutCredits, metrics.TotalBetCredits, 90, 100) &&
			(!periodReady || ratioLess(metrics.PeriodPayoutCredits, metrics.PeriodBetCredits, 90, 100)):
			desired = GameOddsProfileStandard
		}
	}

	return stepGameOddsProfile(current, desired)
}

func (s *GameLoyaltyService) gameOddsProfile(ctx context.Context, gameID string) GameOddsProfile {
	repo, ok := s.repo.(GameOddsPoolRepository)
	if !ok {
		return GameOddsProfileStandard
	}
	policy, err := repo.ResolveGameOddsPolicy(ctx, gameID, time.Now().UTC())
	if err != nil {
		slog.Warn("game odds pool unavailable; using standard profile", "game_id", gameID, "error", err)
		return GameOddsProfileStandard
	}
	if policy == nil {
		return GameOddsProfileStandard
	}
	return NormalizeGameOddsProfile(policy.Profile)
}

func ratioGreater(value, base, numerator, denominator int64) bool {
	if base <= 0 {
		return value > 0
	}
	left := new(big.Int).Mul(big.NewInt(value), big.NewInt(denominator))
	right := new(big.Int).Mul(big.NewInt(base), big.NewInt(numerator))
	return left.Cmp(right) > 0
}

func ratioLess(value, base, numerator, denominator int64) bool {
	if base <= 0 {
		return false
	}
	left := new(big.Int).Mul(big.NewInt(value), big.NewInt(denominator))
	right := new(big.Int).Mul(big.NewInt(base), big.NewInt(numerator))
	return left.Cmp(right) < 0
}

func gameOddsProfileRank(profile GameOddsProfile) int {
	switch NormalizeGameOddsProfile(profile) {
	case GameOddsProfileCooling:
		return 1
	case GameOddsProfileTight:
		return 2
	default:
		return 0
	}
}

func gameOddsProfileAtRank(rank int) GameOddsProfile {
	switch rank {
	case 1:
		return GameOddsProfileCooling
	case 2:
		return GameOddsProfileTight
	default:
		return GameOddsProfileStandard
	}
}

func maxGameOddsProfile(a, b GameOddsProfile) GameOddsProfile {
	if gameOddsProfileRank(b) > gameOddsProfileRank(a) {
		return NormalizeGameOddsProfile(b)
	}
	return NormalizeGameOddsProfile(a)
}

func stepGameOddsProfile(current, desired GameOddsProfile) GameOddsProfile {
	currentRank := gameOddsProfileRank(current)
	desiredRank := gameOddsProfileRank(desired)
	if desiredRank > currentRank+1 {
		desiredRank = currentRank + 1
	}
	if desiredRank < currentRank-1 {
		desiredRank = currentRank - 1
	}
	return gameOddsProfileAtRank(desiredRank)
}
