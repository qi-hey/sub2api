package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIGatewayHasRouteContinuitySessionSticky(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	groupID := int64(12)
	sessionHash := "fallback-session"
	account := Account{
		ID:            101,
		Platform:      PlatformOpenAI,
		Type:          AccountTypeAPIKey,
		Status:        StatusActive,
		Schedulable:   true,
		Concurrency:   1,
		AccountGroups: []AccountGroup{{GroupID: groupID}},
	}
	cache := &stubGatewayCache{sessionBindings: map[string]int64{"openai:" + sessionHash: account.ID}}
	svc := &OpenAIGatewayService{
		accountRepo: groupAwareStubOpenAIAccountRepo{stubOpenAIAccountRepo{accounts: []Account{account}}},
		cache:       cache,
	}

	require.True(t, svc.HasOpenAIRouteContinuity(
		ctx,
		&groupID,
		sessionHash,
		"",
		"gpt-5.4",
		OpenAIEndpointCapabilityChatCompletions,
		false,
	))
}

func TestOpenAIGatewayHasRouteContinuityPreviousResponse(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	groupID := int64(12)
	account := Account{
		ID:            102,
		Platform:      PlatformOpenAI,
		Type:          AccountTypeAPIKey,
		Status:        StatusActive,
		Schedulable:   true,
		Concurrency:   1,
		AccountGroups: []AccountGroup{{GroupID: groupID}},
		Extra: map[string]any{
			"openai_apikey_responses_websockets_v2_enabled": true,
		},
	}
	cache := &stubGatewayCache{}
	store := NewOpenAIWSStateStore(cache)
	svc := &OpenAIGatewayService{
		accountRepo:        groupAwareStubOpenAIAccountRepo{stubOpenAIAccountRepo{accounts: []Account{account}}},
		cache:              cache,
		openaiWSStateStore: store,
		cfg:                newOpenAIWSV2TestConfig(),
	}
	require.NoError(t, store.BindResponseAccount(ctx, groupID, "resp_fallback", account.ID, time.Hour))

	require.True(t, svc.HasOpenAIRouteContinuity(
		ctx,
		&groupID,
		"",
		"resp_fallback",
		"gpt-5.4",
		OpenAIEndpointCapabilityChatCompletions,
		false,
	))
}

func TestOpenAIGatewayHasRouteContinuityRejectsStaleOrCrossGroupBindings(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	requestedGroupID := int64(12)
	otherGroupID := int64(13)
	tests := []struct {
		name    string
		account Account
	}{
		{
			name: "disabled",
			account: Account{
				ID: 201, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
				Status: StatusDisabled, Schedulable: false,
				AccountGroups: []AccountGroup{{GroupID: requestedGroupID}},
			},
		},
		{
			name: "cross group",
			account: Account{
				ID: 202, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
				Status: StatusActive, Schedulable: true,
				AccountGroups: []AccountGroup{{GroupID: otherGroupID}},
			},
		},
		{
			name: "wrong platform",
			account: Account{
				ID: 203, Platform: PlatformGrok, Type: AccountTypeAPIKey,
				Status: StatusActive, Schedulable: true,
				AccountGroups: []AccountGroup{{GroupID: requestedGroupID}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sessionHash := "stale-" + tt.name
			cache := &stubGatewayCache{sessionBindings: map[string]int64{"openai:" + sessionHash: tt.account.ID}}
			svc := &OpenAIGatewayService{
				accountRepo: groupAwareStubOpenAIAccountRepo{stubOpenAIAccountRepo{accounts: []Account{tt.account}}},
				cache:       cache,
			}

			require.False(t, svc.HasOpenAIRouteContinuity(
				ctx,
				&requestedGroupID,
				sessionHash,
				"",
				"gpt-5.4",
				OpenAIEndpointCapabilityChatCompletions,
				false,
			))
		})
	}
}

func TestOpenAIGatewayHasRouteContinuityEmptySignals(t *testing.T) {
	t.Parallel()

	svc := &OpenAIGatewayService{}
	groupID := int64(12)
	require.False(t, svc.HasOpenAIRouteContinuity(
		context.Background(),
		&groupID,
		"",
		"",
		"gpt-5.4",
		OpenAIEndpointCapabilityChatCompletions,
		false,
	))
}
