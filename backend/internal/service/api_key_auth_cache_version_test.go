package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyService_MultiGroupAuthSnapshotRoundTrip(t *testing.T) {
	dailyLimit := 10.0
	weeklyLimit := 20.0
	monthlyLimit := 30.0
	imagePrice1K := 0.1
	imagePrice2K := 0.2
	imagePrice4K := 0.4
	videoPrice480P := 0.5
	videoPrice720P := 0.7
	videoPrice1080P := 1.0
	webSearchPrice := 0.01
	fallbackGroupID := int64(20)
	invalidRequestFallbackGroupID := int64(21)

	defaultGroup := Group{
		ID:               2,
		Name:             "Codex",
		Platform:         PlatformOpenAI,
		RateMultiplier:   1,
		Status:           StatusActive,
		Hydrated:         true,
		SubscriptionType: SubscriptionTypeStandard,
	}
	secondaryGroup := Group{
		ID:                              12,
		Name:                            "Grok",
		Platform:                        PlatformGrok,
		RateMultiplier:                  1.25,
		PeakRateEnabled:                 true,
		PeakStart:                       "08:00",
		PeakEnd:                         "10:00",
		PeakRateMultiplier:              1.5,
		IsExclusive:                     true,
		Status:                          StatusActive,
		Hydrated:                        true,
		SubscriptionType:                SubscriptionTypeStandard,
		DailyLimitUSD:                   &dailyLimit,
		WeeklyLimitUSD:                  &weeklyLimit,
		MonthlyLimitUSD:                 &monthlyLimit,
		AllowImageGeneration:            true,
		AllowBatchImageGeneration:       true,
		ImageRateIndependent:            true,
		ImageRateMultiplier:             1.1,
		ImagePrice1K:                    &imagePrice1K,
		ImagePrice2K:                    &imagePrice2K,
		ImagePrice4K:                    &imagePrice4K,
		VideoRateIndependent:            true,
		VideoRateMultiplier:             1.2,
		VideoPrice480P:                  &videoPrice480P,
		VideoPrice720P:                  &videoPrice720P,
		VideoPrice1080P:                 &videoPrice1080P,
		WebSearchPricePerCall:           &webSearchPrice,
		ClaudeCodeOnly:                  true,
		FallbackGroupID:                 &fallbackGroupID,
		FallbackGroupIDOnInvalidRequest: &invalidRequestFallbackGroupID,
		ModelRouting:                    map[string][]int64{"grok-*": {1, 2}},
		ModelRoutingEnabled:             true,
		MCPXMLInject:                    true,
		SupportedModelScopes:            []string{"claude", "gemini_text"},
		AllowMessagesDispatch:           true,
		DefaultMappedModel:              "grok-4.5",
		MessagesDispatchModelConfig: OpenAIMessagesDispatchModelConfig{
			OpusMappedModel: "grok-4.5",
			ExactModelMappings: map[string]string{
				"claude-opus-4-8": "grok-4.5",
			},
		},
		ModelsListConfig: GroupModelsListConfig{
			Enabled: true,
			Models:  []string{"grok-4.5"},
		},
		RPMLimit: 120,
	}
	defaultGroupID := defaultGroup.ID
	apiKey := &APIKey{
		ID:       1,
		UserID:   2,
		Key:      "k-multi-group-roundtrip",
		Name:     "Multi Group",
		GroupID:  &defaultGroupID,
		Group:    &defaultGroup,
		GroupIDs: []int64{defaultGroup.ID, secondaryGroup.ID},
		Groups:   []Group{defaultGroup, secondaryGroup},
		Status:   StatusActive,
		User: &User{
			ID:          2,
			Status:      StatusActive,
			Role:        RoleUser,
			Balance:     10,
			Concurrency: 3,
		},
	}

	svc := &APIKeyService{}
	snapshot := svc.snapshotFromAPIKey(context.Background(), apiKey)
	require.NotNil(t, snapshot)
	require.Equal(t, 16, snapshot.Version)

	roundTrip, used, err := svc.applyAuthCacheEntry(apiKey.Key, &APIKeyAuthCacheEntry{Snapshot: snapshot})
	require.NoError(t, err)
	require.True(t, used)
	require.NotNil(t, roundTrip)
	require.Equal(t, apiKey.GroupIDs, roundTrip.GroupIDs)
	require.Equal(t, apiKey.Groups, roundTrip.Groups)
	require.Equal(t, defaultGroup.ID, *roundTrip.GroupID)
	require.Equal(t, defaultGroup, *roundTrip.Group)

	selected, err := roundTrip.WithSelectedGroup(secondaryGroup.ID)
	require.NoError(t, err)
	require.Equal(t, secondaryGroup.ID, *selected.GroupID)
	require.Equal(t, secondaryGroup, *selected.Group)
	require.Equal(t, defaultGroup.ID, *roundTrip.GroupID)
	require.Equal(t, defaultGroup, *roundTrip.Group)
}

