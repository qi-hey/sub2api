package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

const (
	FruitPaytableVersion = "fruit-v2"
	FruitDoorCount       = 8
	FruitBoardSize       = 24
	FruitMaxBetPerDoor   = int64(100)
	FruitMaxTotalBet     = FruitDoorCount * FruitMaxBetPerDoor

	GameLedgerEntryFruitBet    = "fruit_bet"
	GameLedgerEntryFruitPayout = "fruit_payout"
)

var (
	ErrGameFruitInvalidBets = infraerrors.BadRequest(
		"GAME_FRUIT_BETS_INVALID", "bets must contain positive integer door amounts within the configured limits",
	)
	ErrGameFruitRepositoryUnavailable = infraerrors.New(
		500, "GAME_FRUIT_REPOSITORY_UNAVAILABLE", "fruit machine repository is unavailable",
	)
)

var FruitDoors = [...]string{"bar", "77", "star", "watermelon", "bell", "mango", "orange", "apple"}

type FruitBetRequest map[string]int64

type FruitBoardCell struct {
	Index      int    `json:"index"`
	Kind       string `json:"kind"`
	Symbol     string `json:"symbol,omitempty"`
	Size       string `json:"size,omitempty"`
	Multiplier int64  `json:"multiplier,omitempty"`
	Tier       string `json:"tier,omitempty"`
	Weight     int    `json:"-"`
}

type FruitOutcome struct {
	Kind              string           `json:"kind"`
	CenterIndex       int              `json:"center_index,omitempty"`
	Symbol            string           `json:"symbol,omitempty"`
	Size              string           `json:"size,omitempty"`
	Multiplier        int64            `json:"multiplier,omitempty"`
	ExtraStops        []FruitBoardCell `json:"extra_stops,omitempty"`
	LittleMaryStops   []FruitBoardCell `json:"little_mary_stops,omitempty"`
	JackpotTier       string           `json:"jackpot_tier,omitempty"`
	JackpotMultiplier int64            `json:"jackpot_multiplier,omitempty"`
}

type FruitSpinResult struct {
	ID               int64
	UserID           int64
	Bets             [FruitDoorCount]int64
	TotalBet         int64
	StopIndex        int
	Outcome          FruitOutcome
	Payout           int64
	CreditsBefore    int64
	CreditsAfter     int64
	PaytableVersion  string
	IdempotencyKey   string
	IdempotentReplay bool
	CreatedAt        time.Time
}

type FruitSpinInput struct {
	Bets           [FruitDoorCount]int64
	TotalBet       int64
	IdempotencyKey string
	RequestHash    string
	LocalDate      string
}

type GameFruitRepository interface {
	FindFruitSpin(ctx context.Context, userID int64, idempotencyKey string) (*FruitSpinResult, error)
	FruitSpin(ctx context.Context, userID int64, input FruitSpinInput, outcome *FruitSpinResult) (*FruitSpinResult, error)
}

type FruitEngine struct{ rand io.Reader }

func NewFruitEngine(r io.Reader) *FruitEngine {
	if r == nil {
		r = defaultCryptoReader()
	}
	return &FruitEngine{rand: r}
}

