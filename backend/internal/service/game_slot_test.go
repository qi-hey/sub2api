package service

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type slotSimulationRandReader struct {
	rng *rand.Rand
}

func (r *slotSimulationRandReader) Read(p []byte) (int, error) {
	return r.rng.Read(p)
}

func TestCryptoUniformIntRejectionSampling(t *testing.T) {
	const n = 3
	limit := uint32(0xffffffff) - (uint32(0xffffffff) % uint32(n))
	r := newFixedRandReader(limit, 0)
	got, err := cryptoUniformInt(r, n)
	require.NoError(t, err)
	require.Equal(t, 0, got)

	r = newFixedRandReader(1, 2, 3, 4, 5)
	counts := map[int]int{}
	for i := 0; i < 5; i++ {
		v, err := cryptoUniformInt(r, n)
		require.NoError(t, err)
		require.GreaterOrEqual(t, v, 0)
		require.Less(t, v, n)
		counts[v]++
	}
	require.NotEmpty(t, counts)
}

func TestCryptoUniformIntRejectsInvalidN(t *testing.T) {
	_, err := cryptoUniformInt(newFixedRandReader(1), 0)
	require.Error(t, err)
}

func TestSlotBetOptionsReachTenThousandAndRejectHigherBets(t *testing.T) {
	options := SlotBetOptions(10)
	require.Equal(t, int64(1), options[0])
	require.Equal(t, SlotBetMaxCredits, options[len(options)-1])
	require.Contains(t, options, int64(200))
	require.Contains(t, options, int64(500))
	require.Contains(t, options, int64(1_000))
	require.Contains(t, options, int64(2_000))
	require.Contains(t, options, int64(5_000))
	require.True(t, IsSupportedSlotBet(SlotBetMaxCredits, 10))
	require.False(t, IsSupportedSlotBet(SlotBetMaxCredits+1, 10))
	require.NotContains(t, SlotBetOptions(SlotBetMaxCredits+1), SlotBetMaxCredits+1)
}

func TestSlotV10SimulatedSpinOutcomeDistribution(t *testing.T) {
	const spins = 100_000
	engine := NewSlotEngine(&slotSimulationRandReader{rng: rand.New(rand.NewSource(20260730))})

	var payoutHits, bellHits, jadeHits, bonusTriggers int
	winningLineHits := make(map[string]int)
	for spin := 0; spin < spins; spin++ {
		result, err := engine.Spin(10)
		require.NoError(t, err)
		if result.TotalPayout > 0 {
			payoutHits++
		}
		if result.BonusCount >= SlotBonusTriggerMin {
			bonusTriggers++
		}
		for _, line := range result.WinningLines {
			winningLineHits[fmt.Sprintf("%s%d", line.Symbol, line.Count)]++
			switch line.Symbol {
			case SlotSymbolBell:
				bellHits++
			case SlotSymbolJade:
				jadeHits++
			}
		}
	}

	payoutHitRate := float64(payoutHits) / spins
	bellHitRate := float64(bellHits) / spins
	jadeHitRate := float64(jadeHits) / spins
	bonusTriggerRate := float64(bonusTriggers) / spins
	t.Logf(
		"slot simulated %d spins: payout_hit_rate=%.4f bell_hit_rate=%.4f jade_hit_rate=%.4f bonus_trigger_rate=%.4f",
		spins,
		payoutHitRate,
		bellHitRate,
		jadeHitRate,
		bonusTriggerRate,
	)
	t.Logf("slot simulated winning_line_rates=%#v", slotSimulationLineRates(winningLineHits, spins))
	require.GreaterOrEqual(t, payoutHitRate, 0.30)
	require.LessOrEqual(t, payoutHitRate, 0.45)
	require.LessOrEqual(t, bellHitRate, 0.25)
	require.LessOrEqual(t, jadeHitRate, 0.16)
	require.InDelta(t, SlotBonusTriggerProbability(), bonusTriggerRate, 0.005)

}