func TestAPIKeyAuthSnapshotWriteAndReadAreDeeplyIsolated(t *testing.T) {
	dailyLimit := 10.0
	fallbackGroupID := int64(20)
	balanceThreshold := 5.0
	override := 7
	groupID := int64(2)
	group := Group{
		ID:                   groupID,
		DailyLimitUSD:        &dailyLimit,
		FallbackGroupID:      &fallbackGroupID,
		ModelRouting:         map[string][]int64{"gpt-*": {1, 2}},
		SupportedModelScopes: []string{"claude"},
		MessagesDispatchModelConfig: OpenAIMessagesDispatchModelConfig{
			ExactModelMappings: map[string]string{"claude": "gpt"},
		},
		ModelsListConfig: GroupModelsListConfig{Models: []string{"gpt"}},
	}
	apiKey := &APIKey{
		ID:                1,
		UserID:            2,
		Key:               "k-snapshot-isolation",
		GroupID:           &groupID,
		GroupIDs:          []int64{groupID},
		Group:             &group,
		Groups:            []Group{group},
		GroupRPMOverrides: map[int64]int{groupID: override},
		IPWhitelist:       []string{"10.0.0.1"},
		IPBlacklist:       []string{"10.0.0.2"},
		User: &User{
			ID:                       2,
			AllowedGroups:            []int64{groupID},
			BalanceNotifyThreshold:   &balanceThreshold,
			BalanceNotifyExtraEmails: []NotifyEmailEntry{{Email: "notify@example.com"}},
			UserGroupRPMOverride:     &override,
		},
	}

	rpmRepo := &apiKeyRPMOverrideRepoStub{overrides: map[int64]*int{groupID: &override}}
	svc := &APIKeyService{userGroupRateRepo: rpmRepo}
	snapshot := svc.snapshotFromAPIKey(context.Background(), apiKey)
	require.NotNil(t, snapshot)
	require.True(t, snapshot.GroupRPMOverridesResolved)
	require.Equal(t, 1, rpmRepo.bulkCalls)
	require.Empty(t, rpmRepo.singleCalls)

	apiKey.IPWhitelist[0] = "source-mutated"
	apiKey.IPBlacklist[0] = "source-mutated"
	apiKey.GroupIDs[0] = 99
	apiKey.GroupRPMOverrides[groupID] = 99
	apiKey.User.AllowedGroups[0] = 99
	apiKey.User.BalanceNotifyExtraEmails[0].Email = "source-mutated@example.com"
	*apiKey.User.BalanceNotifyThreshold = 99
	apiKey.Group.ModelRouting["gpt-*"][0] = 99
	apiKey.Group.SupportedModelScopes[0] = "source-mutated"
	apiKey.Group.MessagesDispatchModelConfig.ExactModelMappings["claude"] = "source-mutated"
	apiKey.Group.ModelsListConfig.Models[0] = "source-mutated"
	*apiKey.Group.DailyLimitUSD = 99
	*apiKey.Group.FallbackGroupID = 99

	require.Equal(t, []string{"10.0.0.1"}, snapshot.IPWhitelist)
	require.Equal(t, []string{"10.0.0.2"}, snapshot.IPBlacklist)
	require.Equal(t, []int64{groupID}, snapshot.GroupIDs)
	require.Equal(t, 7, snapshot.GroupRPMOverrides[groupID])
	require.Equal(t, []int64{groupID}, snapshot.User.AllowedGroups)
	require.Equal(t, "notify@example.com", snapshot.User.BalanceNotifyExtraEmails[0].Email)
	require.Equal(t, 5.0, *snapshot.User.BalanceNotifyThreshold)
	require.Equal(t, int64(1), snapshot.Group.ModelRouting["gpt-*"][0])
	require.Equal(t, "claude", snapshot.Group.SupportedModelScopes[0])
	require.Equal(t, "gpt", snapshot.Group.MessagesDispatchModelConfig.ExactModelMappings["claude"])
	require.Equal(t, "gpt", snapshot.Group.ModelsListConfig.Models[0])
	require.Equal(t, 10.0, *snapshot.Group.DailyLimitUSD)
	require.Equal(t, int64(20), *snapshot.Group.FallbackGroupID)

	first := svc.snapshotToAPIKey(apiKey.Key, snapshot)
	first.IPWhitelist[0] = "first-mutated"
	first.IPBlacklist[0] = "first-mutated"
	first.GroupIDs[0] = 88
	first.GroupRPMOverrides[groupID] = 88
	first.User.AllowedGroups[0] = 88
	first.User.BalanceNotifyExtraEmails[0].Email = "first-mutated@example.com"
	*first.User.BalanceNotifyThreshold = 88
	first.Group.ModelRouting["gpt-*"][0] = 88
	first.Group.SupportedModelScopes[0] = "first-mutated"
	first.Group.MessagesDispatchModelConfig.ExactModelMappings["claude"] = "first-mutated"
	first.Group.ModelsListConfig.Models[0] = "first-mutated"
	*first.Group.DailyLimitUSD = 88
	*first.Group.FallbackGroupID = 88

	second := svc.snapshotToAPIKey(apiKey.Key, snapshot)
	require.Equal(t, []string{"10.0.0.1"}, snapshot.IPWhitelist)
	require.Equal(t, []string{"10.0.0.1"}, second.IPWhitelist)
	require.Equal(t, []string{"10.0.0.2"}, second.IPBlacklist)
	require.Equal(t, []int64{groupID}, second.GroupIDs)
	require.Equal(t, 7, snapshot.GroupRPMOverrides[groupID])
	require.Equal(t, 7, second.GroupRPMOverrides[groupID])
	require.True(t, second.GroupRPMOverridesResolved)
	require.True(t, second.User.UserGroupRPMOverrideResolved)
	require.Equal(t, []int64{groupID}, second.User.AllowedGroups)
	require.Equal(t, "notify@example.com", second.User.BalanceNotifyExtraEmails[0].Email)
	require.Equal(t, 5.0, *second.User.BalanceNotifyThreshold)
	require.Equal(t, int64(1), second.Group.ModelRouting["gpt-*"][0])
	require.Equal(t, "claude", second.Group.SupportedModelScopes[0])
	require.Equal(t, "gpt", second.Group.MessagesDispatchModelConfig.ExactModelMappings["claude"])
	require.Equal(t, "gpt", second.Group.ModelsListConfig.Models[0])
	require.Equal(t, 10.0, *second.Group.DailyLimitUSD)
	require.Equal(t, int64(20), *second.Group.FallbackGroupID)
}

