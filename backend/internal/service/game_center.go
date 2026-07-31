package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

const (
	GameIDSnake     = "snake"
	GameIDTetris    = "tetris"
	GameIDBreakout  = "breakout"
	GameIDMerge2048 = "merge2048"
	GameIDOrbital   = "orbital"
	GameIDHoarder   = "hoarder"
	GameIDLucky     = "lucky"

	GameLedgerEntrySlotBonus = "slot_bonus"
	SlotBonusProtocolV2      = "slot-bonus-v2"
	SlotBonusProtocolV3      = "slot-bonus-v3"
	SlotBonusProtocolV4      = "slot-bonus-v4"
	SlotBonusKindPickChest   = "pick_chest"
	SlotBonusKindLuckyWheel  = "lucky_wheel"
	SlotBonusKindFreeSpins   = "free_spins"

	slotBonusChoiceCount = 3
	slotBonusTTL         = 10 * time.Minute
	gameScoreMax         = int64(1_000_000_000)
)

var localLeaderboardGames = map[string]struct{}{
	GameIDSnake: {}, GameIDTetris: {}, GameIDBreakout: {}, GameIDMerge2048: {},
	GameIDOrbital: {}, GameIDHoarder: {},
}

var (
	ErrGameLeaderboardGameInvalid = infraerrors.BadRequest(
		"GAME_LEADERBOARD_GAME_INVALID", "game_id is not supported",
	)
	ErrGameLeaderboardLuckyClientScore = infraerrors.BadRequest(
		"GAME_LEADERBOARD_LUCKY_SERVER_ONLY", "lucky scores are recorded only by server settlement",
	)
	ErrGameLeaderboardScoreInvalid = infraerrors.BadRequest(
		"GAME_LEADERBOARD_SCORE_INVALID", "score must be an integer between 0 and 1000000000",
	)
	ErrGameSlotBonusTokenInvalid = infraerrors.NotFound(
		"GAME_SLOT_BONUS_NOT_FOUND", "slot bonus round was not found",
	)
	ErrGameSlotBonusChoiceInvalid = infraerrors.BadRequest(
		"GAME_SLOT_BONUS_CHOICE_INVALID", "choice must be between 0 and 2",
	)
	ErrGameSlotBonusExpired = infraerrors.New(
		410, "GAME_SLOT_BONUS_EXPIRED", "slot bonus round has expired",
	)
	ErrGameSlotBonusAlreadyClaimed = infraerrors.Conflict(
		"GAME_SLOT_BONUS_ALREADY_CLAIMED", "slot bonus round was already claimed",
	)
	errGameCenterRepositoryUnavailable = infraerrors.New(
		500, "GAME_CENTER_REPOSITORY_UNAVAILABLE", "game center repository is unavailable",
	)
)

type SlotBonusRoundOfferInput struct {
	Kind      string
	Token     string
	TokenHash string
	LocalDate string
	Rewards   [slotBonusChoiceCount]int64
	Plan      *SlotBonusPlan
	ExpiresAt time.Time
}

type SlotBonusOption struct {
	Choice        int               `json:"choice"`
	Spins         int               `json:"spins"`
	Multiplier    int               `json:"multiplier"`
	BasePayout    int64             `json:"base_payout"`
	RewardCredits int64             `json:"reward_credits"`
	SpinResults   []*SlotSpinResult `json:"spin_results"`
}

type SlotBonusPlan struct {
	Kind            string            `json:"kind"`
	Protocol        string            `json:"protocol"`
	BetCredits      int64             `json:"bet_credits"`
	BonusCount      int               `json:"bonus_count"`
	BonusMultiplier int               `json:"bonus_multiplier"`
	Options         []SlotBonusOption `json:"options"`
}

type SlotBonusOptionView struct {
	Choice     int `json:"choice"`
	Spins      int `json:"spins"`
	Multiplier int `json:"multiplier"`
}

type SlotBonusRoundOffer struct {
	RoundID         int64
	Kind            string
	Token           string
	ExpiresAt       time.Time
	Choices         int
	Status          string
	BetCredits      int64
	BonusCount      int
	BonusMultiplier int
	Options         []SlotBonusOptionView
}

