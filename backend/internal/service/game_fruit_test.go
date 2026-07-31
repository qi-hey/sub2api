package service

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

type fruitServiceRepoStub struct {
	*loyaltyRepoStub
	existing       *FruitSpinResult
	findKey        string
	findCalls      int
	fruitSpinIn    FruitSpinInput
	fruitSpinCalls int
}

func (s *fruitServiceRepoStub) FindFruitSpin(_ context.Context, _ int64, idempotencyKey string) (*FruitSpinResult, error) {
	s.findCalls++
	s.findKey = idempotencyKey
	if s.existing == nil {
		return nil, nil
	}
	result := *s.existing
	return &result, nil
}

func (s *fruitServiceRepoStub) FruitSpin(_ context.Context, userID int64, input FruitSpinInput, outcome *FruitSpinResult) (*FruitSpinResult, error) {
	s.fruitSpinCalls++
	s.fruitSpinIn = input
	result := *outcome
	result.UserID = userID
	result.IdempotencyKey = input.IdempotencyKey
	return &result, nil
}

type rejectingFruitRandReader struct{ reads int }

func (r *rejectingFruitRandReader) Read([]byte) (int, error) {
	r.reads++
	return 0, errors.New("fruit RNG must not be read during replay")
}

func newFruitServiceSettings() *SettingService {
	return &SettingService{settingRepo: &gameWalletSettingRepoStub{values: map[string]string{
		SettingKeyGameLoyaltyEnabled:          "true",
		SettingKeyGameLoyaltyCheckinCredits:   "50",
		SettingKeyGameLoyaltySlotBetCredits:   "10",
		SettingKeyGameLoyaltyDailyRewardLimit: "5",
		SettingKeyGameLoyaltyRewardCatalog:    "[]",
	}}}
}

func TestFruitBoardContractAndRTP(t *testing.T) {
	cells := FruitBoard()
	require.Len(t, cells, FruitBoardSize)
	expected := []struct {
		kind, symbol, size string
		multiplier         int64
		weight             int
	}{
		{"fruit", "orange", "big", 20, 91},
		{"fruit", "bell", "big", 20, 91},
		{"fruit", "bar", "big", 60, 20},
		{"fruit", "bar", "big", 120, 24},
		{"fruit", "bar", "big", 80, 36},
		{"fruit", "apple", "big", 6, 289},
		{"fruit", "mango", "big", 20, 91},
		{"fruit", "watermelon", "big", 40, 84},
		{"fruit", "watermelon", "small", 3, 1210},
		{"special", "", "", 0, 500},
		{"fruit", "apple", "big", 6, 289},
		{"fruit", "orange", "small", 3, 1112},
		{"fruit", "orange", "big", 20, 91},
		{"fruit", "bell", "big", 20, 91},
		{"fruit", "77", "small", 3, 1210},
		{"fruit", "77", "big", 40, 84},
		{"fruit", "apple", "big", 6, 289},
		{"fruit", "mango", "small", 3, 1112},
		{"fruit", "mango", "big", 20, 91},
		{"fruit", "star", "big", 40, 84},
		{"fruit", "star", "small", 3, 1210},
		{"special", "", "", 0, 500},
		{"fruit", "apple", "big", 6, 289},
		{"fruit", "bell", "small", 3, 1112},
	}
	weight, fruitWeight := 0, 0
	for index, cell := range cells {
		require.Equal(t, index, cell.Index)
		require.Equal(t, expected[index].kind, cell.Kind)
		require.Equal(t, expected[index].symbol, cell.Symbol)
		require.Equal(t, expected[index].size, cell.Size)
		require.Equal(t, expected[index].multiplier, cell.Multiplier)
		require.Equal(t, expected[index].weight, cell.Weight)
		weight += cell.Weight
		if cell.Kind == "fruit" {
			fruitWeight += cell.Weight
		}
	}
	require.Equal(t, 10_000, weight)
	require.Equal(t, 9_000, fruitWeight)

	directNumerators := [FruitDoorCount]int64{}
	visibleMultipliers := map[int64]bool{}
	for _, cell := range cells {
		if cell.Kind != "fruit" {
			continue
		}
		visibleMultipliers[cell.Multiplier] = true
		for door, name := range FruitDoors {
			if cell.Symbol == name {
				directNumerators[door] += int64(cell.Weight) * cell.Multiplier
			}
		}
	}
	for _, multiplier := range []int64{3, 6, 20, 40, 60, 80, 120} {
		require.True(t, visibleMultipliers[multiplier], multiplier)
	}

	for door := range FruitDoorCount {
		baseNumerator := directNumerators[door]
		rtp := new(big.Rat).SetFrac64(baseNumerator, 10_000)
		extraDrawValue := new(big.Rat).SetFrac64(baseNumerator, 9_000)
		send := new(big.Rat).Mul(new(big.Rat).SetFrac64(800, 10_000), new(big.Rat).SetFrac64(3, 2))
		send.Mul(send, extraDrawValue)
		mary := new(big.Rat).Mul(new(big.Rat).SetFrac64(150, 10_000), big.NewRat(3, 1))
		mary.Mul(mary, extraDrawValue)
		jackpot := new(big.Rat).Add(
			new(big.Rat).SetFrac64(30*8+15*25, 10_000),
			new(big.Rat).SetFrac64(5*80, 10_000),
		)
		rtp.Add(rtp, send).Add(rtp, mary).Add(rtp, jackpot)
		value, _ := rtp.Float64()
		require.GreaterOrEqual(t, value, 0.92, FruitDoors[door])
		require.LessOrEqual(t, value, 0.94, FruitDoors[door])
		require.LessOrEqual(t, value, 1.0, FruitDoors[door])
	}
}