func TestSlotV11RawNormalOutcomeComposition(t *testing.T) {
	const spins = 100_000
	engine := NewSlotEngine(&slotSimulationRandReader{rng: rand.New(rand.NewSource(20260731))})

	var payoutHits, lowTierHits, nonFeatureLosses, specialBoards int
	for spin := 0; spin < spins; spin++ {
		stops, err := engine.drawStopsWithoutBonusTrigger()
		require.NoError(t, err)
		result, err := EvaluateSlotSpinV2(10, stops)
		require.NoError(t, err)
		if result.TotalPayout > 0 {
			payoutHits++
		}
		if slotIsLowTierWin(result) {
			lowTierHits++
		}
		if slotIsNonFeatureLoss(result) {
			nonFeatureLosses++
		}
		if result.BonusCount > 0 || result.ScatterCount > 0 {
			specialBoards++
		}
	}

	t.Logf(
		"slot raw normal %d spins: payout_hit_rate=%.4f low_tier_rate=%.4f non_feature_loss_rate=%.4f special_board_rate=%.4f",
		spins,
		float64(payoutHits)/spins,
		float64(lowTierHits)/spins,
		float64(nonFeatureLosses)/spins,
		float64(specialBoards)/spins,
	)
}

func TestSlotLowTierWinPreservesFeatureAndHighPayouts(t *testing.T) {
	base := SlotSpinResult{BetCredits: 10, TotalPayout: 5}
	require.True(t, slotIsLowTierWin(&base))

	bonusRound := base
	bonusRound.BonusCount = SlotBonusTriggerMin
	require.False(t, slotIsLowTierWin(&bonusRound))

	freeSpin := base
	freeSpin.FreeSpinsAwarded = 5
	require.False(t, slotIsLowTierWin(&freeSpin))

	higherValue := base
	higherValue.TotalPayout = 40
	require.False(t, slotIsLowTierWin(&higherValue))
}

func slotSimulationLineRates(hits map[string]int, spins int) map[string]float64 {
	rates := make(map[string]float64, len(hits))
	for key, hits := range hits {
		rates[key] = float64(hits) / float64(spins)
	}
	return rates
}

func TestSlotEvaluateKnownGridPays(t *testing.T) {
	engine := NewSlotEngine(newFixedRandReader(0, 0, 0, 0, 0))
	result, err := engine.Spin(10)
	require.NoError(t, err)
	require.Equal(t, SlotPaytableVersionV10, result.PaytableVersion)
	require.Equal(t, SlotReelStripVersionV11, result.ReelStripVersion)
	require.Equal(t, int64(10), result.BetCredits)
	require.Len(t, result.Grid, SlotRowCount)
	require.Len(t, result.Grid[0], SlotReelCount)
	require.NotContains(t, strings.ToLower(result.PaytableVersion), "seed")
}

func TestSlotWaysPaysOneResultPerContinuousSymbol(t *testing.T) {
	grid := scatterGrid()
	grid[0][0], grid[1][0] = SlotSymbolDragon, SlotSymbolDragon
	grid[0][1], grid[1][1], grid[2][1] = SlotSymbolDragon, SlotSymbolDragon, SlotSymbolDragon
	grid[0][2] = SlotSymbolDragon

	wins, payout, err := evaluateSlotWays(grid, 10)
	require.NoError(t, err)
	require.Len(t, wins, 1)
	require.Equal(t, 1, wins[0].LineIndex)
	require.Equal(t, SlotSymbolDragon, wins[0].Symbol)
	require.Equal(t, 3, wins[0].Count)
	require.Equal(t, slotWaysPaysV10[SlotSymbolDragon][3], wins[0].Mult)
	require.Len(t, wins[0].Positions, 3)
	require.Equal(t, slotWaysPaysV10[SlotSymbolDragon][3]*10/SlotWaysBetDivisor, payout)
	require.Equal(t, payout, sumWinningWayPayouts(wins))
}

func TestSlotWaysBonusDoesNotSubstitute(t *testing.T) {
	grid := scatterGrid()
	grid[0][0] = SlotSymbolDragon
	grid[1][1] = SlotSymbolBonus
	grid[2][2] = SlotSymbolDragon

	wins, payout, err := evaluateSlotWays(grid, 5)
	require.NoError(t, err)
	require.Empty(t, wins)
	require.Zero(t, payout)
}

func TestSlotWaysPureBonusPrefixDoesNotPay(t *testing.T) {
	grid := scatterGrid()
	grid[0][0], grid[1][1], grid[2][2] = SlotSymbolBonus, SlotSymbolBonus, SlotSymbolBonus

	wins, payout, err := evaluateSlotWays(grid, 10)
	require.NoError(t, err)
	require.Empty(t, wins)
	require.Zero(t, payout)
}