// The physical reference uses a 7+5+7+5 clockwise ring. Its printed ranges
// are configured at their upper values (5-6 => 6, 10-20 => 20, 20-40 => 40).
// Fruit cells total 9,000 virtual stops. The two gold-pig cells total 1,000
// stops and select the existing audited special outcomes in a second draw.
func FruitBoard() []FruitBoardCell {
	cells := []FruitBoardCell{
		{Kind: "fruit", Symbol: "orange", Size: "big", Multiplier: 20, Weight: 91},
		{Kind: "fruit", Symbol: "bell", Size: "big", Multiplier: 20, Weight: 91},
		{Kind: "fruit", Symbol: "bar", Size: "big", Multiplier: 60, Weight: 20},
		{Kind: "fruit", Symbol: "bar", Size: "big", Multiplier: 120, Weight: 24},
		{Kind: "fruit", Symbol: "bar", Size: "big", Multiplier: 80, Weight: 36},
		{Kind: "fruit", Symbol: "apple", Size: "big", Multiplier: 6, Weight: 289},
		{Kind: "fruit", Symbol: "mango", Size: "big", Multiplier: 20, Weight: 91},
		{Kind: "fruit", Symbol: "watermelon", Size: "big", Multiplier: 40, Weight: 84},
		{Kind: "fruit", Symbol: "watermelon", Size: "small", Multiplier: 3, Weight: 1210},
		{Kind: "special", Weight: 500},
		{Kind: "fruit", Symbol: "apple", Size: "big", Multiplier: 6, Weight: 289},
		{Kind: "fruit", Symbol: "orange", Size: "small", Multiplier: 3, Weight: 1112},
		{Kind: "fruit", Symbol: "orange", Size: "big", Multiplier: 20, Weight: 91},
		{Kind: "fruit", Symbol: "bell", Size: "big", Multiplier: 20, Weight: 91},
		{Kind: "fruit", Symbol: "77", Size: "small", Multiplier: 3, Weight: 1210},
		{Kind: "fruit", Symbol: "77", Size: "big", Multiplier: 40, Weight: 84},
		{Kind: "fruit", Symbol: "apple", Size: "big", Multiplier: 6, Weight: 289},
		{Kind: "fruit", Symbol: "mango", Size: "small", Multiplier: 3, Weight: 1112},
		{Kind: "fruit", Symbol: "mango", Size: "big", Multiplier: 20, Weight: 91},
		{Kind: "fruit", Symbol: "star", Size: "big", Multiplier: 40, Weight: 84},
		{Kind: "fruit", Symbol: "star", Size: "small", Multiplier: 3, Weight: 1210},
		{Kind: "special", Weight: 500},
		{Kind: "fruit", Symbol: "apple", Size: "big", Multiplier: 6, Weight: 289},
		{Kind: "fruit", Symbol: "bell", Size: "small", Multiplier: 3, Weight: 1112},
	}
	for index := range cells {
		cells[index].Index = index
	}
	return cells
}

func (e *FruitEngine) Spin(bets [FruitDoorCount]int64) (*FruitSpinResult, error) {
	return e.SpinWithOddsProfile(bets, GameOddsProfileStandard)
}

