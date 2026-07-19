package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type apiKeyRPMOverrideRepoStub struct {
	UserGroupRateRepository
	overrides    map[int64]*int
	bulkErr      error
	bulkCalls    int
	bulkGroupIDs [][]int64
	singleErr    error
	singleCalls  []int64
}

func (s *apiKeyRPMOverrideRepoStub) GetRPMOverrideByUserAndGroup(_ context.Context, _, groupID int64) (*int, error) {
	s.singleCalls = append(s.singleCalls, groupID)
	if s.singleErr != nil {
		return nil, s.singleErr
	}
	return s.overrides[groupID], nil
}

func (s *apiKeyRPMOverrideRepoStub) GetRPMOverridesByUserAndGroupIDs(_ context.Context, _ int64, groupIDs []int64) (map[int64]int, error) {
	s.bulkCalls++
	s.bulkGroupIDs = append(s.bulkGroupIDs, append([]int64(nil), groupIDs...))
	if s.bulkErr != nil {
		return nil, s.bulkErr
	}
	result := make(map[int64]int)
	for _, groupID := range groupIDs {
		if override := s.overrides[groupID]; override != nil {
			result[groupID] = *override
		}
	}
	return result, nil
}

type apiKeyRPMCacheNoop struct{}

func (apiKeyRPMCacheNoop) IncrementUserGroupRPM(context.Context, int64, int64) (int, error) {
	return 1, nil
}

func (apiKeyRPMCacheNoop) IncrementUserRPM(context.Context, int64) (int, error) {
	return 1, nil
}

func (apiKeyRPMCacheNoop) GetUserGroupRPM(context.Context, int64, int64) (int, error) {
	return 0, nil
}

func (apiKeyRPMCacheNoop) GetUserRPM(context.Context, int64) (int, error) {
	return 0, nil
}

func TestAPIKeyWithSelectedGroupReturnsRequestLocalCopy(t *testing.T) {
	defaultGroupID := int64(2)
	key := &APIKey{
		GroupID:  &defaultGroupID,
		Group:    &Group{ID: 2, Name: "Codex"},
		GroupIDs: []int64{2, 11, 12},
		Groups: []Group{
			{ID: 2, Name: "Codex"},
			{ID: 11, Name: "Claude"},
			{ID: 12, Name: "Grok"},
		},
	}

	selected, err := key.WithSelectedGroup(12)
	require.NoError(t, err)
	require.NotSame(t, key, selected)
	require.Equal(t, int64(12), *selected.GroupID)
	require.Equal(t, "Grok", selected.Group.Name)
	require.Equal(t, int64(2), *key.GroupID)
	require.Equal(t, "Codex", key.Group.Name)
}

func TestAPIKeyWithSelectedGroupRejectsUnboundGroup(t *testing.T) {
	defaultGroupID := int64(2)
	key := &APIKey{
		GroupID: &defaultGroupID,
		Group:   &Group{ID: 2, Name: "Codex"},
		Groups:  []Group{{ID: 2, Name: "Codex"}},
	}

	selected, err := key.WithSelectedGroup(12)
	require.Nil(t, selected)
	require.ErrorIs(t, err, ErrAPIKeyGroupNotBound)
}

func TestAPIKeyWithSelectedGroupSupportsLegacyHydration(t *testing.T) {
	defaultGroupID := int64(2)
	key := &APIKey{
		GroupID: &defaultGroupID,
		Group:   &Group{ID: 2, Name: "Codex"},
	}

	selected, err := key.WithSelectedGroup(2)
	require.NoError(t, err)
	require.Equal(t, int64(2), *selected.GroupID)
	require.Equal(t, "Codex", selected.Group.Name)
}