func TestSlotWaysScreenshotRegressionUsesOnlyPaytableMultiplier(t *testing.T) {
	grid := [SlotRowCount][SlotReelCount]string{
		{SlotSymbolBonus, SlotSymbolJade, SlotSymbolIngot, SlotSymbolIngot, SlotSymbolIngot},
		{SlotSymbolBell, SlotSymbolBell, SlotSymbolIngot, SlotSymbolJackpot, SlotSymbolBell},
		{SlotSymbolIngot, SlotSymbolIngot, SlotSymbolJade, SlotSymbolBell, SlotSymbolJade},
	}
	wins, payout, err := evaluateSlotWays(grid, 10)
	require.NoError(t, err)
	require.Len(t, wins, 1)
	for _, win := range wins {
		require.Equal(t, SlotSymbolIngot, win.Symbol)
		require.Equal(t, 5, win.Count)
		require.Equal(t, slotWaysPaysV10[SlotSymbolIngot][5], win.Mult)
		for _, position := range win.Positions {
			require.NotEqual(t, SlotSymbolBonus, grid[position[1]][position[0]])
		}
	}
	require.Equal(t, slotWaysPaysV10[SlotSymbolIngot][5]*10/SlotWaysBetDivisor, payout)
	require.Equal(t, payout, sumWinningWayPayouts(wins))
}

func TestSlotWaysScreenshotRegressionDoesNotMultiplyMatchingCellsOnOneReel(t *testing.T) {
	grid := [SlotRowCount][SlotReelCount]string{
		{SlotSymbolDragon, SlotSymbolJade, SlotSymbolIngot, SlotSymbolDragon, SlotSymbolJade},
		{SlotSymbolBell, SlotSymbolDragon, SlotSymbolDragon, SlotSymbolDragon, SlotSymbolJackpot},
		{SlotSymbolBell, SlotSymbolJade, SlotSymbolIngot, SlotSymbolDragon, SlotSymbolJackpot},
	}
	wins, payout, err := evaluateSlotWays(grid, 10)
	require.NoError(t, err)
	require.Len(t, wins, 1)
	require.Equal(t, SlotSymbolDragon, wins[0].Symbol)
	require.Equal(t, 4, wins[0].Count)
	require.Equal(t, int64(80), wins[0].Payout)
	require.Equal(t, int64(80), payout)
}

func TestSlotWaysOnlyPaysLongestMatch(t *testing.T) {
	grid := scatterGrid()
	for reel := 0; reel < 4; reel++ {
		grid[0][reel] = SlotSymbolDragon
	}

	wins, payout, err := evaluateSlotWays(grid, 10)
	require.NoError(t, err)
	require.Len(t, wins, 1)
	require.Equal(t, 4, wins[0].Count)
	require.Equal(t, slotWaysPaysV10[SlotSymbolDragon][4]*10/SlotWaysBetDivisor, payout)
}

func TestSlotWaysScatterNeverSubstitutes(t *testing.T) {
	grid := scatterGrid()
	grid[0][0], grid[0][1] = SlotSymbolDragon, SlotSymbolDragon
	grid[0][2] = SlotSymbolScatter

	wins, payout, err := evaluateSlotWays(grid, 10)
	require.NoError(t, err)
	require.Empty(t, wins)
	require.Zero(t, payout)
}

func TestSlotWaysDetectsPayoutOverflow(t *testing.T) {
	grid := scatterGrid()
	grid[0][0], grid[0][1], grid[0][2] = SlotSymbolJackpot, SlotSymbolJackpot, SlotSymbolJackpot
	_, _, err := evaluateSlotWays(grid, math.MaxInt64)
	require.ErrorContains(t, err, "overflow")
}

func TestSlotV10VersionsAndBonusFrequencyAreDeterministic(t *testing.T) {
	require.Equal(t, "slot-paytable-v10", SlotPaytableVersionV10)
	require.Equal(t, "slot-reels-v11", SlotReelStripVersionV11)
	require.InDelta(t, 0.0075, SlotBonusTriggerProbability(), 1e-12)
	for reel := 0; reel < SlotReelCount; reel++ {
		require.NotEmpty(t, slotStopsByBonusVisibility(reel, 0))
		require.NotEmpty(t, slotStopsByBonusVisibility(reel, 1))
	}
}

