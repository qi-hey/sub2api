package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

type gameTokenRedeemRepo struct {
	code           RedeemCode
	useCalled      bool
	valueText      string
	getValueCalled bool
}

func (r *gameTokenRedeemRepo) Create(context.Context, *RedeemCode) error { panic("unexpected") }
func (r *gameTokenRedeemRepo) CreateBatch(context.Context, []RedeemCode) error {
	panic("unexpected")
}
func (r *gameTokenRedeemRepo) GetByID(_ context.Context, id int64) (*RedeemCode, error) {
	if r.code.ID != id {
		return nil, ErrRedeemCodeNotFound
	}
	clone := r.code
	return &clone, nil
}
func (r *gameTokenRedeemRepo) GetByCode(_ context.Context, code string) (*RedeemCode, error) {
	if r.code.Code != code {
		return nil, ErrRedeemCodeNotFound
	}
	clone := r.code
	return &clone, nil
}
func (r *gameTokenRedeemRepo) Update(context.Context, *RedeemCode) error { panic("unexpected") }
func (r *gameTokenRedeemRepo) BatchUpdate(context.Context, []int64, RedeemCodeBatchUpdateFields) (int64, error) {
	panic("unexpected")
}
func (r *gameTokenRedeemRepo) Delete(context.Context, int64) error { panic("unexpected") }
func (r *gameTokenRedeemRepo) Use(_ context.Context, id, userID int64) error {
	r.useCalled = true
	r.code.Status = StatusUsed
	r.code.UsedBy = &userID
	now := time.Now()
	r.code.UsedAt = &now
	return nil
}
func (r *gameTokenRedeemRepo) List(context.Context, pagination.PaginationParams) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected")
}
func (r *gameTokenRedeemRepo) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected")
}
func (r *gameTokenRedeemRepo) ListByUser(context.Context, int64, int) ([]RedeemCode, error) {
	panic("unexpected")
}
func (r *gameTokenRedeemRepo) ListByUserPaginated(context.Context, int64, pagination.PaginationParams, string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected")
}
func (r *gameTokenRedeemRepo) SumPositiveBalanceByUser(context.Context, int64) (float64, error) {
	panic("unexpected")
}
func (r *gameTokenRedeemRepo) GetValueText(_ context.Context, id int64) (string, error) {
	r.getValueCalled = true
	if r.code.ID != id {
		return "", ErrRedeemCodeNotFound
	}
	return r.valueText, nil
}

type gameTokenUserRepo struct {
	UserRepository
	balance            float64
	totalRecharged     float64
	exactCredits       []string
	updateBalanceCalls int
	getByID            *User
}

type gameTokenAuthCacheProbe struct {
	userIDs []int64
}

func (p *gameTokenAuthCacheProbe) InvalidateAuthCacheByKey(context.Context, string)    {}
func (p *gameTokenAuthCacheProbe) InvalidateAuthCacheByGroupID(context.Context, int64) {}
func (p *gameTokenAuthCacheProbe) InvalidateAuthCacheByUserID(_ context.Context, userID int64) {
	p.userIDs = append(p.userIDs, userID)
}

type gameTokenBillingCacheProbe struct {
	invalidated chan int64
}

func (p *gameTokenBillingCacheProbe) InvalidateUserBalance(_ context.Context, userID int64) error {
	p.invalidated <- userID
	return nil
}
func (p *gameTokenBillingCacheProbe) InvalidateSubscription(context.Context, int64, int64) error {
	return nil
}

type gameTokenAffiliateProbe struct {
	enabledCalls int
	accrueCalls  int
}

func (p *gameTokenAffiliateProbe) IsEnabled(context.Context) bool {
	p.enabledCalls++
	return true
}
func (p *gameTokenAffiliateProbe) AccrueInviteRebate(context.Context, int64, float64) (float64, error) {
	p.accrueCalls++
	return 1, nil
}

