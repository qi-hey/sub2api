package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIToGrokFallbackRouteEligibility(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		model    string
		platform string
		want     bool
	}{
		{name: "gpt 5.2", model: "gpt-5.2", platform: service.PlatformOpenAI, want: true},
		{name: "normalized gpt 5.4", model: " GPT-5.4 ", platform: service.PlatformOpenAI, want: true},
		{name: "gpt 5.4 mini", model: "gpt-5.4-mini", platform: service.PlatformOpenAI, want: true},
		{name: "gpt 5.5", model: "gpt-5.5", platform: service.PlatformOpenAI, want: true},
		{name: "gpt 5.6 luna", model: "gpt-5.6-luna", platform: service.PlatformOpenAI, want: true},
		{name: "gpt 5.6 sol", model: "gpt-5.6-sol", platform: service.PlatformOpenAI, want: true},
		{name: "gpt 5.6 terra", model: "gpt-5.6-terra", platform: service.PlatformOpenAI, want: true},
		{name: "grok route is not primary", model: "gpt-5.4", platform: service.PlatformGrok, want: false},
		{name: "other gpt model", model: "gpt-5.3", platform: service.PlatformOpenAI, want: false},
		{name: "grok model", model: "grok-4.5", platform: service.PlatformOpenAI, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			key := openAIToGrokRouteTestAPIKey(tt.platform, false)
			route := newOpenAIToGrokFallbackRoute(key, nil, tt.model)
			require.Equal(t, tt.want, route.canFallback())
			require.Same(t, key, route.apiKey())
		})
	}
}

func TestOpenAIToGrokFallbackRouteSwitchesToBoundGrokGroup(t *testing.T) {
	t.Parallel()

	key := openAIToGrokRouteTestAPIKey(service.PlatformOpenAI, false)
	originalGroupID := *key.GroupID
	originalGroup := key.Group
	route := newOpenAIToGrokFallbackRoute(key, nil, "gpt-5.4")

	route.failedAccountIDs()[101] = struct{}{}
	route.sameAccountRetries()[101] = 2

	err := route.switchToGrok("openai_accounts_exhausted", false)
	require.NoError(t, err)
	require.True(t, route.switched())
	require.Equal(t, service.PlatformGrok, route.platform())
	require.NotNil(t, route.apiKey().GroupID)
	require.Equal(t, int64(12), *route.apiKey().GroupID)
	require.Equal(t, "openai_accounts_exhausted", route.fallbackReason())
	require.Empty(t, route.failedAccountIDs())
	require.Empty(t, route.sameAccountRetries())

	// The authenticated/cached key and primary attempt state remain untouched.
	require.Equal(t, originalGroupID, *key.GroupID)
	require.Same(t, originalGroup, key.Group)
	require.Equal(t, service.PlatformOpenAI, key.Group.Platform)
	require.Contains(t, route.primaryFailedAccountIDs(), int64(101))
	require.Equal(t, 2, route.primarySameAccountRetries()[101])
}

func TestOpenAIToGrokFallbackRouteRejectsUnsafeOrInvalidSwitch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		key         *service.APIKey
		model       string
		outputBegun bool
		wantErr     error
	}{
		{
			name:        "output already started",
			key:         openAIToGrokRouteTestAPIKey(service.PlatformOpenAI, false),
			model:       "gpt-5.4",
			outputBegun: true,
			wantErr:     errOpenAIToGrokFallbackOutputStarted,
		},
		{
			name:    "grok group unbound",
			key:     openAIToGrokRouteTestAPIKey(service.PlatformOpenAI, false, service.PlatformOpenAI),
			model:   "gpt-5.4",
			wantErr: errOpenAIToGrokFallbackNotEligible,
		},
		{
			name:    "grok group ambiguous",
			key:     openAIToGrokRouteTestAPIKey(service.PlatformOpenAI, true),
			model:   "gpt-5.4",
			wantErr: errOpenAIToGrokFallbackNotEligible,
		},
		{
			name:    "other model",
			key:     openAIToGrokRouteTestAPIKey(service.PlatformOpenAI, false),
			model:   "gpt-5.3",
			wantErr: errOpenAIToGrokFallbackNotEligible,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			route := newOpenAIToGrokFallbackRoute(tt.key, nil, tt.model)
			err := route.switchToGrok("test", tt.outputBegun)
			require.ErrorIs(t, err, tt.wantErr)
			require.False(t, route.switched())
		})
	}
}

func TestOpenAIToGrokFallbackRouteCannotSwitchTwice(t *testing.T) {
	t.Parallel()

	route := newOpenAIToGrokFallbackRoute(openAIToGrokRouteTestAPIKey(service.PlatformOpenAI, false), nil, "gpt-5.4")
	require.NoError(t, route.switchToGrok("first", false))
	require.ErrorIs(t, route.switchToGrok("second", false), errOpenAIToGrokFallbackAlreadySwitched)
	require.Equal(t, "first", route.fallbackReason())
}

func TestOpenAIToGrokFallbackExhaustsAuthenticationFailuresOnly(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		require.True(t, shouldExhaustOpenAIBeforeGrokFallback(&service.UpstreamFailoverError{StatusCode: status}))
	}
	for _, status := range []int{http.StatusBadRequest, http.StatusNotFound, http.StatusTooManyRequests, http.StatusInternalServerError} {
		require.False(t, shouldExhaustOpenAIBeforeGrokFallback(&service.UpstreamFailoverError{StatusCode: status}))
	}
	require.False(t, shouldExhaustOpenAIBeforeGrokFallback(nil))
}