func TestAPIKeyAuthSnapshotSelectsGroupSpecificRPMOverride(t *testing.T) {
	defaultOverride := 0
	secondaryOverride := 10
	defaultGroupID := int64(2)
	apiKey := &APIKey{
		ID:       1,
		UserID:   7,
		Key:      "k-group-rpm-overrides",
		GroupID:  &defaultGroupID,
		Group:    &Group{ID: defaultGroupID, Name: "Default"},
		GroupIDs: []int64{defaultGroupID, 12, 13},
		Groups: []Group{
			{ID: defaultGroupID, Name: "Default"},
			{ID: 12, Name: "Secondary"},
			{ID: 13, Name: "No Override"},
		},
		User: &User{ID: 7, UserGroupRPMOverride: &secondaryOverride},
	}
	rpmRepo := &apiKeyRPMOverrideRepoStub{overrides: map[int64]*int{
		defaultGroupID: &defaultOverride,
		12:             &secondaryOverride,
	}}
	svc := &APIKeyService{userGroupRateRepo: rpmRepo}

	snapshot := svc.snapshotFromAPIKey(context.Background(), apiKey)
	require.Equal(t, 1, rpmRepo.bulkCalls)
	require.Equal(t, [][]int64{{defaultGroupID, 12, 13}}, rpmRepo.bulkGroupIDs)
	require.Empty(t, rpmRepo.singleCalls)
	roundTrip := svc.snapshotToAPIKey(apiKey.Key, snapshot)

	selectedDefault, err := roundTrip.WithSelectedGroup(defaultGroupID)
	require.NoError(t, err)
	require.NotNil(t, selectedDefault.User.UserGroupRPMOverride)
	require.Equal(t, 0, *selectedDefault.User.UserGroupRPMOverride)
	require.True(t, selectedDefault.User.UserGroupRPMOverrideResolved)

	selectedSecondary, err := roundTrip.WithSelectedGroup(12)
	require.NoError(t, err)
	require.NotNil(t, selectedSecondary.User.UserGroupRPMOverride)
	require.Equal(t, 10, *selectedSecondary.User.UserGroupRPMOverride)
	require.True(t, selectedSecondary.User.UserGroupRPMOverrideResolved)

	selectedWithoutOverride, err := roundTrip.WithSelectedGroup(13)
	require.NoError(t, err)
	require.Nil(t, selectedWithoutOverride.User.UserGroupRPMOverride)
	require.True(t, selectedWithoutOverride.User.UserGroupRPMOverrideResolved)
	require.True(t, roundTrip.GroupRPMOverridesResolved)
	require.True(t, snapshot.GroupRPMOverridesResolved)
	require.Equal(t, defaultGroupID, *roundTrip.GroupID)
}

func TestAPIKeyAuthSnapshotQueriesLegacyDefaultGroupRPMOverride(t *testing.T) {
	groupID := int64(42)
	override := 3
	rpmRepo := &apiKeyRPMOverrideRepoStub{overrides: map[int64]*int{groupID: &override}}
	svc := &APIKeyService{userGroupRateRepo: rpmRepo}
	apiKey := &APIKey{
		ID:      1,
		UserID:  7,
		Key:     "k-legacy-group-rpm-override",
		GroupID: &groupID,
		Group:   &Group{ID: groupID},
		User:    &User{ID: 7},
	}

	snapshot := svc.snapshotFromAPIKey(context.Background(), apiKey)
	require.Equal(t, 1, rpmRepo.bulkCalls)
	require.Equal(t, [][]int64{{groupID}}, rpmRepo.bulkGroupIDs)
	require.Empty(t, rpmRepo.singleCalls)
	require.Equal(t, 3, snapshot.GroupRPMOverrides[groupID])
	roundTrip := svc.snapshotToAPIKey(apiKey.Key, snapshot)
	selected, err := roundTrip.WithSelectedGroup(groupID)
	require.NoError(t, err)
	require.NotNil(t, selected.User.UserGroupRPMOverride)
	require.Equal(t, 3, *selected.User.UserGroupRPMOverride)
	require.True(t, selected.User.UserGroupRPMOverrideResolved)
}