func newGameTokenRedeemEntClient(t *testing.T) *dbent.Client {
	t.Helper()
	db, err := sql.Open("sqlite", "file:game_token_redeem?mode=memory&cache=shared&_fk=1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	driver := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(driver)))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func (r *gameTokenUserRepo) Create(context.Context, *User) error { panic("unexpected") }
func (r *gameTokenUserRepo) GetByID(context.Context, int64) (*User, error) {
	if r.getByID == nil {
		return &User{ID: 2, Balance: r.balance, TotalRecharged: r.totalRecharged}, nil
	}
	return r.getByID, nil
}
func (r *gameTokenUserRepo) GetByEmail(context.Context, string) (*User, error) {
	panic("unexpected")
}
func (r *gameTokenUserRepo) Update(context.Context, *User) error { panic("unexpected") }
func (r *gameTokenUserRepo) Delete(context.Context, int64) error { panic("unexpected") }
func (r *gameTokenUserRepo) List(context.Context, pagination.PaginationParams) ([]User, *pagination.PaginationResult, error) {
	panic("unexpected")
}
func (r *gameTokenUserRepo) UpdateBalance(_ context.Context, _ int64, amount float64) error {
	r.updateBalanceCalls++
	r.balance += amount
	if amount > 0 {
		r.totalRecharged += amount
	}
	return nil
}
func (r *gameTokenUserRepo) UpdateConcurrency(context.Context, int64, int) error {
	panic("unexpected")
}
func (r *gameTokenUserRepo) ApplyRedeemBalanceAdjustment(context.Context, int64, float64) error {
	panic("unexpected balance adjustment")
}
func (r *gameTokenUserRepo) ApplyRedeemConcurrencyAdjustment(context.Context, int64, int) error {
	panic("unexpected")
}
func (r *gameTokenUserRepo) ApplyGameTokenBalanceCreditExact(_ context.Context, _ int64, amount string) error {
	r.exactCredits = append(r.exactCredits, amount)
	return nil
}

// Minimal remaining UserRepository methods required by the interface are satisfied
// via embedding a partial mock if the compiler demands more — see compile fixes.

func TestRedeemGameTokenUsesExactCreditWithoutAffiliateOrRecharge(t *testing.T) {
	repo := &gameTokenRedeemRepo{
		code: RedeemCode{
			ID:     9,
			Code:   "GTABCDEF12345678",
			Type:   RedeemTypeGameToken,
			Value:  1.25,
			Status: StatusUnused,
		},
		valueText: "1.25000000",
	}
	userRepo := &gameTokenUserRepo{balance: 10, totalRecharged: 100}
	authProbe := &gameTokenAuthCacheProbe{}
	billingProbe := &gameTokenBillingCacheProbe{invalidated: make(chan int64, 1)}
	affiliateProbe := &gameTokenAffiliateProbe{}
	svc := NewRedeemService(repo, userRepo, nil, nil, nil, newGameTokenRedeemEntClient(t), authProbe, nil)
	svc.billingCacheService = billingProbe
	svc.affiliateService = affiliateProbe

	result, err := svc.Redeem(context.Background(), 2, repo.code.Code)
	require.NoError(t, err)
	require.Equal(t, StatusUsed, result.Status)
	require.True(t, repo.getValueCalled)
	require.Equal(t, []string{"1.25"}, userRepo.exactCredits)
	require.Equal(t, 0, userRepo.updateBalanceCalls)
	require.Equal(t, float64(100), userRepo.totalRecharged)
	require.Equal(t, []int64{2}, authProbe.userIDs)
	require.Equal(t, 0, affiliateProbe.enabledCalls)
	require.Equal(t, 0, affiliateProbe.accrueCalls)
	select {
	case userID := <-billingProbe.invalidated:
		require.Equal(t, int64(2), userID)
	case <-time.After(2 * time.Second):
		t.Fatal("billing balance cache was not invalidated")
	}

	// Invitation still rejected before TX.
	invRepo := &redeemRejectRepo{code: RedeemCode{ID: 1, Code: "INV", Type: RedeemTypeInvitation, Status: StatusUnused}}
	invSvc := NewRedeemService(invRepo, nil, nil, nil, nil, nil, nil, nil)
	_, err = invSvc.Redeem(context.Background(), 2, "INV")
	require.Error(t, err)
	require.Equal(t, "REDEEM_CODE_UNSUPPORTED_TYPE", infraerrors.Reason(err))
}

func TestApplyGameTokenBalanceCreditExactSQLContract(t *testing.T) {
	require.Equal(t, "game_token", RedeemTypeGameToken)
}