// SpinWithOddsProfile changes only the shared outer-board distribution. The
// visible board, multipliers, secure RNG, and inner reward tables stay intact.
func (e *FruitEngine) SpinWithOddsProfile(bets [FruitDoorCount]int64, profile GameOddsProfile) (*FruitSpinResult, error) {
	total, err := validateFruitBets(bets)
	if err != nil {
		return nil, err
	}
	cells := FruitBoardForOddsProfile(profile)
	main, err := e.drawCell(cells, 10_000)
	if err != nil {
		return nil, err
	}
	result := &FruitSpinResult{Bets: bets, TotalBet: total, StopIndex: main.Index, PaytableVersion: FruitPaytableVersion}
	result.Outcome = FruitOutcome{Kind: main.Kind, Symbol: main.Symbol, Size: main.Size, Multiplier: main.Multiplier}
	switch main.Kind {
	case "fruit":
		result.Payout, err = fruitCellPayout(main, bets)
	case "special":
		special, drawErr := e.drawSpecialOutcome()
		if drawErr != nil {
			return nil, drawErr
		}
		result.Outcome = FruitOutcome{Kind: special.Kind, CenterIndex: special.Index}
		// Special reward draws retain the R41 table. The global controller
		// adjusts how often the shared special gate is reached, not its prizes.
		rewardCells := fruitRewardBoard(FruitBoard())
		switch special.Kind {
		case "send_light":
			count, drawErr := e.drawSendLightCount()
			if drawErr != nil {
				return nil, drawErr
			}
			result.Outcome.ExtraStops, result.Payout, err = e.drawFruitRewards(rewardCells, count, bets)
		case "little_mary":
			result.Outcome.LittleMaryStops, result.Payout, err = e.drawFruitRewards(rewardCells, 3, bets)
		case "jackpot":
			result.Outcome.JackpotTier = special.Tier
			result.Outcome.JackpotMultiplier = special.Multiplier
			result.Payout, err = checkedMulInt64(total, special.Multiplier)
		default:
			err = fmt.Errorf("unknown fruit special outcome kind %q", special.Kind)
		}
	default:
		err = fmt.Errorf("unknown fruit outcome kind %q", main.Kind)
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

func FruitBoardForOddsProfile(profile GameOddsProfile) []FruitBoardCell {
	cells := FruitBoard()
	specialWeight := fruitSpecialWeightForOddsProfile(profile)
	if specialWeight == 1_000 {
		return cells
	}

	const standardFruitWeight = 9_000
	targetFruitWeight := 10_000 - specialWeight
	type remainder struct {
		index int
		value int
	}
	remainders := make([]remainder, 0, len(cells)-2)
	assigned := 0
	for index := range cells {
		if cells[index].Kind != "fruit" {
			continue
		}
		product := cells[index].Weight * targetFruitWeight
		cells[index].Weight = product / standardFruitWeight
		assigned += cells[index].Weight
		remainders = append(remainders, remainder{index: index, value: product % standardFruitWeight})
	}
	sort.SliceStable(remainders, func(i, j int) bool {
		return remainders[i].value > remainders[j].value
	})
	for i := 0; i < targetFruitWeight-assigned; i++ {
		cells[remainders[i%len(remainders)].index].Weight++
	}

	specialAssigned := 0
	for index := range cells {
		if cells[index].Kind != "special" {
			continue
		}
		cells[index].Weight = specialWeight / 2
		if specialAssigned == 0 {
			cells[index].Weight += specialWeight % 2
		}
		specialAssigned++
	}
	return cells
}

func fruitSpecialWeightForOddsProfile(profile GameOddsProfile) int {
	switch NormalizeGameOddsProfile(profile) {
	case GameOddsProfileCooling:
		return 700
	case GameOddsProfileTight:
		return 400
	default:
		return 1_000
	}
}

func fruitRewardBoard(cells []FruitBoardCell) []FruitBoardCell {
	rewards := make([]FruitBoardCell, 0, len(cells)-2)
	for _, cell := range cells {
		if cell.Kind == "fruit" {
			rewards = append(rewards, cell)
		}
	}
	return rewards
}

func (e *FruitEngine) drawSpecialOutcome() (FruitBoardCell, error) {
	// Center indices match the 16-light reference wheel: train=14,
	// small-three=10, and jackpot=4.
	outcomes := []FruitBoardCell{
		{Index: 14, Kind: "send_light", Weight: 800},
		{Index: 10, Kind: "little_mary", Weight: 150},
		{Index: 4, Kind: "jackpot", Tier: "bronze", Multiplier: 8, Weight: 30},
		{Index: 4, Kind: "jackpot", Tier: "silver", Multiplier: 25, Weight: 15},
		{Index: 4, Kind: "jackpot", Tier: "gold", Multiplier: 80, Weight: 5},
	}
	return e.drawCell(outcomes, 1_000)
}

func (e *FruitEngine) drawCell(cells []FruitBoardCell, totalWeight int) (FruitBoardCell, error) {
	draw, err := cryptoUniformInt(e.rand, totalWeight)
	if err != nil {
		return FruitBoardCell{}, err
	}
	for _, cell := range cells {
		if draw < cell.Weight {
			return cell, nil
		}
		draw -= cell.Weight
	}
	return FruitBoardCell{}, fmt.Errorf("fruit weight table does not cover draw")
}

func (e *FruitEngine) drawSendLightCount() (int, error) {
	draw, err := cryptoUniformInt(e.rand, 100)
	if err != nil {
		return 0, err
	}
	if draw < 60 {
		return 1, nil
	}
	if draw < 90 {
		return 2, nil
	}
	return 3, nil
}

func (e *FruitEngine) drawFruitRewards(cells []FruitBoardCell, count int, bets [FruitDoorCount]int64) ([]FruitBoardCell, int64, error) {
	stops := make([]FruitBoardCell, 0, count)
	var payout int64
	for range count {
		cell, err := e.drawCell(cells, 9_000)
		if err != nil {
			return nil, 0, err
		}
		win, err := fruitCellPayout(cell, bets)
		if err != nil || payout > math.MaxInt64-win {
			return nil, 0, fmt.Errorf("fruit payout overflow")
		}
		payout += win
		stops = append(stops, cell)
	}
	return stops, payout, nil
}

func fruitCellPayout(cell FruitBoardCell, bets [FruitDoorCount]int64) (int64, error) {
	for i, door := range FruitDoors {
		if door == cell.Symbol {
			return checkedMulInt64(bets[i], cell.Multiplier)
		}
	}
	return 0, nil
}

func checkedMulInt64(a, b int64) (int64, error) {
	if a < 0 || b < 0 || (a != 0 && b > math.MaxInt64/a) {
		return 0, fmt.Errorf("integer multiplication overflow")
	}
	return a * b, nil
}

func validateFruitBets(bets [FruitDoorCount]int64) (int64, error) {
	var total int64
	for _, bet := range bets {
		if bet < 0 || bet > FruitMaxBetPerDoor || total > FruitMaxTotalBet-bet {
			return 0, ErrGameFruitInvalidBets
		}
		total += bet
	}
	if total <= 0 || total > FruitMaxTotalBet {
		return 0, ErrGameFruitInvalidBets
	}
	return total, nil
}

func normalizeFruitBets(input FruitBetRequest) ([FruitDoorCount]int64, int64, error) {
	var bets [FruitDoorCount]int64
	if len(input) == 0 || len(input) > FruitDoorCount {
		return bets, 0, ErrGameFruitInvalidBets
	}
	known := make(map[string]int, FruitDoorCount)
	for i, door := range FruitDoors {
		known[door] = i
	}
	seen := make(map[string]struct{}, len(input))
	for rawDoor, bet := range input {
		door := strings.ToLower(strings.TrimSpace(rawDoor))
		index, ok := known[door]
		if !ok || bet < 0 {
			return bets, 0, ErrGameFruitInvalidBets
		}
		if _, duplicate := seen[door]; duplicate {
			return bets, 0, ErrGameFruitInvalidBets
		}
		seen[door] = struct{}{}
		bets[index] = bet
	}
	total, err := validateFruitBets(bets)
	return bets, total, err
}

func (s *GameLoyaltyService) WithFruitEngine(engine *FruitEngine) *GameLoyaltyService {
	if engine != nil {
		s.fruit = engine
	}
	return s
}

func (s *GameLoyaltyService) FruitSpin(ctx context.Context, userID int64, idempotencyKey string, requested FruitBetRequest) (*FruitSpinResult, error) {
	key, err := normalizeGameWalletIdempotencyKey(idempotencyKey)
	if err != nil {
		return nil, err
	}
	cfg := s.getConfig(ctx)
	if !cfg.Enabled {
		return nil, ErrGameLoyaltyDisabled
	}
	if !cfg.Available() {
		return nil, ErrGameLoyaltyNotConfigured
	}
	bets, total, err := normalizeFruitBets(requested)
	if err != nil {
		return nil, err
	}
	repo, ok := s.repo.(GameFruitRepository)
	if !ok {
		return nil, ErrGameFruitRepositoryUnavailable
	}
	existing, err := repo.FindFruitSpin(ctx, userID, key)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if existing.Bets != bets || existing.TotalBet != total {
			return nil, ErrGameLoyaltyIdempotencyConflict
		}
		existing.IdempotentReplay = true
		return existing, nil
	}

	parts := []string{"fruit-spin"}
	for _, bet := range bets {
		parts = append(parts, strconv.FormatInt(bet, 10))
	}
	hash := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	engine := s.fruit
	if engine == nil {
		engine = NewFruitEngine(nil)
	}
	profile := s.gameOddsProfile(ctx, GameOddsPoolFruit)
	outcome, err := engine.SpinWithOddsProfile(bets, profile)
	if err != nil {
		return nil, ErrGameLoyaltyRNGFailed
	}
	return repo.FruitSpin(ctx, userID, FruitSpinInput{
		Bets: bets, TotalBet: total, IdempotencyKey: key,
		RequestHash: hex.EncodeToString(hash[:]), LocalDate: timezone.Today().Format("2006-01-02"),
	}, outcome)
}