func TestAPIKeyResolvedMissingRPMOverrideSkipsBillingFallback(t *testing.T) {
	defaultGroupID := int64(2)
	secondaryGroupID := int64(12)
	defaultOverride := 5
	rpmRepo := &apiKeyRPMOverrideRepoStub{overrides: map[int64]*int{defaultGroupID: &defaultOverride}}
	svc := &APIKeyService{userGroupRateRepo: rpmRepo}
	apiKey := &APIKey{
		ID:       1,
		UserID:   7,
		Key:      "k-resolved-missing-rpm",
		GroupID:  &defaultGroupID,
		Group:    &Group{ID: defaultGroupID},
		GroupIDs: []int64{defaultGroupID, secondaryGroupID},
		Groups:   []Group{{ID: defaultGroupID}, {ID: secondaryGroupID}},
		User:     &User{ID: 7},
	}

	snapshot := svc.snapshotFromAPIKey(context.Background(), apiKey)
	require.True(t, snapshot.GroupRPMOverridesResolved)
	roundTrip := svc.snapshotToAPIKey(apiKey.Key, snapshot)
	require.True(t, roundTrip.GroupRPMOverridesResolved)
	selected, err := roundTrip.WithSelectedGroup(secondaryGroupID)
	require.NoError(t, err)
	require.Nil(t, selected.User.UserGroupRPMOverride)
	require.True(t, selected.User.UserGroupRPMOverrideResolved)

	billing := &BillingCacheService{userRPMCache: apiKeyRPMCacheNoop{}, userGroupRateRepo: rpmRepo}
	require.NoError(t, billing.checkRPM(context.Background(), selected.User, selected.Group))
	require.Empty(t, rpmRepo.singleCalls)
	require.Equal(t, 1, rpmRepo.bulkCalls)
}

func TestAPIKeyUnresolvedRPMOverrideFallsBackDuringBilling(t *testing.T) {
	defaultGroupID := int64(2)
	secondaryGroupID := int64(12)
	fallbackOverride := 4
	rpmRepo := &apiKeyRPMOverrideRepoStub{
		overrides: map[int64]*int{secondaryGroupID: &fallbackOverride},
		bulkErr:   errors.New("bulk lookup failed"),
	}
	svc := &APIKeyService{userGroupRateRepo: rpmRepo}
	apiKey := &APIKey{
		ID:       1,
		UserID:   7,
		Key:      "k-unresolved-rpm",
		GroupID:  &defaultGroupID,
		Group:    &Group{ID: defaultGroupID},
		GroupIDs: []int64{defaultGroupID, secondaryGroupID},
		Groups:   []Group{{ID: defaultGroupID}, {ID: secondaryGroupID}},
		User:     &User{ID: 7},
	}

	snapshot := svc.snapshotFromAPIKey(context.Background(), apiKey)
	require.False(t, snapshot.GroupRPMOverridesResolved)
	roundTrip := svc.snapshotToAPIKey(apiKey.Key, snapshot)
	require.False(t, roundTrip.GroupRPMOverridesResolved)
	selected, err := roundTrip.WithSelectedGroup(secondaryGroupID)
	require.NoError(t, err)
	require.Nil(t, selected.User.UserGroupRPMOverride)
	require.False(t, selected.User.UserGroupRPMOverrideResolved)

	billing := &BillingCacheService{userRPMCache: apiKeyRPMCacheNoop{}, userGroupRateRepo: rpmRepo}
	require.NoError(t, billing.checkRPM(context.Background(), selected.User, selected.Group))
	require.Equal(t, []int64{secondaryGroupID}, rpmRepo.singleCalls)
	require.Equal(t, 1, rpmRepo.bulkCalls)
}