func TestSlotFirstTwoReelsRequireDistinctVisibleSymbols(t *testing.T) {
	for reel := 0; reel < SlotProtectedVerticalReelCount; reel++ {
		hasDisallowedStop := false
		for stop := range slotReelStripsV3[reel] {
			if slotStopHasRepeatedVisibleSymbol(reel, stop) {
				hasDisallowedStop = true
				break
			}
		}
		require.True(t, hasDisallowedStop, "reel %d fixture must contain a filtered stack", reel)

		for _, stop := range slotAllowedStopsForReel(reel) {
			require.False(t, slotStopHasRepeatedVisibleSymbol(reel, stop))
		}
	}
}

func TestSlotEngineKeepsFirstTwoReelsDistinct(t *testing.T) {
	for _, tc := range []struct {
		name   string
		engine *SlotEngine
	}{
		{name: "normal spin", engine: NewSlotEngine(newFixedRandReader(109, 0, 1, 2, 3, 4, 5, 6, 7))},
		{name: "bonus trigger", engine: NewSlotEngine(newFixedRandReader(0, 0, 1, 2, 3, 4, 5, 6, 7))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.engine.Spin(10)
			require.NoError(t, err)
			for reel := 0; reel < SlotProtectedVerticalReelCount; reel++ {
				require.False(t, slotStopHasRepeatedVisibleSymbol(reel, result.Stops[reel]))
			}
		})
	}
}

func TestSlotTriggeredBonusCountsKeepFirstTwoReelsDistinct(t *testing.T) {
	for _, bonusCount := range []int{3, 4, 5} {
		t.Run(fmt.Sprintf("bonus_count_%d", bonusCount), func(t *testing.T) {
			engine := NewSlotEngine(newFixedRandReader(0, 1, 2, 3, 4, 5, 6, 7))
			stops, err := engine.drawStopsWithBonusCount(bonusCount)
			require.NoError(t, err)
			result, err := EvaluateSlotSpinV2(10, stops)
			require.NoError(t, err)
			require.Equal(t, bonusCount, result.BonusCount)
			for reel := 0; reel < SlotProtectedVerticalReelCount; reel++ {
				require.False(t, slotStopHasRepeatedVisibleSymbol(reel, result.Stops[reel]))
			}
		})
	}
}

func TestSlotPreferredTripleBoostIsDisabledForTheCurrentProfile(t *testing.T) {
	config := newSlotPreferredTripleBoostConfig()
	require.Greater(t, config.rates.bell, 0.0)
	require.Greater(t, config.rates.jade, 0.0)
	require.Greater(t, config.rates.either, config.rates.bell)
	require.Greater(t, config.rates.eligible, 0.0)
	require.Equal(t, 1.0, slotPreferredTripleEffectiveBoostFactor(config.rates))
	require.Zero(t, config.bellCutoff)
	require.Zero(t, config.jadeCutoff)

	original := &SlotSpinResult{}
	got, err := NewSlotEngine(newFixedRandReader(0)).maybeBoostPreferredTriple(10, original)
	require.NoError(t, err)
	require.Same(t, original, got)
}

func TestSlotBonusGuaranteedWinningSpinUsesAValidNonTriggeringOutcome(t *testing.T) {
	for _, bet := range []int64{1, 10, 50} {
		t.Run(fmt.Sprintf("bet_%d", bet), func(t *testing.T) {
			result, err := bonusFeatureGuaranteedWinningSpin(bet)
			require.NoError(t, err)
			require.Zero(t, result.BonusCount)
			require.Zero(t, result.ScatterCount)
			require.Greater(t, result.TotalPayout, int64(0))
			require.Len(t, result.WinningLines, 1)
			require.Equal(t, SlotSymbolIngot, result.WinningLines[0].Symbol)
			require.Equal(t, 3, result.WinningLines[0].Count)
			for reel := 0; reel < SlotProtectedVerticalReelCount; reel++ {
				require.False(t, slotStopHasRepeatedVisibleSymbol(reel, result.Stops[reel]))
			}
		})
	}
}

func TestSlotV10BellPaysDirectCreditsAtTenCreditBet(t *testing.T) {
	for count, want := range map[int]int64{3: 5, 4: 10, 5: 18} {
		grid := scatterGrid()
		for reel := 0; reel < count; reel++ {
			grid[0][reel] = SlotSymbolBell
		}
		wins, payout, err := evaluateSlotWays(grid, 10)
		require.NoError(t, err)
		require.Len(t, wins, 1)
		require.Equal(t, want, payout)
	}
}