func TestFruitEngineNormalSendLightAndJackpot(t *testing.T) {
	bets := [FruitDoorCount]int64{}
	bets[6] = 1 // orange

	normal, err := NewFruitEngine(newFixedRandReader(0)).Spin(bets)
	require.NoError(t, err)
	require.Equal(t, 0, normal.StopIndex)
	require.Equal(t, int64(20), normal.Payout)
	require.Equal(t, FruitPaytableVersion, normal.PaytableVersion)

	send, err := NewFruitEngine(newFixedRandReader(1_936, 0, 0, 0)).Spin(bets)
	require.NoError(t, err)
	require.Equal(t, 9, send.StopIndex)
	require.Equal(t, "send_light", send.Outcome.Kind)
	require.Equal(t, 14, send.Outcome.CenterIndex)
	require.Len(t, send.Outcome.ExtraStops, 1)
	require.Equal(t, int64(20), send.Payout)

	mary, err := NewFruitEngine(newFixedRandReader(1_936, 800, 0, 0, 0)).Spin(bets)
	require.NoError(t, err)
	require.Equal(t, "little_mary", mary.Outcome.Kind)
	require.Equal(t, 10, mary.Outcome.CenterIndex)
	require.Len(t, mary.Outcome.LittleMaryStops, 3)
	require.Equal(t, int64(60), mary.Payout)

	jackpot, err := NewFruitEngine(newFixedRandReader(1_936, 999)).Spin(bets)
	require.NoError(t, err)
	require.Equal(t, 9, jackpot.StopIndex)
	require.Equal(t, "gold", jackpot.Outcome.JackpotTier)
	require.Equal(t, 4, jackpot.Outcome.CenterIndex)
	require.Equal(t, int64(80), jackpot.Payout)
}

func TestFruitBetLimits(t *testing.T) {
	_, _, err := normalizeFruitBets(FruitBetRequest{"apple": 0})
	require.ErrorIs(t, err, ErrGameFruitInvalidBets)
	_, _, err = normalizeFruitBets(FruitBetRequest{"apple": 101})
	require.ErrorIs(t, err, ErrGameFruitInvalidBets)
	_, _, err = normalizeFruitBets(FruitBetRequest{"unknown": 1})
	require.ErrorIs(t, err, ErrGameFruitInvalidBets)
	_, _, err = normalizeFruitBets(FruitBetRequest{"apple": 1, " APPLE ": 2})
	require.ErrorIs(t, err, ErrGameFruitInvalidBets)

	bets, total, err := normalizeFruitBets(FruitBetRequest{"bar": 2, "apple": 3})
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Equal(t, int64(2), bets[0])
	require.Equal(t, int64(3), bets[7])
}

func TestFruitSpinReplaysPersistedOutcomeBeforeRNG(t *testing.T) {
	bets := [FruitDoorCount]int64{}
	bets[7] = 3
	repo := &fruitServiceRepoStub{
		loyaltyRepoStub: &loyaltyRepoStub{},
		existing: &FruitSpinResult{
			ID: 9, UserID: 7, Bets: bets, TotalBet: 3, StopIndex: 7,
			Payout: 15, PaytableVersion: "fruit-v0", IdempotencyKey: "fruit-replay",
		},
	}
	rand := &rejectingFruitRandReader{}
	svc := NewGameLoyaltyService(repo, newFruitServiceSettings(), nil, nil).
		WithFruitEngine(NewFruitEngine(rand))

	result, err := svc.FruitSpin(context.Background(), 7, "fruit-replay", FruitBetRequest{" APPLE ": 3})
	require.NoError(t, err)
	require.True(t, result.IdempotentReplay)
	require.Equal(t, "fruit-v0", result.PaytableVersion)
	require.Equal(t, "fruit-replay", repo.findKey)
	require.Equal(t, 1, repo.findCalls)
	require.Zero(t, repo.fruitSpinCalls)
	require.Zero(t, rand.reads)
}

func TestFruitSpinRejectsReplayWithDifferentNormalizedBetsBeforeRNG(t *testing.T) {
	persistedBets := [FruitDoorCount]int64{}
	persistedBets[7] = 2
	repo := &fruitServiceRepoStub{
		loyaltyRepoStub: &loyaltyRepoStub{},
		existing:        &FruitSpinResult{Bets: persistedBets, TotalBet: 2, PaytableVersion: "fruit-v0"},
	}
	rand := &rejectingFruitRandReader{}
	svc := NewGameLoyaltyService(repo, newFruitServiceSettings(), nil, nil).
		WithFruitEngine(NewFruitEngine(rand))

	_, err := svc.FruitSpin(context.Background(), 7, "fruit-conflict", FruitBetRequest{"apple": 3})
	require.ErrorIs(t, err, ErrGameLoyaltyIdempotencyConflict)
	require.Zero(t, repo.fruitSpinCalls)
	require.Zero(t, rand.reads)
}