// SlotBonusPendingSnapshot contains the persisted fields needed to restore an
// unclaimed bonus offer without storing its bearer token in plaintext.
type SlotBonusPendingSnapshot struct {
	RoundID            int64
	Kind               string
	SpinIdempotencyKey string
	ExpiresAt          time.Time
	Choices            int
	Status             string
	BetCredits         int64
	BonusCount         int
	BonusMultiplier    int
	Options            []SlotBonusOptionView
}

type SlotBonusClaimInput struct {
	TokenHash      string
	Choice         int
	IdempotencyKey string
	RequestHash    string
	Now            time.Time
}

type SlotBonusClaimResult struct {
	RoundID          int64
	Kind             string
	Choice           int
	RewardCredits    int64
	CreditsAfter     int64
	ClaimedAt        time.Time
	IdempotentReplay bool
	Spins            int
	Multiplier       int
	BonusMultiplier  int
	BasePayout       int64
	SpinResults      []*SlotSpinResult
}

type GameLeaderboardEntry struct {
	Rank        int64
	UserID      int64
	Username    string
	Email       string
	Score       int64
	AchievedAt  time.Time
	CurrentUser bool
	UserDisplay string
}

type GameLeaderboard struct {
	GameID    string
	LocalDate string
	Items     []GameLeaderboardEntry
	MyEntry   *GameLeaderboardEntry
}

type GameCenterRepository interface {
	SubmitDailyScore(ctx context.Context, userID int64, localDate, gameID string, score int64) error
	GetAllTimeLeaderboard(ctx context.Context, userID int64, gameID string, limit int) ([]GameLeaderboardEntry, error)
	ClaimSlotBonus(ctx context.Context, userID int64, input SlotBonusClaimInput) (*SlotBonusClaimResult, error)
}

// GameSlotBonusSnapshotRepository is optional so existing loyalty repository
// implementations remain compatible while the SQL repository supports restore.
type GameSlotBonusSnapshotRepository interface {
	GetPendingSlotBonus(ctx context.Context, userID int64, now time.Time) (*SlotBonusPendingSnapshot, error)
}

func (s *GameLoyaltyService) SubmitLeaderboardScore(ctx context.Context, userID int64, gameID string, score int64) error {
	gameID = strings.TrimSpace(strings.ToLower(gameID))
	if gameID == GameIDLucky {
		return ErrGameLeaderboardLuckyClientScore
	}
	if _, ok := localLeaderboardGames[gameID]; !ok {
		return ErrGameLeaderboardGameInvalid
	}
	if score < 0 || score > gameScoreMax {
		return ErrGameLeaderboardScoreInvalid
	}
	repo, ok := s.repo.(GameCenterRepository)
	if !ok {
		return errGameCenterRepositoryUnavailable
	}
	return repo.SubmitDailyScore(ctx, userID, timezone.Today().Format("2006-01-02"), gameID, score)
}