func TestSlotV10PaytableConvertsReferenceMultipliersToDirectCredits(t *testing.T) {
	want := map[string]map[int]int64{
		SlotSymbolJackpot: {3: 80, 4: 200, 5: 400},
		SlotSymbolDragon:  {3: 40, 4: 80, 5: 160},
		SlotSymbolIngot:   {3: 10, 4: 30, 5: 54},
		SlotSymbolJade:    {3: 5, 4: 20, 5: 30},
		SlotSymbolBell:    {3: 5, 4: 10, 5: 18},
	}
	require.Equal(t, want, slotWaysPaysV10)
}

func TestSlotV10FourAndFiveReelPaytableFollowsConfiguredRatios(t *testing.T) {
	jade := slotWaysPaysV10[SlotSymbolJade]
	require.Equal(t, jade[3]*4, jade[4])
	require.Equal(t, jade[4]*3/2, jade[5])

	ingot := slotWaysPaysV10[SlotSymbolIngot]
	require.Equal(t, ingot[3]*3, ingot[4])
	require.Equal(t, ingot[4]*9/5, ingot[5])

	dragon := slotWaysPaysV10[SlotSymbolDragon]
	require.Equal(t, dragon[3]*2, dragon[4])
	require.Equal(t, dragon[4]*2, dragon[5])

	jackpot := slotWaysPaysV10[SlotSymbolJackpot]
	require.Equal(t, jackpot[3]*5/2, jackpot[4])
	require.Equal(t, jackpot[4]*2, jackpot[5])
}

func TestSlotV10ThirdReelFurtherLowersIngotAndDragonFrequency(t *testing.T) {
	counts := make(map[string]int)
	for _, symbol := range slotReelStripsV3[2] {
		counts[symbol]++
	}
	require.Len(t, slotReelStripsV3[2], 60)
	require.Equal(t, SlotThirdReelIngotStopsV9, counts[SlotSymbolIngot])
	require.Equal(t, SlotThirdReelDragonStopsV9, counts[SlotSymbolDragon])
	require.Less(t, counts[SlotSymbolIngot], SlotThirdReelIngotStopsV8)
	require.Less(t, counts[SlotSymbolDragon], 4)
}

func TestSlotV10RTPIsDeterministicAndEnumerable(t *testing.T) {
	rtp, combinations := EstimateSlotV4RTP()
	require.Equal(t, int64(60*60*60*60*60), combinations)
	require.False(t, math.IsNaN(rtp))
	require.False(t, math.IsInf(rtp, 0))
	require.Greater(t, rtp, 0.1)
	require.Less(t, rtp, 1.13)
	t.Logf("slot v10 base RTP by symbol: %#v", estimateSlotWaysRTPBySymbol())
	t.Logf("slot v10 base RTP excluding all BONUS effects and free-spin value: %.6f over %d stop combinations", rtp, combinations)
}

