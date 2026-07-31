package service

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGameOddsPoolsAreSeparatedByGame(t *testing.T) {
	require.NotEqual(t, GameOddsPoolFruit, GameOddsPoolSlot)
	for _, gameID := range []string{GameOddsPoolFruit, GameOddsPoolSlot, "future-game.v1"} {
		normalized, ok := NormalizeGameOddsPoolID(gameID)
		require.True(t, ok)
		require.Equal(t, gameID, normalized)
	}
	_, ok := NormalizeGameOddsPoolID("../shared")
	require.False(t, ok)
}

func TestSelectNextGameOddsProfileUsesOnlyPoolMetrics(t *testing.T) {
	highPeriod := GameOddsMetrics{
		TotalBetCredits: 1_500_000, TotalPayoutCredits: 1_350_000, TotalRounds: 150,
		PeriodBetCredits: 200_000, PeriodPayoutCredits: 240_000, PeriodRounds: 30,
	}
	require.Equal(t, GameOddsProfileCooling, SelectNextGameOddsProfile(GameOddsProfileStandard, highPeriod))
	require.Equal(t, GameOddsProfileTight, SelectNextGameOddsProfile(GameOddsProfileCooling, highPeriod))

	healthy := GameOddsMetrics{
		TotalBetCredits: 1_500_000, TotalPayoutCredits: 1_275_000, TotalRounds: 150,
		PeriodBetCredits: 200_000, PeriodPayoutCredits: 140_000, PeriodRounds: 30,
	}
	require.Equal(t, GameOddsProfileCooling, SelectNextGameOddsProfile(GameOddsProfileTight, healthy))
	require.Equal(t, GameOddsProfileStandard, SelectNextGameOddsProfile(GameOddsProfileCooling, healthy))

	// Sparse windows retain the current profile rather than reacting to one win.
	sparse := GameOddsMetrics{PeriodBetCredits: 10, PeriodPayoutCredits: 100, PeriodRounds: 1}
	require.Equal(t, GameOddsProfileStandard, SelectNextGameOddsProfile(GameOddsProfileStandard, sparse))
}

func TestSelectNextGameOddsProfileRequiresEnoughRoundsAndCredits(t *testing.T) {
	oneLargeSpin := GameOddsMetrics{
		TotalBetCredits: 1_000_000, TotalPayoutCredits: 1_200_000, TotalRounds: 1,
		PeriodBetCredits: 1_000_000, PeriodPayoutCredits: 1_200_000, PeriodRounds: 1,
	}
	require.Equal(t, GameOddsProfileStandard,
		SelectNextGameOddsProfile(GameOddsProfileStandard, oneLargeSpin))

	tooFewCredits := GameOddsMetrics{
		TotalBetCredits: 999_999, TotalPayoutCredits: 1_200_000, TotalRounds: 100,
		PeriodBetCredits: 99_999, PeriodPayoutCredits: 120_000, PeriodRounds: 20,
	}
	require.Equal(t, GameOddsProfileStandard,
		SelectNextGameOddsProfile(GameOddsProfileStandard, tooFewCredits))

	ready := GameOddsMetrics{
		TotalBetCredits: 1_000_000, TotalPayoutCredits: 1_200_000, TotalRounds: 100,
		PeriodBetCredits: 100_000, PeriodPayoutCredits: 120_000, PeriodRounds: 20,
	}
	require.Equal(t, GameOddsProfileCooling,
		SelectNextGameOddsProfile(GameOddsProfileStandard, ready))
}

func TestFruitOddsProfilesKeepBoardAndReduceRTPMonotonically(t *testing.T) {
	profiles := []GameOddsProfile{GameOddsProfileStandard, GameOddsProfileCooling, GameOddsProfileTight}
	rtps := make(map[GameOddsProfile][]float64, len(profiles))
	for _, profile := range profiles {
		cells := FruitBoardForOddsProfile(profile)
		require.Len(t, cells, FruitBoardSize)
		totalWeight, specialWeight := 0, 0
		for index, cell := range cells {
			require.Equal(t, index, cell.Index)
			require.Greater(t, cell.Weight, 0)
			totalWeight += cell.Weight
			if cell.Kind == "special" {
				specialWeight += cell.Weight
			}
		}
		require.Equal(t, 10_000, totalWeight)
		require.Equal(t, fruitSpecialWeightForOddsProfile(profile), specialWeight)
		rtps[profile] = fruitProfileRTPByDoor(cells, specialWeight)
	}

	for door := 0; door < FruitDoorCount; door++ {
		require.Greater(t, rtps[GameOddsProfileStandard][door], rtps[GameOddsProfileCooling][door], FruitDoors[door])
		require.Greater(t, rtps[GameOddsProfileCooling][door], rtps[GameOddsProfileTight][door], FruitDoors[door])
		require.Greater(t, rtps[GameOddsProfileTight][door], 0.65, FruitDoors[door])
	}
}

func TestSlotOddsProfilesOnlyTightenSharedFeatureRates(t *testing.T) {
	standard := slotPolicyForOddsProfile(GameOddsProfileStandard)
	cooling := slotPolicyForOddsProfile(GameOddsProfileCooling)
	tight := slotPolicyForOddsProfile(GameOddsProfileTight)

	require.Equal(t, SlotBonusTriggerNumerator, standard.bonusTriggerNumerator)
	require.Greater(t, standard.bonusTriggerNumerator, cooling.bonusTriggerNumerator)
	require.Greater(t, cooling.bonusTriggerNumerator, tight.bonusTriggerNumerator)
	require.Greater(t, float64(standard.lowTierKeepNumerator)/float64(standard.lowTierKeepDenominator),
		float64(cooling.lowTierKeepNumerator)/float64(cooling.lowTierKeepDenominator))
	require.Greater(t, float64(cooling.lowTierKeepNumerator)/float64(cooling.lowTierKeepDenominator),
		float64(tight.lowTierKeepNumerator)/float64(tight.lowTierKeepDenominator))
}

func fruitProfileRTPByDoor(cells []FruitBoardCell, specialWeight int) []float64 {
	direct := make([]int64, FruitDoorCount)
	for _, cell := range cells {
		if cell.Kind != "fruit" {
			continue
		}
		for door, name := range FruitDoors {
			if cell.Symbol == name {
				direct[door] += int64(cell.Weight) * cell.Multiplier
			}
		}
	}
	standardRewardDirect := make([]int64, FruitDoorCount)
	for _, cell := range FruitBoard() {
		if cell.Kind != "fruit" {
			continue
		}
		for door, name := range FruitDoors {
			if cell.Symbol == name {
				standardRewardDirect[door] += int64(cell.Weight) * cell.Multiplier
			}
		}
	}

	result := make([]float64, FruitDoorCount)
	for door := range FruitDoorCount {
		rtp := new(big.Rat).SetFrac64(direct[door], 10_000)
		rewardValue := new(big.Rat).SetFrac64(standardRewardDirect[door], 9_000)
		send := new(big.Rat).SetFrac64(int64(specialWeight*800), 10_000*1_000)
		send.Mul(send, big.NewRat(3, 2)).Mul(send, rewardValue)
		mary := new(big.Rat).SetFrac64(int64(specialWeight*150), 10_000*1_000)
		mary.Mul(mary, big.NewRat(3, 1)).Mul(mary, rewardValue)
		jackpot := new(big.Rat).SetFrac64(int64(specialWeight*(30*8+15*25+5*80)), 10_000*1_000)
		rtp.Add(rtp, send).Add(rtp, mary).Add(rtp, jackpot)
		result[door], _ = rtp.Float64()
	}
	return result
}
