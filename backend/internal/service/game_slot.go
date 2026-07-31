package service

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
)

// Slot v3: fixed 5x3 / 243-ways entertainment cabinet.
// Outcomes and payouts are evaluated only on the server. Paytable and reel
// strips are versioned constants; runtime mutation is intentionally impossible.

const (
	SlotPaytableVersionV2       = "slot-paytable-v2"
	SlotReelStripVersionV2      = "slot-reels-v2"
	SlotPaytableVersionV3       = "slot-paytable-v3"
	SlotPaytableVersionV4       = "slot-paytable-v4"
	SlotPaytableVersionV5       = "slot-paytable-v5"
	SlotPaytableVersionV6       = "slot-paytable-v6"
	SlotPaytableVersionV7       = "slot-paytable-v7"
	SlotPaytableVersionV8       = "slot-paytable-v8"
	SlotPaytableVersionV9       = "slot-paytable-v9"
	SlotPaytableVersionV10      = "slot-paytable-v10"
	SlotReelStripVersionV3      = "slot-reels-v3"
	SlotReelStripVersionV4      = "slot-reels-v4"
	SlotReelStripVersionV5      = "slot-reels-v5"
	SlotReelStripVersionV6      = "slot-reels-v6"
	SlotReelStripVersionV7      = "slot-reels-v7"
	SlotReelStripVersionV8      = "slot-reels-v8"
	SlotReelStripVersionV9      = "slot-reels-v9"
	SlotReelStripVersionV10     = "slot-reels-v10"
	SlotReelStripVersionV11     = "slot-reels-v11"
	SlotBonusTriggerMin         = 3
	SlotBonusTriggerNumerator   = 60
	SlotBonusTriggerDenominator = 8000
	SlotPaytableVersionV1       = "slot-paytable-v1"
	SlotReelStripVersionV1      = "slot-reels-v1"
	SlotReelCount               = 5
	SlotRowCount                = 3
	SlotBetMaxCredits           = int64(10_000)
	SlotScatterFreeSpinsMin     = 3
	SlotWaysBetDivisor          = 10
	// The preferred three-of-a-kind boost is intentionally disabled in the
	// current profile; ordinary small-win frequency is controlled below.
	SlotPreferredTripleBoostFactor      = 1.00
	SlotPreferredTripleBoostDenominator = 1_000_000
	SlotPreferredTripleBoostAttempts    = 64
	// A low-tier-only three-of-a-kind is kept 50% of the time. Rejected
	// outcomes are redrawn as non-feature loss boards before they reach the client.
	SlotLowTierWinKeepNumerator   = 1
	SlotLowTierWinKeepDenominator = 2
	SlotLowTierWinRedrawAttempts  = 64
	SlotLowTierWinMaxBetMultiple  = int64(4)
	// The first two visible reels must show three distinct symbols. This is a
	// presentation rule applied before authoritative scoring.
	SlotProtectedVerticalReelCount = 2
	SlotThirdReelIngotStopsV7      = 34
	SlotThirdReelDragonStopsV7     = 2
	SlotThirdReelIngotStopsV8      = 24
	SlotThirdReelDragonStopsV8     = 2
	SlotThirdReelIngotStopsV9      = 8
	SlotThirdReelDragonStopsV9     = 2
)

const (
	SlotSymbolJackpot = "JP"
	SlotSymbolDragon  = "DR"
	SlotSymbolIngot   = "IG"
	SlotSymbolJade    = "JA"
	SlotSymbolBell    = "BE"
	SlotSymbolWild    = "WILD"
	SlotSymbolScatter = "SCATTER"
	SlotSymbolBonus   = "BONUS"
)

var slotScatterPaysV1 = map[int]int64{3: 1, 4: 3, 5: 10}

var slotScatterFreeSpinsV1 = map[int]int{3: 5, 4: 8, 5: 12}

// V10 values are direct credits for one symbol's continuous result at a
// 10-credit bet.
// Other supported bets scale proportionally and are rounded once after all
// winning ways have been accumulated.
var slotWaysPaysV10 = map[string]map[int]int64{
	SlotSymbolJackpot: {3: 80, 4: 200, 5: 400},
	SlotSymbolDragon:  {3: 40, 4: 80, 5: 160},
	SlotSymbolIngot:   {3: 10, 4: 30, 5: 54},
	SlotSymbolJade:    {3: 5, 4: 20, 5: 30},
	SlotSymbolBell:    {3: 5, 4: 10, 5: 18},
}

// This is a real v6-reel outcome with exactly one three-reel Ingot result.
// It is used only to replace an otherwise empty spin when a free-spin feature
// needs to satisfy its disclosed minimum hit count.
var slotBonusGuaranteedWinStops = [SlotReelCount]int{40, 40, 0, 0, 0}