func (s *GameLoyaltyService) GetLeaderboard(ctx context.Context, userID int64, gameID string, limit int) (*GameLeaderboard, error) {
	gameID = strings.TrimSpace(strings.ToLower(gameID))
	if !isLeaderboardGame(gameID) {
		return nil, ErrGameLeaderboardGameInvalid
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	repo, ok := s.repo.(GameCenterRepository)
	if !ok {
		return nil, errGameCenterRepositoryUnavailable
	}
	rows, err := repo.GetAllTimeLeaderboard(ctx, userID, gameID, limit)
	if err != nil {
		return nil, err
	}
	result := &GameLeaderboard{GameID: gameID, Items: make([]GameLeaderboardEntry, 0, limit)}
	for i := range rows {
		rows[i].UserDisplay = leaderboardUserDisplay(rows[i].Username, rows[i].Email)
		rows[i].CurrentUser = rows[i].UserID == userID
		if rows[i].Rank <= int64(limit) {
			result.Items = append(result.Items, rows[i])
		}
		if rows[i].CurrentUser {
			entry := rows[i]
			result.MyEntry = &entry
		}
	}
	return result, nil
}

func (s *GameLoyaltyService) ClaimSlotBonus(ctx context.Context, userID int64, idempotencyKey, token string, choice int) (*SlotBonusClaimResult, error) {
	key, err := normalizeGameWalletIdempotencyKey(idempotencyKey)
	if err != nil {
		return nil, err
	}
	if choice < 0 || choice >= slotBonusChoiceCount {
		return nil, ErrGameSlotBonusChoiceInvalid
	}
	token = strings.TrimSpace(token)
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(decoded) != 32 || len(token) > 64 {
		return nil, ErrGameSlotBonusTokenInvalid
	}
	tokenDigest := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(tokenDigest[:])
	requestDigest := sha256.Sum256([]byte("slot_bonus_claim\n" + tokenHash + "\n" + strconv.Itoa(choice)))
	repo, ok := s.repo.(GameCenterRepository)
	if !ok {
		return nil, errGameCenterRepositoryUnavailable
	}
	return repo.ClaimSlotBonus(ctx, userID, SlotBonusClaimInput{
		TokenHash:      tokenHash,
		Choice:         choice,
		IdempotencyKey: key,
		RequestHash:    hex.EncodeToString(requestDigest[:]),
		Now:            timezone.Now(),
	})
}

func (s *GameLoyaltyService) newSlotBonusOffer(userID int64, idempotencyKey, localDate string, betCredits int64, bonusCount int) (*SlotBonusRoundOfferInput, error) {
	token := deriveSlotBonusToken(userID, idempotencyKey)
	digest := sha256.Sum256([]byte(token))
	bonusMultiplier := slotBonusCountMultiplier(bonusCount)
	if bonusMultiplier == 0 {
		return nil, fmt.Errorf("invalid bonus count %d", bonusCount)
	}
	configs := [...]SlotBonusOptionView{
		{Choice: 0, Spins: 8, Multiplier: 5},
		{Choice: 1, Spins: 12, Multiplier: 3},
		{Choice: 2, Spins: 20, Multiplier: 2},
	}
	plan := &SlotBonusPlan{
		Kind: SlotBonusKindFreeSpins, Protocol: SlotBonusProtocolV4,
		BetCredits: betCredits, BonusCount: bonusCount, BonusMultiplier: bonusMultiplier,
		Options: make([]SlotBonusOption, 0, len(configs)),
	}
	for _, config := range configs {
		option := SlotBonusOption{
			Choice: config.Choice, Spins: config.Spins, Multiplier: config.Multiplier,
			SpinResults: make([]*SlotSpinResult, 0, config.Spins),
		}
		for spinIndex := 0; spinIndex < config.Spins; spinIndex++ {
			spin, err := s.slot.SpinWithoutBonusTrigger(betCredits)
			if err != nil {
				return nil, fmt.Errorf("generate bonus spin %d/%d: %w", spinIndex+1, config.Spins, err)
			}
			option.SpinResults = append(option.SpinResults, spin)
		}
		if err := ensureSlotBonusFeatureHits(&option, betCredits); err != nil {
			return nil, err
		}
		basePayout, err := sumSlotBonusSpinPayouts(option.SpinResults)
		if err != nil {
			return nil, err
		}
		option.BasePayout = basePayout
		reward, err := SlotBonusFeatureRewardCredits(
			option.BasePayout, betCredits, config.Multiplier, bonusMultiplier,
		)
		if err != nil {
			return nil, err
		}
		option.RewardCredits = reward
		plan.Options = append(plan.Options, option)
	}
	return &SlotBonusRoundOfferInput{
		Kind:      SlotBonusKindFreeSpins,
		Token:     token,
		TokenHash: hex.EncodeToString(digest[:]),
		LocalDate: localDate,
		Plan:      plan,
		ExpiresAt: timezone.Now().Add(slotBonusTTL),
	}, nil
}

func slotBonusCountMultiplier(count int) int {
	switch count {
	case 3:
		return 1
	case 4:
		return 2
	case 5:
		return 4
	default:
		return 0
	}
}

// SlotBonusMinimumWinningSpins keeps each free-spin option visibly active.
// The values are about a 40%% hit floor: 3/8, 5/12, and 8/20 spins.
func SlotBonusMinimumWinningSpins(spins int) (int, error) {
	switch spins {
	case 8:
		return 3, nil
	case 12:
		return 5, nil
	case 20:
		return 8, nil
	default:
		return 0, fmt.Errorf("unsupported bonus spin count %d", spins)
	}
}

// SlotBonusMinimumRewardCredits prevents a completed free-spin feature from
// ending as an effectively empty reward. The 3/4/5-BONUS multiplier remains
// part of the floor, so larger triggers still have a larger guarantee.
func SlotBonusMinimumRewardCredits(betCredits int64, bonusMultiplier int) (int64, error) {
	minimum, err := mulCredits(betCredits, 5)
	if err != nil {
		return 0, err
	}
	return mulCredits(minimum, int64(bonusMultiplier))
}

// SlotBonusFeatureRewardCredits calculates the visible feature reward after
// the selected free-spin multiplier and the trigger's BONUS multiplier.
func SlotBonusFeatureRewardCredits(
	basePayout, betCredits int64, optionMultiplier, bonusMultiplier int,
) (int64, error) {
	if basePayout < 0 || optionMultiplier <= 0 || bonusMultiplier <= 0 {
		return 0, fmt.Errorf("invalid free-spin bonus reward inputs")
	}
	reward := int64(0)
	if basePayout > 0 {
		var err error
		reward, err = mulCredits(basePayout, int64(optionMultiplier))
		if err != nil {
			return 0, err
		}
		reward, err = mulCredits(reward, int64(bonusMultiplier))
		if err != nil {
			return 0, err
		}
	}
	minimum, err := SlotBonusMinimumRewardCredits(betCredits, bonusMultiplier)
	if err != nil {
		return 0, err
	}
	if reward < minimum {
		return minimum, nil
	}
	return reward, nil
}

func ensureSlotBonusFeatureHits(option *SlotBonusOption, betCredits int64) error {
	minimumWins, err := SlotBonusMinimumWinningSpins(option.Spins)
	if err != nil {
		return err
	}
	wins := 0
	for _, spin := range option.SpinResults {
		if spin != nil && spin.TotalPayout > 0 {
			wins++
		}
	}
	for index, spin := range option.SpinResults {
		if wins >= minimumWins {
			break
		}
		if spin == nil || spin.TotalPayout > 0 {
			continue
		}
		guaranteed, err := bonusFeatureGuaranteedWinningSpin(betCredits)
		if err != nil {
			return err
		}
		option.SpinResults[index] = guaranteed
		wins++
	}
	if wins < minimumWins {
		return fmt.Errorf("free-spin feature has only %d of %d guaranteed hits", wins, minimumWins)
	}
	return nil
}

func sumSlotBonusSpinPayouts(spins []*SlotSpinResult) (int64, error) {
	var total int64
	for _, spin := range spins {
		if spin == nil {
			return 0, fmt.Errorf("nil free-spin result")
		}
		var err error
		total, err = addCredits(total, spin.TotalPayout)
		if err != nil {
			return 0, err
		}
	}
	return total, nil
}

func deriveSlotBonusToken(userID int64, idempotencyKey string) string {
	digest := sha256.Sum256([]byte(
		"game-slot-bonus-token-v1\n" + strconv.FormatInt(userID, 10) + "\n" + idempotencyKey,
	))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func isLeaderboardGame(gameID string) bool {
	if gameID == GameIDLucky {
		return true
	}
	_, ok := localLeaderboardGames[gameID]
	return ok
}

func leaderboardUserDisplay(username, email string) string {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(email)
	if username != "" {
		if parsed, ok := leaderboardEmailAddress(username); ok {
			return maskLeaderboardEmail(parsed)
		}
		if email != "" && strings.EqualFold(username, email) {
			return maskLeaderboardEmail(email)
		}
		return username
	}
	return maskLeaderboardEmail(email)
}

func leaderboardEmailAddress(value string) (string, bool) {
	address, err := mail.ParseAddress(strings.TrimSpace(value))
	if err != nil || address.Address == "" {
		return "", false
	}
	return address.Address, true
}

func maskLeaderboardEmail(email string) string {
	email = strings.TrimSpace(email)
	if parsed, ok := leaderboardEmailAddress(email); ok {
		email = parsed
	}
	at := strings.LastIndexByte(email, '@')
	if at <= 0 || at == len(email)-1 {
		digest := sha256.Sum256([]byte(email))
		return "player-" + hex.EncodeToString(digest[:4])
	}
	local := email[:at]
	first, _ := utf8.DecodeRuneInString(local)
	if first == utf8.RuneError && len(local) == 0 {
		first = '*'
	}
	return fmt.Sprintf("%c***@%s", first, email[at+1:])
}