func TestAPIKeyWithSelectedGroupDeepCopiesMutableAuthState(t *testing.T) {
	defaultGroupID := int64(2)
	secondaryGroupID := int64(12)
	dailyLimit := 10.0
	fallbackGroupID := int64(20)
	balanceThreshold := 5.0
	override := 7
	expiresAt := time.Date(2030, time.January, 2, 3, 4, 5, 0, time.UTC)
	secondary := Group{
		ID:                   secondaryGroupID,
		DailyLimitUSD:        &dailyLimit,
		FallbackGroupID:      &fallbackGroupID,
		ModelRouting:         map[string][]int64{"grok-*": {1, 2}},
		SupportedModelScopes: []string{"claude"},
		MessagesDispatchModelConfig: OpenAIMessagesDispatchModelConfig{
			ExactModelMappings: map[string]string{"claude": "grok"},
		},
		ModelsListConfig: GroupModelsListConfig{Models: []string{"grok"}},
	}
	apiKey := &APIKey{
		GroupID:                   &defaultGroupID,
		Group:                     &Group{ID: defaultGroupID, ModelRouting: map[string][]int64{"default": {3}}},
		GroupIDs:                  []int64{defaultGroupID, secondaryGroupID},
		Groups:                    []Group{{ID: defaultGroupID}, secondary},
		GroupRPMOverrides:         map[int64]int{secondaryGroupID: override},
		GroupRPMOverridesResolved: true,
		IPWhitelist:               []string{"10.0.0.1"},
		IPBlacklist:               []string{"10.0.0.2"},
		ExpiresAt:                 &expiresAt,
		User: &User{
			AllowedGroups:            []int64{defaultGroupID, secondaryGroupID},
			BalanceNotifyThreshold:   &balanceThreshold,
			BalanceNotifyExtraEmails: []NotifyEmailEntry{{Email: "notify@example.com"}},
			UserGroupRPMOverride:     &override,
		},
	}

	selected, err := apiKey.WithSelectedGroup(secondaryGroupID)
	require.NoError(t, err)
	selected.IPWhitelist[0] = "mutated"
	selected.IPBlacklist[0] = "mutated"
	selected.GroupIDs[0] = 99
	selected.GroupRPMOverrides[secondaryGroupID] = 99
	selected.Groups[0].ID = 99
	selected.Group.ModelRouting["grok-*"][0] = 99
	selected.Group.SupportedModelScopes[0] = "mutated"
	selected.Group.MessagesDispatchModelConfig.ExactModelMappings["claude"] = "mutated"
	selected.Group.ModelsListConfig.Models[0] = "mutated"
	*selected.Group.DailyLimitUSD = 99
	*selected.Group.FallbackGroupID = 99
	selected.User.AllowedGroups[0] = 99
	selected.User.BalanceNotifyExtraEmails[0].Email = "mutated@example.com"
	*selected.User.BalanceNotifyThreshold = 99
	*selected.User.UserGroupRPMOverride = 99
	*selected.ExpiresAt = selected.ExpiresAt.Add(time.Hour)

	require.Equal(t, []string{"10.0.0.1"}, apiKey.IPWhitelist)
	require.Equal(t, []string{"10.0.0.2"}, apiKey.IPBlacklist)
	require.Equal(t, []int64{defaultGroupID, secondaryGroupID}, apiKey.GroupIDs)
	require.Equal(t, 7, apiKey.GroupRPMOverrides[secondaryGroupID])
	require.Equal(t, defaultGroupID, apiKey.Groups[0].ID)
	require.Equal(t, int64(1), apiKey.Groups[1].ModelRouting["grok-*"][0])
	require.Equal(t, "claude", apiKey.Groups[1].SupportedModelScopes[0])
	require.Equal(t, "grok", apiKey.Groups[1].MessagesDispatchModelConfig.ExactModelMappings["claude"])
	require.Equal(t, "grok", apiKey.Groups[1].ModelsListConfig.Models[0])
	require.Equal(t, 10.0, *apiKey.Groups[1].DailyLimitUSD)
	require.Equal(t, int64(20), *apiKey.Groups[1].FallbackGroupID)
	require.Equal(t, []int64{defaultGroupID, secondaryGroupID}, apiKey.User.AllowedGroups)
	require.Equal(t, "notify@example.com", apiKey.User.BalanceNotifyExtraEmails[0].Email)
	require.Equal(t, 5.0, *apiKey.User.BalanceNotifyThreshold)
	require.Equal(t, 7, *apiKey.User.UserGroupRPMOverride)
	require.Equal(t, expiresAt, *apiKey.ExpiresAt)
}