// Each reel contains one isolated BONUS stop so a triggered feature can visibly
// land three, four, or five symbols. V11 further lowers high-return Ingot
// exposure on reel three without changing BONUS or SCATTER placement.
var slotReelStripsV3 = [SlotReelCount][]string{
	{
		SlotSymbolBell, SlotSymbolBell, SlotSymbolJade, SlotSymbolBell, SlotSymbolBell,
		SlotSymbolIngot, SlotSymbolBell, SlotSymbolBell, SlotSymbolDragon, SlotSymbolBell,
		SlotSymbolBell, SlotSymbolJackpot, SlotSymbolBell, SlotSymbolBell, SlotSymbolBell,
		SlotSymbolJade, SlotSymbolBonus, SlotSymbolIngot, SlotSymbolDragon, SlotSymbolBell,
		SlotSymbolJade, SlotSymbolBell, SlotSymbolIngot, SlotSymbolBell, SlotSymbolBell,
		SlotSymbolDragon, SlotSymbolBell, SlotSymbolBell, SlotSymbolBell, SlotSymbolBell,
		SlotSymbolJackpot, SlotSymbolBell, SlotSymbolBell, SlotSymbolScatter, SlotSymbolBell,
		SlotSymbolBell, SlotSymbolBell, SlotSymbolJade, SlotSymbolBell, SlotSymbolBell,
		SlotSymbolIngot, SlotSymbolBell, SlotSymbolDragon, SlotSymbolBell, SlotSymbolBell,
		SlotSymbolBell, SlotSymbolBell, SlotSymbolJackpot, SlotSymbolBell, SlotSymbolBell,
		SlotSymbolBell, SlotSymbolJade, SlotSymbolBell, SlotSymbolBell, SlotSymbolIngot,
		SlotSymbolBell, SlotSymbolBell, SlotSymbolDragon, SlotSymbolBell, SlotSymbolBell,
	},
	{
		SlotSymbolJade, SlotSymbolJade, SlotSymbolBell, SlotSymbolJade, SlotSymbolJade,
		SlotSymbolIngot, SlotSymbolJade, SlotSymbolJade, SlotSymbolDragon, SlotSymbolJade,
		SlotSymbolJade, SlotSymbolJackpot, SlotSymbolJade, SlotSymbolJade, SlotSymbolJade,
		SlotSymbolBell, SlotSymbolBonus, SlotSymbolIngot, SlotSymbolDragon, SlotSymbolJade,
		SlotSymbolBell, SlotSymbolJade, SlotSymbolIngot, SlotSymbolJade, SlotSymbolJade,
		SlotSymbolDragon, SlotSymbolJade, SlotSymbolJade, SlotSymbolJade, SlotSymbolJade,
		SlotSymbolJackpot, SlotSymbolJade, SlotSymbolJade, SlotSymbolScatter, SlotSymbolJade,
		SlotSymbolJade, SlotSymbolJade, SlotSymbolBell, SlotSymbolJade, SlotSymbolJade,
		SlotSymbolIngot, SlotSymbolJade, SlotSymbolDragon, SlotSymbolJade, SlotSymbolJade,
		SlotSymbolJade, SlotSymbolJade, SlotSymbolJackpot, SlotSymbolJade, SlotSymbolJade,
		SlotSymbolJade, SlotSymbolBell, SlotSymbolJade, SlotSymbolJade, SlotSymbolIngot,
		SlotSymbolJade, SlotSymbolJade, SlotSymbolDragon, SlotSymbolJade, SlotSymbolJade,
	},
	{
		SlotSymbolIngot, SlotSymbolIngot, SlotSymbolBell, SlotSymbolBell, SlotSymbolBell,
		SlotSymbolJade, SlotSymbolIngot, SlotSymbolBell, SlotSymbolBell, SlotSymbolBell,
		SlotSymbolIngot, SlotSymbolJackpot, SlotSymbolBell, SlotSymbolBell, SlotSymbolJade,
		SlotSymbolIngot, SlotSymbolBonus, SlotSymbolIngot, SlotSymbolBell, SlotSymbolBell,
		SlotSymbolBell, SlotSymbolIngot, SlotSymbolJade, SlotSymbolBell, SlotSymbolBell,
		SlotSymbolDragon, SlotSymbolIngot, SlotSymbolBell, SlotSymbolBell, SlotSymbolJade,
		SlotSymbolJackpot, SlotSymbolBell, SlotSymbolJade, SlotSymbolScatter, SlotSymbolBell,
		SlotSymbolBell, SlotSymbolBell, SlotSymbolBell, SlotSymbolJade, SlotSymbolBell,
		SlotSymbolJade, SlotSymbolBell, SlotSymbolBell, SlotSymbolBell, SlotSymbolJade,
		SlotSymbolBell, SlotSymbolBell, SlotSymbolJackpot, SlotSymbolBell, SlotSymbolBell,
		SlotSymbolJade, SlotSymbolBell, SlotSymbolBell, SlotSymbolBell, SlotSymbolJade,
		SlotSymbolJade, SlotSymbolBell, SlotSymbolDragon, SlotSymbolJade, SlotSymbolBell,
	},
	{
		SlotSymbolDragon, SlotSymbolDragon, SlotSymbolBell, SlotSymbolDragon, SlotSymbolDragon,
		SlotSymbolJade, SlotSymbolDragon, SlotSymbolDragon, SlotSymbolIngot, SlotSymbolDragon,
		SlotSymbolDragon, SlotSymbolJackpot, SlotSymbolDragon, SlotSymbolDragon, SlotSymbolDragon,
		SlotSymbolDragon, SlotSymbolBonus, SlotSymbolDragon, SlotSymbolDragon, SlotSymbolDragon,
		SlotSymbolBell, SlotSymbolDragon, SlotSymbolJade, SlotSymbolDragon, SlotSymbolDragon,
		SlotSymbolIngot, SlotSymbolDragon, SlotSymbolDragon, SlotSymbolDragon, SlotSymbolDragon,
		SlotSymbolJackpot, SlotSymbolDragon, SlotSymbolDragon, SlotSymbolScatter, SlotSymbolDragon,
		SlotSymbolDragon, SlotSymbolDragon, SlotSymbolBell, SlotSymbolDragon, SlotSymbolDragon,
		SlotSymbolJade, SlotSymbolDragon, SlotSymbolIngot, SlotSymbolDragon, SlotSymbolDragon,
		SlotSymbolDragon, SlotSymbolDragon, SlotSymbolJackpot, SlotSymbolDragon, SlotSymbolDragon,
		SlotSymbolDragon, SlotSymbolBell, SlotSymbolDragon, SlotSymbolDragon, SlotSymbolJade,
		SlotSymbolDragon, SlotSymbolDragon, SlotSymbolIngot, SlotSymbolDragon, SlotSymbolDragon,
	},
	{
		SlotSymbolJackpot, SlotSymbolJackpot, SlotSymbolBell, SlotSymbolJackpot, SlotSymbolJackpot,
		SlotSymbolJade, SlotSymbolJackpot, SlotSymbolIngot, SlotSymbolJackpot, SlotSymbolJackpot,
		SlotSymbolDragon, SlotSymbolJackpot, SlotSymbolJackpot, SlotSymbolWild, SlotSymbolJackpot,
		SlotSymbolJackpot, SlotSymbolJackpot, SlotSymbolBonus, SlotSymbolJackpot, SlotSymbolJackpot,
		SlotSymbolBell, SlotSymbolJackpot, SlotSymbolJade, SlotSymbolJackpot, SlotSymbolJackpot,
		SlotSymbolIngot, SlotSymbolJackpot, SlotSymbolJackpot, SlotSymbolScatter, SlotSymbolJackpot,
		SlotSymbolJackpot, SlotSymbolDragon, SlotSymbolJackpot, SlotSymbolWild, SlotSymbolJackpot,
		SlotSymbolJackpot, SlotSymbolJackpot, SlotSymbolBell, SlotSymbolJackpot, SlotSymbolJackpot,
		SlotSymbolJade, SlotSymbolJackpot, SlotSymbolIngot, SlotSymbolJackpot, SlotSymbolJackpot,
		SlotSymbolJackpot, SlotSymbolDragon, SlotSymbolJackpot, SlotSymbolJackpot, SlotSymbolWild,
		SlotSymbolJackpot, SlotSymbolJackpot, SlotSymbolBell, SlotSymbolJackpot, SlotSymbolJade,
		SlotSymbolJackpot, SlotSymbolJackpot, SlotSymbolIngot, SlotSymbolJackpot, SlotSymbolJackpot,
	},
}