func TestSlotBonusPayoutMultiplierRequiresBonusOnTheFifthReel(t *testing.T) {
	for _, tc := range []struct {
		name              string
		bonusCount        int
		fifthReelHasBonus bool
		want              int64
	}{
		{name: "no bonus", bonusCount: 0, want: 160},
		{name: "two bonus without a base win", bonusCount: 2, fifthReelHasBonus: true, want: 0},
		{name: "one bonus outside fifth reel", bonusCount: 1, want: 160},
		{name: "two bonus outside fifth reel", bonusCount: 2, want: 160},
		{name: "one bonus on fifth reel", bonusCount: 1, fifthReelHasBonus: true, want: 240},
		{name: "two bonus with one on fifth reel", bonusCount: 2, fifthReelHasBonus: true, want: 320},
		{name: "three bonus reward round", bonusCount: 3, fifthReelHasBonus: true, want: 160},
	} {
		t.Run(tc.name, func(t *testing.T) {
			basePayout := int64(160)
			if tc.want == 0 {
				basePayout = 0
			}
			got, err := applySlotBonusPayoutMultiplier(basePayout, tc.bonusCount, tc.fifthReelHasBonus)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestSlotBonusPayoutMultiplierUsesTheWholeBaseGamePayout(t *testing.T) {
	grid := scatterGrid()
	for reel := 0; reel < 4; reel++ {
		grid[0][reel] = SlotSymbolDragon
	}

	grid[1][4] = SlotSymbolBonus
	oneBonus, err := evaluateSlotGrid(grid, [SlotReelCount]int{}, 10)
	require.NoError(t, err)
	require.Equal(t, 1, oneBonus.BonusCount)
	require.Equal(t, int64(80), oneBonus.LinePayout)
	require.Equal(t, int64(120), oneBonus.TotalPayout)

	grid[2][4] = SlotSymbolBonus
	twoBonus, err := evaluateSlotGrid(grid, [SlotReelCount]int{}, 10)
	require.NoError(t, err)
	require.Equal(t, 2, twoBonus.BonusCount)
	require.Equal(t, int64(80), twoBonus.LinePayout)
	require.Equal(t, int64(160), twoBonus.TotalPayout)

	grid[1][3] = SlotSymbolBonus
	threeBonus, err := evaluateSlotGrid(grid, [SlotReelCount]int{}, 10)
	require.NoError(t, err)
	require.Equal(t, 3, threeBonus.BonusCount)
	require.Equal(t, int64(80), threeBonus.LinePayout)
	require.Equal(t, int64(80), threeBonus.TotalPayout)
}

func TestSlotBonusMultiplierDoesNotApplyWithoutBonusOnFifthReel(t *testing.T) {
	grid := scatterGrid()
	for reel := 0; reel < 4; reel++ {
		grid[0][reel] = SlotSymbolDragon
	}
	grid[1][2] = SlotSymbolBonus
	grid[2][3] = SlotSymbolBonus

	result, err := evaluateSlotGrid(grid, [SlotReelCount]int{}, 10)
	require.NoError(t, err)
	require.Equal(t, 2, result.BonusCount)
	require.False(t, slotHasBonusOnFifthReel(grid))
	require.Equal(t, int64(80), result.LinePayout)
	require.Equal(t, int64(80), result.TotalPayout)
}

func scatterGrid() [SlotRowCount][SlotReelCount]string {
	var grid [SlotRowCount][SlotReelCount]string
	for row := 0; row < SlotRowCount; row++ {
		for reel := 0; reel < SlotReelCount; reel++ {
			grid[row][reel] = SlotSymbolScatter
		}
	}
	return grid
}

func sumWinningWayPayouts(wins []SlotWinningLine) int64 {
	var total int64
	for _, win := range wins {
		total += win.Payout
	}
	return total
}

func TestSlotScatterAwardsFreeSpinsWithoutBonus(t *testing.T) {
	var stops [SlotReelCount]int
	scatterStops := 0
	for reel := 0; reel < SlotReelCount && scatterStops < 3; reel++ {
		for i, symbol := range slotReelStripsV3[reel] {
			if symbol == SlotSymbolScatter {
				stops[reel] = (i - 1 + len(slotReelStripsV3[reel])) % len(slotReelStripsV3[reel])
				scatterStops++
				break
			}
		}
	}
	require.GreaterOrEqual(t, scatterStops, 3)
	result, err := EvaluateSlotSpinV2(10, stops)
	require.NoError(t, err)
	require.GreaterOrEqual(t, result.ScatterCount, 3)
	require.Zero(t, result.BonusCount)
	require.Greater(t, result.FreeSpinsAwarded, 0)
	require.Greater(t, result.ScatterPayout, int64(0))
}

func TestSlotBonusRequiresThreeVisibleSymbols(t *testing.T) {
	var twoStops [SlotReelCount]int
	for reel := range twoStops {
		visibility := 0
		if reel < 2 {
			visibility = 1
		}
		twoStops[reel] = slotStopsByBonusVisibility(reel, visibility)[0]
	}
	two, err := EvaluateSlotSpinV2(10, twoStops)
	require.NoError(t, err)
	require.Equal(t, 2, two.BonusCount)
	require.Less(t, two.BonusCount, SlotBonusTriggerMin)

	var threeStops [SlotReelCount]int
	for reel := range threeStops {
		visibility := 0
		if reel < 3 {
			visibility = 1
		}
		threeStops[reel] = slotStopsByBonusVisibility(reel, visibility)[0]
	}
	three, err := EvaluateSlotSpinV2(10, threeStops)
	require.NoError(t, err)
	require.Equal(t, SlotBonusTriggerMin, three.BonusCount)
}

func firstSlotStopShowingSymbol(t *testing.T, reel int, symbol string) uint32 {
	t.Helper()
	strip := slotReelStripsV3[reel]
	for stop := range strip {
		for row := 0; row < SlotRowCount; row++ {
			if strip[(stop+row)%len(strip)] == symbol {
				return uint32(stop)
			}
		}
	}
	t.Fatalf("reel %d has no visible %s stop", reel, symbol)
	return 0
}