func TestAPIKeyService_RejectsV15AuthSnapshotWithoutMultiGroupBindings(t *testing.T) {
	svc := &APIKeyService{}
	apiKey, used, err := svc.applyAuthCacheEntry("k-v15", &APIKeyAuthCacheEntry{
		Snapshot: &APIKeyAuthSnapshot{Version: 15},
	})
	require.NoError(t, err)
	require.False(t, used)
	require.Nil(t, apiKey)
}

func TestAPIKeyService_RejectsV10AuthSnapshotWithoutModelsListConfig(t *testing.T) {
	groupID := int64(9)
	svc := &APIKeyService{}

	apiKey, ok, err := svc.applyAuthCacheEntry("k-legacy-models-list", &APIKeyAuthCacheEntry{
		Snapshot: &APIKeyAuthSnapshot{
			Version:  10,
			APIKeyID: 1,
			UserID:   2,
			GroupID:  &groupID,
			Status:   StatusActive,
			User: APIKeyAuthUserSnapshot{
				ID:          2,
				Status:      StatusActive,
				Role:        RoleUser,
				Balance:     10,
				Concurrency: 3,
			},
			Group: &APIKeyAuthGroupSnapshot{
				ID:               groupID,
				Name:             "openai",
				Platform:         PlatformOpenAI,
				Status:           StatusActive,
				SubscriptionType: SubscriptionTypeStandard,
				RateMultiplier:   1,
			},
		},
	})

	if err != nil {
		t.Fatalf("expected stale snapshot to be ignored without error, got %v", err)
	}
	if ok {
		t.Fatalf("expected v10 auth snapshot to be rejected after models_list_config was added")
	}
	if apiKey != nil {
		t.Fatalf("expected no API key from stale snapshot, got %#v", apiKey)
	}
}