type slotPreferredTripleRates struct {
	bell     float64
	jade     float64
	either   float64
	eligible float64
}

type slotPreferredTripleBoostConfig struct {
	bellCutoff int
	jadeCutoff int
	rates      slotPreferredTripleRates
}

type slotPreferredTripleWindowState struct {
	bellMatch  bool
	bellActual bool
	jadeMatch  bool
	jadeActual bool
}

var slotPreferredTripleBoostV1 = newSlotPreferredTripleBoostConfig()

type SlotWinningLine struct {
	LineIndex int      `json:"line_index"`
	Symbol    string   `json:"symbol"`
	Count     int      `json:"count"`
	Mult      int64    `json:"multiplier"`
	Payout    int64    `json:"payout"`
	Positions [][2]int `json:"positions"` // [reel, row]
}

// SlotSpinResult is the authoritative spin evaluation (no RNG seed).
type SlotSpinResult struct {
	Grid             [SlotRowCount][SlotReelCount]string `json:"grid"`
	Stops            [SlotReelCount]int                  `json:"stops"`
	WinningLines     []SlotWinningLine                   `json:"winning_lines"`
	ScatterCount     int                                 `json:"scatter_count"`
	BonusCount       int                                 `json:"bonus_count"`
	ScatterPayout    int64                               `json:"scatter_payout"`
	LinePayout       int64                               `json:"line_payout"`
	TotalPayout      int64                               `json:"total_payout"`
	FreeSpinsAwarded int                                 `json:"free_spins_awarded"`
	PaytableVersion  string                              `json:"paytable_version"`
	ReelStripVersion string                              `json:"reel_strip_version"`
	BetCredits       int64                               `json:"bet_credits"`
}

// SlotEngine evaluates fixed v3 reels/paytable. The RNG reader is injectable.
type SlotEngine struct {
	rand io.Reader
}

type slotOddsProfilePolicy struct {
	bonusTriggerNumerator  int
	lowTierKeepNumerator   int
	lowTierKeepDenominator int
}

func NewSlotEngine(randReader io.Reader) *SlotEngine {
	if randReader == nil {
		randReader = defaultCryptoReader()
	}
	return &SlotEngine{rand: randReader}
}

func (e *SlotEngine) PaytableVersion() string  { return SlotPaytableVersionV10 }
func (e *SlotEngine) ReelStripVersion() string { return SlotReelStripVersionV11 }

// Spin draws five reel stops with crypto rejection sampling and evaluates payouts.
func (e *SlotEngine) Spin(betCredits int64) (*SlotSpinResult, error) {
	return e.SpinWithOddsProfile(betCredits, GameOddsProfileStandard)
}

// SpinWithOddsProfile applies one period-wide profile shared by every player of
// this game. It does not inspect user identity, balance, or personal history.
func (e *SlotEngine) SpinWithOddsProfile(betCredits int64, profile GameOddsProfile) (*SlotSpinResult, error) {
	if betCredits <= 0 {
		return nil, fmt.Errorf("bet must be positive")
	}
	policy := slotPolicyForOddsProfile(profile)
	trigger, err := cryptoUniformInt(e.rand, SlotBonusTriggerDenominator)
	if err != nil {
		return nil, fmt.Errorf("draw bonus trigger: %w", err)
	}
	var stops [SlotReelCount]int
	triggeredBonus := trigger < policy.bonusTriggerNumerator
	if triggeredBonus {
		bonusCount, drawErr := e.drawTriggeredBonusCount()
		if drawErr != nil {
			return nil, drawErr
		}
		stops, err = e.drawStopsWithBonusCount(bonusCount)
	} else {
		stops, err = e.drawStopsWithoutBonusTrigger()
	}
	if err != nil {
		return nil, err
	}
	result, err := EvaluateSlotSpinV2(betCredits, stops)
	if err != nil || triggeredBonus {
		return result, err
	}
	result, err = e.rebalanceLowTierWinWithPolicy(betCredits, result, policy)
	if err != nil {
		return nil, err
	}
	return e.maybeBoostPreferredTriple(betCredits, result)
}

func slotPolicyForOddsProfile(profile GameOddsProfile) slotOddsProfilePolicy {
	switch NormalizeGameOddsProfile(profile) {
	case GameOddsProfileCooling:
		return slotOddsProfilePolicy{
			bonusTriggerNumerator:  45,
			lowTierKeepNumerator:   1,
			lowTierKeepDenominator: 3,
		}
	case GameOddsProfileTight:
		return slotOddsProfilePolicy{
			bonusTriggerNumerator:  30,
			lowTierKeepNumerator:   1,
			lowTierKeepDenominator: 4,
		}
	default:
		return slotOddsProfilePolicy{
			bonusTriggerNumerator:  SlotBonusTriggerNumerator,
			lowTierKeepNumerator:   SlotLowTierWinKeepNumerator,
			lowTierKeepDenominator: SlotLowTierWinKeepDenominator,
		}
	}
}

// SpinWithoutBonusTrigger is used inside an already active BONUS feature.
// It prevents nested reward rounds while preserving ordinary ways/scatter play.
func (e *SlotEngine) SpinWithoutBonusTrigger(betCredits int64) (*SlotSpinResult, error) {
	if betCredits <= 0 {
		return nil, fmt.Errorf("bet must be positive")
	}
	stops, err := e.drawStopsWithoutBonusTrigger()
	if err != nil {
		return nil, err
	}
	return EvaluateSlotSpinV2(betCredits, stops)
}

// bonusFeatureGuaranteedWinningSpin returns a valid, non-triggering reel
// outcome for the paid BONUS feature. It never changes the base-game RNG.
func bonusFeatureGuaranteedWinningSpin(betCredits int64) (*SlotSpinResult, error) {
	return EvaluateSlotSpinV2(betCredits, slotBonusGuaranteedWinStops)
}

func (e *SlotEngine) drawTriggeredBonusCount() (int, error) {
	draw, err := cryptoUniformInt(e.rand, 10000)
	if err != nil {
		return 0, fmt.Errorf("draw bonus symbol count: %w", err)
	}
	switch {
	case draw < 9380:
		return 3, nil
	case draw < 9980:
		return 4, nil
	default:
		return 5, nil
	}
}