func TestOpenAIToGrokFallbackRouteHandlerSwitchLoadsDestinationSubscriptionAndUpdatesContext(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	key := openAIToGrokRouteTestAPIKey(service.PlatformOpenAI, false)
	for i := range key.Groups {
		if key.Groups[i].Platform == service.PlatformGrok {
			key.Groups[i].SubscriptionType = service.SubscriptionTypeSubscription
		}
	}
	subscription := &service.UserSubscription{
		ID:        501,
		UserID:    key.User.ID,
		GroupID:   12,
		Status:    service.SubscriptionStatusActive,
		StartsAt:  time.Now().Add(-time.Hour),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	resolver := &openAIFallbackSubscriptionResolverStub{subscription: subscription}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	h := &OpenAIGatewayHandler{
		billingCacheService:       billing,
		fallbackSubscriptionStore: resolver,
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), key)
	route := newOpenAIToGrokFallbackRoute(key, nil, "gpt-5.4")

	err := h.switchOpenAIToGrokFallbackRoute(c, route, "openai_accounts_exhausted", false)
	require.NoError(t, err)
	require.True(t, route.switched())
	require.Same(t, subscription, route.subscription())
	require.Equal(t, []openAIFallbackSubscriptionCall{{userID: key.User.ID, groupID: 12}}, resolver.calls)

	contextKey, ok := middleware2.GetAPIKeyFromContext(c)
	require.True(t, ok)
	require.Same(t, route.apiKey(), contextKey)
	require.Equal(t, int64(12), *contextKey.GroupID)
	contextGroup, ok := c.Request.Context().Value(ctxkey.Group).(*service.Group)
	require.True(t, ok)
	require.Equal(t, int64(12), contextGroup.ID)
}

func TestOpenAIToGrokFallbackRouteHandlerSubscriptionFailureDoesNotSwitch(t *testing.T) {
	t.Parallel()

	key := openAIToGrokRouteTestAPIKey(service.PlatformOpenAI, false)
	for i := range key.Groups {
		if key.Groups[i].Platform == service.PlatformGrok {
			key.Groups[i].SubscriptionType = service.SubscriptionTypeSubscription
		}
	}
	resolverErr := context.DeadlineExceeded
	resolver := &openAIFallbackSubscriptionResolverStub{err: resolverErr}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	h := &OpenAIGatewayHandler{
		billingCacheService:       billing,
		fallbackSubscriptionStore: resolver,
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	route := newOpenAIToGrokFallbackRoute(key, nil, "gpt-5.4")

	err := h.switchOpenAIToGrokFallbackRoute(c, route, "openai_accounts_exhausted", false)
	require.ErrorIs(t, err, resolverErr)
	require.False(t, route.switched())
	require.Same(t, key, route.apiKey())
}

type openAIFallbackSubscriptionCall struct {
	userID  int64
	groupID int64
}

type openAIFallbackSubscriptionResolverStub struct {
	subscription *service.UserSubscription
	err          error
	calls        []openAIFallbackSubscriptionCall
}

func (s *openAIFallbackSubscriptionResolverStub) GetActiveSubscription(_ context.Context, userID, groupID int64) (*service.UserSubscription, error) {
	s.calls = append(s.calls, openAIFallbackSubscriptionCall{userID: userID, groupID: groupID})
	return s.subscription, s.err
}

func openAIToGrokRouteTestAPIKey(primaryPlatform string, ambiguousGrok bool, onlyPlatforms ...string) *service.APIKey {
	primaryID := int64(11)
	primary := service.Group{
		ID:                    primaryID,
		Name:                  "primary",
		Platform:              primaryPlatform,
		Status:                service.StatusActive,
		AllowMessagesDispatch: primaryPlatform == service.PlatformOpenAI,
	}
	groups := []service.Group{primary}

	platforms := onlyPlatforms
	if len(platforms) == 0 && !ambiguousGrok {
		platforms = []string{service.PlatformGrok}
	}
	for _, platform := range platforms {
		if platform == primaryPlatform {
			continue
		}
		groups = append(groups, service.Group{ID: 12, Name: "fallback", Platform: platform, Status: service.StatusActive, AllowMessagesDispatch: platform == service.PlatformOpenAI})
	}
	if ambiguousGrok {
		groups = append(groups,
			service.Group{ID: 12, Name: "fallback-a", Platform: service.PlatformGrok, Status: service.StatusActive},
			service.Group{ID: 13, Name: "fallback-b", Platform: service.PlatformGrok, Status: service.StatusActive},
		)
	}

	return &service.APIKey{
		ID:       99,
		UserID:   7,
		GroupID:  &primaryID,
		Group:    &primary,
		GroupIDs: groupIDsForOpenAIToGrokRouteTest(groups),
		Groups:   groups,
		User:     &service.User{ID: 7, Status: service.StatusActive},
	}
}

func groupIDsForOpenAIToGrokRouteTest(groups []service.Group) []int64 {
	ids := make([]int64, 0, len(groups))
	for i := range groups {
		ids = append(ids, groups[i].ID)
	}
	return ids
}