func (e *SlotEngine) drawStopsWithBonusCount(count int) ([SlotReelCount]int, error) {
	var stops [SlotReelCount]int
	if count < SlotBonusTriggerMin || count > SlotReelCount {
		return stops, fmt.Errorf("invalid triggered bonus count %d", count)
	}
	reels := []int{0, 1, 2, 3, 4}
	for i := len(reels) - 1; i > 0; i-- {
		j, err := cryptoUniformInt(e.rand, i+1)
		if err != nil {
			return stops, fmt.Errorf("shuffle bonus reels: %w", err)
		}
		reels[i], reels[j] = reels[j], reels[i]
	}
	selected := make(map[int]bool, count)
	for _, reel := range reels[:count] {
		selected[reel] = true
	}
	for reel := 0; reel < SlotReelCount; reel++ {
		visible := 0
		if selected[reel] {
			visible = 1
		}
		candidates := slotFilterStopsForRepeatedVisibleSymbol(reel, slotStopsByBonusVisibility(reel, visible))
		if len(candidates) == 0 {
			return stops, fmt.Errorf("reel %d has no permitted stop with %d visible bonus", reel, visible)
		}
		index, err := cryptoUniformInt(e.rand, len(candidates))
		if err != nil {
			return stops, fmt.Errorf("draw triggered reel %d stop: %w", reel, err)
		}
		stops[reel] = candidates[index]
	}
	return stops, nil
}

func (e *SlotEngine) drawStopsWithoutBonusTrigger() ([SlotReelCount]int, error) {
	for attempt := 0; attempt < 256; attempt++ {
		var stops [SlotReelCount]int
		bonusCount := 0
		for reel := 0; reel < SlotReelCount; reel++ {
			candidates := slotAllowedStopsForReel(reel)
			if len(candidates) == 0 {
				return stops, fmt.Errorf("reel %d has no permitted stop", reel)
			}
			stopIndex, err := cryptoUniformInt(e.rand, len(candidates))
			if err != nil {
				return stops, fmt.Errorf("draw reel %d stop: %w", reel, err)
			}
			stops[reel] = candidates[stopIndex]
			bonusCount += slotVisibleBonusCount(reel, stops[reel])
		}
		if bonusCount < SlotBonusTriggerMin {
			return stops, nil
		}
	}
	return [SlotReelCount]int{}, fmt.Errorf("failed to draw non-triggering slot outcome")
}

// rebalanceLowTierWin reduces ordinary low-value hit frequency by replacing
// rejected normal boards paying under four times the current bet with a real
// non-feature loss board. BONUS reward rounds, SCATTER free spins, and
// higher-value wins bypass it.
func (e *SlotEngine) rebalanceLowTierWin(betCredits int64, result *SlotSpinResult) (*SlotSpinResult, error) {
	return e.rebalanceLowTierWinWithPolicy(betCredits, result, slotPolicyForOddsProfile(GameOddsProfileStandard))
}

func (e *SlotEngine) rebalanceLowTierWinWithPolicy(
	betCredits int64,
	result *SlotSpinResult,
	policy slotOddsProfilePolicy,
) (*SlotSpinResult, error) {
	if !slotIsLowTierWin(result) {
		return result, nil
	}

	keepDraw, err := cryptoUniformInt(e.rand, policy.lowTierKeepDenominator)
	if err != nil {
		return nil, fmt.Errorf("draw low-tier win keep decision: %w", err)
	}
	if keepDraw < policy.lowTierKeepNumerator {
		return result, nil
	}

	for attempt := 0; attempt < SlotLowTierWinRedrawAttempts; attempt++ {
		stops, drawErr := e.drawStopsWithoutBonusTrigger()
		if drawErr != nil {
			return nil, drawErr
		}
		candidate, evaluateErr := EvaluateSlotSpinV2(betCredits, stops)
		if evaluateErr != nil {
			return nil, evaluateErr
		}
		if slotIsNonFeatureLoss(candidate) {
			return candidate, nil
		}
	}

	// Keeping the original outcome is safer than failing an otherwise valid spin
	// if an unusually long sequence contains no eligible empty board.
	return result, nil
}

func slotIsLowTierWin(result *SlotSpinResult) bool {
	if result == nil || result.TotalPayout <= 0 || result.BonusCount >= SlotBonusTriggerMin ||
		result.FreeSpinsAwarded != 0 || result.BetCredits <= 0 {
		return false
	}
	return result.TotalPayout < result.BetCredits*SlotLowTierWinMaxBetMultiple
}

func slotIsNonFeatureLoss(result *SlotSpinResult) bool {
	return result != nil && result.TotalPayout == 0 && result.BonusCount < SlotBonusTriggerMin &&
		result.FreeSpinsAwarded == 0 && len(result.WinningLines) == 0
}

// maybeBoostPreferredTriple is retained for compatibility with earlier reel
// profiles. The current profile disables it before consuming any RNG state.
func (e *SlotEngine) maybeBoostPreferredTriple(betCredits int64, result *SlotSpinResult) (*SlotSpinResult, error) {
	if SlotPreferredTripleBoostFactor <= 1 {
		return result, nil
	}
	if result == nil || result.BonusCount != 0 || result.ScatterCount != 0 ||
		slotHasPreferredTriple(result, SlotSymbolBell) || slotHasPreferredTriple(result, SlotSymbolJade) {
		return result, nil
	}

	draw, err := cryptoUniformInt(e.rand, SlotPreferredTripleBoostDenominator)
	if err != nil {
		return nil, fmt.Errorf("draw preferred triple boost: %w", err)
	}
	target := ""
	switch {
	case draw < slotPreferredTripleBoostV1.bellCutoff:
		target = SlotSymbolBell
	case draw < slotPreferredTripleBoostV1.jadeCutoff:
		target = SlotSymbolJade
	default:
		return result, nil
	}

	for attempt := 0; attempt < SlotPreferredTripleBoostAttempts; attempt++ {
		stops, drawErr := e.drawPreferredTripleStops(target, attempt)
		if drawErr != nil {
			return nil, drawErr
		}
		boosted, evaluateErr := EvaluateSlotSpinV2(betCredits, stops)
		if evaluateErr != nil {
			return nil, evaluateErr
		}
		if slotIsIsolatedPreferredTriple(boosted, target) {
			return boosted, nil
		}
	}

	return nil, fmt.Errorf("draw preferred %s three-of-a-kind: no isolated outcome", target)
}

func (e *SlotEngine) drawPreferredTripleStops(target string, attempt int) ([SlotReelCount]int, error) {
	var stops [SlotReelCount]int
	actualReel, err := cryptoUniformInt(e.rand, 3)
	if err != nil {
		return stops, fmt.Errorf("draw preferred %s actual reel: %w", target, err)
	}

	for reel := 0; reel < 3; reel++ {
		candidates := slotStopsMatchingPreferredTarget(reel, target, reel == actualReel)
		stop, drawErr := e.drawSlotCandidate(candidates, attempt+reel, target)
		if drawErr != nil {
			return stops, drawErr
		}
		stops[reel] = stop
	}

	// Reel four must break the left-to-right match, guaranteeing exactly three.
	breakCandidates := slotStopsBreakingPreferredTarget(3, target)
	stop, drawErr := e.drawSlotCandidate(breakCandidates, attempt+3, target)
	if drawErr != nil {
		return stops, drawErr
	}
	stops[3] = stop

	// The fifth reel is unrelated to a three-reel result. Keep it free of
	// SCATTER/BONUS so a boost never invents a separate feature outcome.
	plainCandidates := slotPlainStops(4)
	stop, drawErr = e.drawSlotCandidate(plainCandidates, attempt+4, target)
	if drawErr != nil {
		return stops, drawErr
	}
	stops[4] = stop
	return stops, nil
}

func (e *SlotEngine) drawSlotCandidate(candidates []int, attempt int, target string) (int, error) {
	if len(candidates) == 0 {
		return 0, fmt.Errorf("no candidate stop for preferred %s triple", target)
	}
	index, err := cryptoUniformInt(e.rand, len(candidates))
	if err != nil {
		return 0, fmt.Errorf("draw preferred %s triple stop: %w", target, err)
	}
	// Advancing by attempt guarantees deterministic test readers can still walk
	// through valid candidates if the first combination has another winning line.
	return candidates[(index+attempt)%len(candidates)], nil
}

func slotStopsMatchingPreferredTarget(reel int, target string, requireActual bool) []int {
	strip := slotReelStripsV3[reel]
	stops := make([]int, 0, len(strip))
	for stop := range strip {
		matches, actual := slotWindowMatchesTarget(reel, stop, target)
		if matches && (!requireActual || actual) && slotStopIsPlain(reel, stop) && !slotStopHasRepeatedVisibleSymbol(reel, stop) {
			stops = append(stops, stop)
		}
	}
	return stops
}

func slotStopsBreakingPreferredTarget(reel int, target string) []int {
	strip := slotReelStripsV3[reel]
	stops := make([]int, 0, len(strip))
	for stop := range strip {
		matches, _ := slotWindowMatchesTarget(reel, stop, target)
		if !matches && slotStopIsPlain(reel, stop) && !slotStopHasRepeatedVisibleSymbol(reel, stop) {
			stops = append(stops, stop)
		}
	}
	return stops
}

func slotPlainStops(reel int) []int {
	strip := slotReelStripsV3[reel]
	stops := make([]int, 0, len(strip))
	for stop := range strip {
		if slotStopIsPlain(reel, stop) {
			stops = append(stops, stop)
		}
	}
	return stops
}

func slotAllowedStopsForReel(reel int) []int {
	if reel < 0 || reel >= SlotReelCount {
		return nil
	}
	strip := slotReelStripsV3[reel]
	stops := make([]int, 0, len(strip))
	for stop := range strip {
		if !slotStopHasRepeatedVisibleSymbol(reel, stop) {
			stops = append(stops, stop)
		}
	}
	return stops
}

func slotFilterStopsForRepeatedVisibleSymbol(reel int, candidates []int) []int {
	if reel < 0 || reel >= SlotReelCount {
		return nil
	}
	if reel >= SlotProtectedVerticalReelCount {
		return candidates
	}
	stops := make([]int, 0, len(candidates))
	for _, stop := range candidates {
		if !slotStopHasRepeatedVisibleSymbol(reel, stop) {
			stops = append(stops, stop)
		}
	}
	return stops
}

func slotStopHasRepeatedVisibleSymbol(reel, stop int) bool {
	if reel < 0 || reel >= SlotProtectedVerticalReelCount {
		return false
	}
	strip := slotReelStripsV3[reel]
	if stop < 0 || stop >= len(strip) {
		return false
	}
	first := strip[stop]
	second := strip[(stop+1)%len(strip)]
	third := strip[(stop+2)%len(strip)]
	return first == second || first == third || second == third
}

func slotStopIsPlain(reel, stop int) bool {
	return slotVisibleBonusCount(reel, stop) == 0 && slotVisibleSymbolCount(reel, stop, SlotSymbolScatter) == 0
}

func slotVisibleSymbolCount(reel, stop int, target string) int {
	strip := slotReelStripsV3[reel]
	count := 0
	for row := 0; row < SlotRowCount; row++ {
		if strip[(stop+row)%len(strip)] == target {
			count++
		}
	}
	return count
}

func slotWindowMatchesTarget(reel, stop int, target string) (matches, actual bool) {
	strip := slotReelStripsV3[reel]
	hasWild := false
	for row := 0; row < SlotRowCount; row++ {
		symbol := strip[(stop+row)%len(strip)]
		actual = actual || symbol == target
		hasWild = hasWild || symbol == SlotSymbolWild
	}
	return actual || hasWild, actual
}

func slotHasPreferredTriple(result *SlotSpinResult, target string) bool {
	if result == nil {
		return false
	}
	for _, line := range result.WinningLines {
		if line.Symbol == target && line.Count == 3 {
			return true
		}
	}
	return false
}

func slotIsIsolatedPreferredTriple(result *SlotSpinResult, target string) bool {
	if result == nil || result.BonusCount != 0 || result.ScatterCount != 0 || result.FreeSpinsAwarded != 0 {
		return false
	}
	if len(result.WinningLines) != 1 {
		return false
	}
	line := result.WinningLines[0]
	return line.Symbol == target && line.Count == 3
}

func newSlotPreferredTripleBoostConfig() slotPreferredTripleBoostConfig {
	rates := slotPreferredTripleBaseRates()
	if rates.eligible <= 0 {
		return slotPreferredTripleBoostConfig{rates: rates}
	}

	effectiveFactor := slotPreferredTripleEffectiveBoostFactor(rates)
	additionalBell := rates.bell * (effectiveFactor - 1) / rates.eligible
	additionalJade := rates.jade * (effectiveFactor - 1) / rates.eligible
	bellCutoff := int(math.Round(additionalBell * SlotPreferredTripleBoostDenominator))
	jadeCutoff := int(math.Round(additionalJade * SlotPreferredTripleBoostDenominator))
	if bellCutoff < 0 {
		bellCutoff = 0
	}
	if jadeCutoff < 0 {
		jadeCutoff = 0
	}
	if bellCutoff > SlotPreferredTripleBoostDenominator {
		bellCutoff = SlotPreferredTripleBoostDenominator
	}
	if jadeCutoff > SlotPreferredTripleBoostDenominator-bellCutoff {
		jadeCutoff = SlotPreferredTripleBoostDenominator - bellCutoff
	}
	return slotPreferredTripleBoostConfig{
		bellCutoff: bellCutoff,
		jadeCutoff: bellCutoff + jadeCutoff,
		rates:      rates,
	}
}

// slotPreferredTripleEffectiveBoostFactor keeps the Bell and Jade uplift
// balanced when a reel layout leaves too few normal outcomes to fund the full
// target boost. It never replaces BONUS or SCATTER outcomes.
func slotPreferredTripleEffectiveBoostFactor(rates slotPreferredTripleRates) float64 {
	if rates.eligible <= 0 || rates.bell <= 0 || rates.jade <= 0 {
		return 1
	}

	maxAdditionalFactor := rates.eligible / (rates.bell + rates.jade)
	return 1 + math.Min(SlotPreferredTripleBoostFactor-1, maxAdditionalFactor)
}

// slotPreferredTripleBaseRates groups each reel's visible three-symbol window
// by the facts needed to score Bell/Jade. This avoids enumerating 60^5 grids
// while still producing exact base-game three-reel probabilities.
func slotPreferredTripleBaseRates() slotPreferredTripleRates {
	var allGroups [4]map[slotPreferredTripleWindowState]int64
	var plainGroups [4]map[slotPreferredTripleWindowState]int64
	for reel := 0; reel < len(allGroups); reel++ {
		allGroups[reel] = make(map[slotPreferredTripleWindowState]int64)
		plainGroups[reel] = make(map[slotPreferredTripleWindowState]int64)
		for _, stop := range slotAllowedStopsForReel(reel) {
			state := slotPreferredTripleStateForStop(reel, stop)
			allGroups[reel][state]++
			if slotStopIsPlain(reel, stop) {
				plainGroups[reel][state]++
			}
		}
	}

	totalCombinations := int64(1)
	for reel := 0; reel < SlotReelCount; reel++ {
		totalCombinations *= int64(len(slotAllowedStopsForReel(reel)))
	}
	_, bell, jade, either := slotPreferredTripleRateCounts(allGroups, int64(len(slotAllowedStopsForReel(4))))
	plainTotal, _, _, plainEither := slotPreferredTripleRateCounts(plainGroups, int64(len(slotPlainStops(4))))
	if totalCombinations == 0 {
		return slotPreferredTripleRates{}
	}
	return slotPreferredTripleRates{
		bell:     float64(bell) / float64(totalCombinations),
		jade:     float64(jade) / float64(totalCombinations),
		either:   float64(either) / float64(totalCombinations),
		eligible: float64(plainTotal-plainEither) / float64(totalCombinations),
	}
}

func slotPreferredTripleRateCounts(
	groups [4]map[slotPreferredTripleWindowState]int64,
	fifthReelWeight int64,
) (total, bell, jade, either int64) {
	if fifthReelWeight <= 0 {
		return 0, 0, 0, 0
	}
	var states [4]slotPreferredTripleWindowState
	var visit func(reel int, weight int64)
	visit = func(reel int, weight int64) {
		if reel == len(groups) {
			weight *= fifthReelWeight
			total += weight
			hasBell := slotPreferredTripleMatches(states, SlotSymbolBell)
			hasJade := slotPreferredTripleMatches(states, SlotSymbolJade)
			if hasBell {
				bell += weight
			}
			if hasJade {
				jade += weight
			}
			if hasBell || hasJade {
				either += weight
			}
			return
		}
		for state, count := range groups[reel] {
			states[reel] = state
			visit(reel+1, weight*count)
		}
	}
	visit(0, 1)
	return total, bell, jade, either
}

func slotPreferredTripleStateForStop(reel, stop int) slotPreferredTripleWindowState {
	bellMatch, bellActual := slotWindowMatchesTarget(reel, stop, SlotSymbolBell)
	jadeMatch, jadeActual := slotWindowMatchesTarget(reel, stop, SlotSymbolJade)
	return slotPreferredTripleWindowState{
		bellMatch:  bellMatch,
		bellActual: bellActual,
		jadeMatch:  jadeMatch,
		jadeActual: jadeActual,
	}
}

func slotPreferredTripleMatches(states [4]slotPreferredTripleWindowState, target string) bool {
	for reel := 0; reel < 3; reel++ {
		matches, _ := slotPreferredTripleStateTarget(states[reel], target)
		if !matches {
			return false
		}
	}
	breaks, _ := slotPreferredTripleStateTarget(states[3], target)
	if breaks {
		return false
	}
	for reel := 0; reel < 3; reel++ {
		_, actual := slotPreferredTripleStateTarget(states[reel], target)
		if actual {
			return true
		}
	}
	return false
}

func slotPreferredTripleStateTarget(state slotPreferredTripleWindowState, target string) (matches, actual bool) {
	switch target {
	case SlotSymbolBell:
		return state.bellMatch, state.bellActual
	case SlotSymbolJade:
		return state.jadeMatch, state.jadeActual
	default:
		return false, false
	}
}

func slotStopsByBonusVisibility(reel, visible int) []int {
	strip := slotReelStripsV3[reel]
	stops := make([]int, 0, len(strip))
	for stop := range strip {
		if slotVisibleBonusCount(reel, stop) == visible {
			stops = append(stops, stop)
		}
	}
	return stops
}

func slotVisibleBonusCount(reel, stop int) int {
	strip := slotReelStripsV3[reel]
	count := 0
	for row := 0; row < SlotRowCount; row++ {
		if strip[(stop+row)%len(strip)] == SlotSymbolBonus {
			count++
		}
	}
	return count
}

func SlotBonusTriggerProbability() float64 {
	return float64(SlotBonusTriggerNumerator) / float64(SlotBonusTriggerDenominator)
}

// EvaluateSlotSpinV2 is the compatibility entry point for v3 ways/scatter scoring.
func EvaluateSlotSpinV2(betCredits int64, stops [SlotReelCount]int) (*SlotSpinResult, error) {
	if betCredits <= 0 {
		return nil, fmt.Errorf("bet must be positive")
	}
	var grid [SlotRowCount][SlotReelCount]string
	for reel := 0; reel < SlotReelCount; reel++ {
		strip := slotReelStripsV3[reel]
		if len(strip) == 0 {
			return nil, fmt.Errorf("empty reel strip %d", reel)
		}
		stop := stops[reel]
		if stop < 0 || stop >= len(strip) {
			return nil, fmt.Errorf("stop %d out of range for reel %d", stop, reel)
		}
		for row := 0; row < SlotRowCount; row++ {
			idx := (stop + row) % len(strip)
			grid[row][reel] = strip[idx]
		}
	}

	return evaluateSlotGrid(grid, stops, betCredits)
}

func evaluateSlotGrid(
	grid [SlotRowCount][SlotReelCount]string,
	stops [SlotReelCount]int,
	betCredits int64,
) (*SlotSpinResult, error) {
	if betCredits <= 0 {
		return nil, fmt.Errorf("bet must be positive")
	}
	result := &SlotSpinResult{
		Grid:             grid,
		Stops:            stops,
		WinningLines:     make([]SlotWinningLine, 0, len(slotWaysPaySymbols)),
		PaytableVersion:  SlotPaytableVersionV10,
		ReelStripVersion: SlotReelStripVersionV11,
		BetCredits:       betCredits,
	}

	wins, linePayout, err := evaluateSlotWays(grid, betCredits)
	if err != nil {
		return nil, err
	}
	result.WinningLines = wins
	result.LinePayout = linePayout

	scatters := 0
	bonuses := 0
	for row := 0; row < SlotRowCount; row++ {
		for reel := 0; reel < SlotReelCount; reel++ {
			switch grid[row][reel] {
			case SlotSymbolScatter:
				scatters++
			case SlotSymbolBonus:
				bonuses++
			}
		}
	}
	result.ScatterCount = scatters
	result.BonusCount = bonuses
	if mult, ok := slotScatterPaysV1[scatters]; ok {
		payout, err := mulCredits(betCredits, mult)
		if err != nil {
			return nil, err
		}
		result.ScatterPayout = payout
	}
	if free, ok := slotScatterFreeSpinsV1[scatters]; ok {
		result.FreeSpinsAwarded = free
	}

	basePayout, err := addCredits(result.LinePayout, result.ScatterPayout)
	if err != nil {
		return nil, err
	}
	result.TotalPayout, err = applySlotBonusPayoutMultiplier(
		basePayout,
		bonuses,
		slotHasBonusOnFifthReel(grid),
	)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// EvaluateSlotSpinV1 is retained for source compatibility. New outcomes use
// the v3 strips and report v3 version constants.
func EvaluateSlotSpinV1(betCredits int64, stops [SlotReelCount]int) (*SlotSpinResult, error) {
	return EvaluateSlotSpinV2(betCredits, stops)
}

var slotWaysPaySymbols = [...]string{
	SlotSymbolJackpot, SlotSymbolDragon, SlotSymbolIngot,
	SlotSymbolJade, SlotSymbolBell,
}

var supportedSlotBets = [...]int64{
	1, 2, 5, 10, 20, 50, 100,
	200, 500, 1_000, 2_000, 5_000, SlotBetMaxCredits,
}

func SlotBetOptions(configuredBet int64) []int64 {
	options := make([]int64, 0, len(supportedSlotBets)+1)
	if configuredBet < 0 || configuredBet > SlotBetMaxCredits {
		configuredBet = 0
	}
	inserted := false
	for _, bet := range supportedSlotBets {
		if !inserted && configuredBet > 0 && configuredBet < bet {
			options = append(options, configuredBet)
			inserted = true
		}
		if bet != configuredBet {
			options = append(options, bet)
		} else if !inserted {
			options = append(options, bet)
			inserted = true
		}
	}
	if configuredBet > supportedSlotBets[len(supportedSlotBets)-1] {
		options = append(options, configuredBet)
	}
	return options
}

func IsSupportedSlotBet(bet, configuredBet int64) bool {
	if bet <= 0 || bet > SlotBetMaxCredits {
		return false
	}
	for _, option := range SlotBetOptions(configuredBet) {
		if bet == option {
			return true
		}
	}
	return false
}

// evaluateSlotWays scores one longest left-to-right result per base symbol.
// A reel counts once when it shows the symbol or WILD, regardless of how many
// matching cells are visible on that reel. BONUS never substitutes for a base
// symbol and is handled separately by the bonus-round and multiplier rules.
func evaluateSlotWays(grid [SlotRowCount][SlotReelCount]string, bet int64) ([]SlotWinningLine, int64, error) {
	if bet <= 0 {
		return nil, 0, fmt.Errorf("bet must be positive")
	}
	wins := make([]SlotWinningLine, 0, len(slotWaysPaySymbols))
	for _, target := range slotWaysPaySymbols {
		pays := slotWaysPaysV10[target]
		positions := make([][2]int, 0, SlotReelCount)
		hasTarget := false
		for reel := 0; reel < SlotReelCount; reel++ {
			position, matched, actualTarget := slotWinningPositionOnReel(grid, target, reel)
			if !matched {
				break
			}
			positions = append(positions, position)
			hasTarget = hasTarget || actualTarget
		}

		count := len(positions)
		mult, paysCount := pays[count]
		if count < 3 || !paysCount || !hasTarget {
			continue
		}
		payout, err := mulCredits(bet, mult)
		if err != nil {
			return nil, 0, err
		}
		wins = append(wins, SlotWinningLine{
			Symbol: target, Count: count, Mult: mult, Payout: payout, Positions: positions,
		})
	}

	if len(wins) > len(slotWaysPaySymbols) {
		return nil, 0, fmt.Errorf("winning ways exceed safe bound")
	}
	for i := range wins {
		wins[i].LineIndex = i + 1
	}
	total, err := normalizeWaysPayouts(wins)
	if err != nil {
		return nil, 0, err
	}
	return wins, total, nil
}

// slotWinningPositionOnReel prefers an actual matching symbol for highlighting
// and uses WILD only when that reel contains no target symbol.
func slotWinningPositionOnReel(
	grid [SlotRowCount][SlotReelCount]string,
	target string,
	reel int,
) (position [2]int, matched bool, actualTarget bool) {
	var wildPosition [2]int
	hasWild := false
	for row := 0; row < SlotRowCount; row++ {
		symbol := grid[row][reel]
		if symbol == target {
			return [2]int{reel, row}, true, true
		}
		if symbol == SlotSymbolWild && !hasWild {
			wildPosition = [2]int{reel, row}
			hasWild = true
		}
	}
	if hasWild {
		return wildPosition, true, false
	}
	return [2]int{}, false, false
}

func normalizeWaysPayouts(wins []SlotWinningLine) (int64, error) {
	var numerator int64
	var allocated int64
	for i := range wins {
		nextNumerator, err := addCredits(numerator, wins[i].Payout)
		if err != nil {
			return 0, err
		}
		nextAllocated := nextNumerator / SlotWaysBetDivisor
		wins[i].Payout = nextAllocated - allocated
		numerator = nextNumerator
		allocated = nextAllocated
	}
	return allocated, nil
}

func mulCredits(bet, mult int64) (int64, error) {
	if bet <= 0 || mult < 0 {
		return 0, fmt.Errorf("invalid credit multiply")
	}
	if mult == 0 {
		return 0, nil
	}
	if bet > math.MaxInt64/mult {
		return 0, fmt.Errorf("credit multiply overflow")
	}
	return bet * mult, nil
}

// slotHasBonusOnFifthReel reports whether the fifth reel contains a visible
// BONUS. One- and two-BONUS base-game multipliers are valid only in this case.
func slotHasBonusOnFifthReel(grid [SlotRowCount][SlotReelCount]string) bool {
	for row := 0; row < SlotRowCount; row++ {
		if grid[row][SlotReelCount-1] == SlotSymbolBonus {
			return true
		}
	}
	return false
}

// applySlotBonusPayoutMultiplier modifies only the completed base-game payout.
// One or two BONUS symbols multiply only when the fifth reel contains BONUS.
// Three or more BONUS symbols create a separate free-spin reward round and do
// not multiply the base payout here.
func applySlotBonusPayoutMultiplier(
	basePayout int64,
	bonusCount int,
	fifthReelHasBonus bool,
) (int64, error) {
	if basePayout < 0 {
		return 0, fmt.Errorf("negative bonus base payout")
	}
	if basePayout == 0 {
		return 0, nil
	}
	if !fifthReelHasBonus {
		return basePayout, nil
	}
	switch bonusCount {
	case 1:
		if basePayout > math.MaxInt64/3 {
			return 0, fmt.Errorf("bonus payout overflow")
		}
		return basePayout * 3 / 2, nil
	case 2:
		return mulCredits(basePayout, 2)
	default:
		return basePayout, nil
	}
}

func addCredits(a, b int64) (int64, error) {
	if a < 0 || b < 0 {
		return 0, fmt.Errorf("negative credit add")
	}
	if a > math.MaxInt64-b {
		return 0, fmt.Errorf("credit add overflow")
	}
	return a + b, nil
}

// SlotSpinResultJSON helpers keep repository metadata compact and stable.
func marshalSlotGrid(grid [SlotRowCount][SlotReelCount]string) (json.RawMessage, error) {
	return json.Marshal(grid)
}

func marshalSlotStops(stops [SlotReelCount]int) (json.RawMessage, error) {
	return json.Marshal(stops)
}

func marshalSlotWinningLines(lines []SlotWinningLine) (json.RawMessage, error) {
	if lines == nil {
		lines = []SlotWinningLine{}
	}
	return json.Marshal(lines)
}

// SlotMaxFreeSpinsCap bounds accumulated free-spin state against abuse/overflow.
func SlotMaxFreeSpinsCap() int { return 1000 }

// EstimateSlotV4RTP enumerates each reel's stops and combines the probability
// that a symbol appears on each consecutive reel. It is the deterministic base
// game RTP before per-spin integer rounding; all BONUS effects and free-spin
// value are excluded.
func EstimateSlotV4RTP() (rtp float64, combinations int64) {
	combinations = 1
	for reel := 0; reel < SlotReelCount; reel++ {
		combinations *= int64(len(slotReelStripsV3[reel]))
	}
	var waysRTP float64
	for _, value := range estimateSlotWaysRTPBySymbol() {
		waysRTP += value
	}
	rtp = waysRTP/SlotWaysBetDivisor + estimateScatterRTP()
	return rtp, combinations
}

func estimateSlotWaysRTPBySymbol() map[string]float64 {
	result := make(map[string]float64, len(slotWaysPaySymbols))
	for _, target := range slotWaysPaySymbols {
		var symbolRTP float64
		matchingPrefix := 1.0
		wildOnlyPrefix := 1.0
		for reel := 0; reel < SlotReelCount; reel++ {
			matchingStops, wildOnlyStops, _ := slotReelWayStats(reel, target)
			matchingPrefix *= matchingStops / float64(len(slotReelStripsV3[reel]))
			wildOnlyPrefix *= wildOnlyStops / float64(len(slotReelStripsV3[reel]))
			count := reel + 1
			mult, pays := slotWaysPaysV10[target][count]
			if !pays {
				continue
			}
			stopProbability := 1.0
			if reel+1 < SlotReelCount {
				_, _, nextZeroStops := slotReelWayStats(reel+1, target)
				stopProbability = float64(nextZeroStops) / float64(len(slotReelStripsV3[reel+1]))
			}
			symbolRTP += (matchingPrefix - wildOnlyPrefix) * stopProbability * float64(mult)
		}
		result[target] = symbolRTP
	}
	return result
}

// EstimateSlotV3RTP is retained for source compatibility.
func EstimateSlotV3RTP() (rtp float64, combinations int64) {
	return EstimateSlotV4RTP()
}

// EstimateSlotV2RTP is retained for source compatibility.
func EstimateSlotV2RTP() (rtp float64, combinations int64) { return EstimateSlotV4RTP() }

func slotReelWayStats(reel int, target string) (matchingStops, wildOnlyStops float64, zeroStops int) {
	strip := slotReelStripsV3[reel]
	for stop := range strip {
		hasTarget := false
		hasWild := false
		for row := 0; row < SlotRowCount; row++ {
			symbol := strip[(stop+row)%len(strip)]
			hasTarget = hasTarget || symbol == target
			hasWild = hasWild || symbol == SlotSymbolWild
		}
		if hasTarget || hasWild {
			matchingStops++
		}
		if !hasTarget && hasWild {
			wildOnlyStops++
		}
		if !hasTarget && !hasWild {
			zeroStops++
		}
	}
	return matchingStops, wildOnlyStops, zeroStops
}

func estimateScatterRTP() float64 {
	distribution := []float64{1}
	for reel := 0; reel < SlotReelCount; reel++ {
		strip := slotReelStripsV3[reel]
		counts := make([]float64, SlotRowCount+1)
		for stop := range strip {
			visible := 0
			for row := 0; row < SlotRowCount; row++ {
				if strip[(stop+row)%len(strip)] == SlotSymbolScatter {
					visible++
				}
			}
			counts[visible]++
		}
		next := make([]float64, len(distribution)+SlotRowCount)
		for total, probability := range distribution {
			for visible, count := range counts {
				next[total+visible] += probability * count / float64(len(strip))
			}
		}
		distribution = next
	}
	var rtp float64
	for count, probability := range distribution {
		rtp += probability * float64(slotScatterPaysV1[count])
	}
	return rtp
}

// EstimateSlotV1RTP is retained for source compatibility.
func EstimateSlotV1RTP() (rtp float64, combinations int64) {
	return EstimateSlotV4RTP()
}
