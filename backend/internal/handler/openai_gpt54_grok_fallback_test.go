package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGPT54GrokFirstRouteEligibility(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		model    string
		platform string
		want     bool
	}{
		{name: "exact model", model: "gpt-5.4", platform: service.PlatformGrok, want: true},
		{name: "normalized exact model", model: " GPT-5.4 ", platform: service.PlatformGrok, want: true},
		{name: "openai route is not primary", model: "gpt-5.4", platform: service.PlatformOpenAI, want: false},
		{name: "other gpt model", model: "gpt-5.5", platform: service.PlatformGrok, want: false},
		{name: "grok model", model: "grok-4.5", platform: service.PlatformGrok, want: false},
		{name: "model suffix", model: "gpt-5.4-mini", platform: service.PlatformGrok, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			key := gpt54RouteTestAPIKey(tt.platform, false)
			route := newOpenAIGPT54Route(key, nil, tt.model)
			require.Equal(t, tt.want, route.canFallback())
			require.Same(t, key, route.apiKey())
		})
	}
}

func TestGPT54GrokFirstRouteSwitchesToBoundOpenAIGroup(t *testing.T) {
	t.Parallel()

	key := gpt54RouteTestAPIKey(service.PlatformGrok, false)
	originalGroupID := *key.GroupID
	originalGroup := key.Group
	route := newOpenAIGPT54Route(key, nil, "gpt-5.4")

	route.failedAccountIDs()[101] = struct{}{}
	route.sameAccountRetries()[101] = 2

	err := route.switchToOpenAI("grok_accounts_exhausted", false)
	require.NoError(t, err)
	require.True(t, route.switched())
	require.Equal(t, service.PlatformOpenAI, route.platform())
	require.NotNil(t, route.apiKey().GroupID)
	require.Equal(t, int64(12), *route.apiKey().GroupID)
	require.Equal(t, "grok_accounts_exhausted", route.fallbackReason())
	require.Empty(t, route.failedAccountIDs())
	require.Empty(t, route.sameAccountRetries())

	// The authenticated/cached key and primary attempt state remain untouched.
	require.Equal(t, originalGroupID, *key.GroupID)
	require.Same(t, originalGroup, key.Group)
	require.Equal(t, service.PlatformGrok, key.Group.Platform)
	require.Contains(t, route.primaryFailedAccountIDs(), int64(101))
	require.Equal(t, 2, route.primarySameAccountRetries()[101])
}

func TestGPT54GrokFirstRouteRejectsUnsafeOrInvalidSwitch(t *testing.T) {
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
			key:         gpt54RouteTestAPIKey(service.PlatformGrok, false),
			model:       "gpt-5.4",
			outputBegun: true,
			wantErr:     errOpenAIGPT54FallbackOutputStarted,
		},
		{
			name:    "openai group unbound",
			key:     gpt54RouteTestAPIKey(service.PlatformGrok, false, service.PlatformGrok),
			model:   "gpt-5.4",
			wantErr: service.ErrAPIKeyGroupNotBound,
		},
		{
			name:    "openai group ambiguous",
			key:     gpt54RouteTestAPIKey(service.PlatformGrok, true),
			model:   "gpt-5.4",
			wantErr: service.ErrAPIKeyGroupAmbiguous,
		},
		{
			name:    "other model",
			key:     gpt54RouteTestAPIKey(service.PlatformGrok, false),
			model:   "gpt-5.5",
			wantErr: errOpenAIGPT54FallbackNotEligible,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			route := newOpenAIGPT54Route(tt.key, nil, tt.model)
			err := route.switchToOpenAI("test", tt.outputBegun)
			require.ErrorIs(t, err, tt.wantErr)
			require.False(t, route.switched())
		})
	}
}

func TestGPT54GrokFirstRouteCannotSwitchTwice(t *testing.T) {
	t.Parallel()

	route := newOpenAIGPT54Route(gpt54RouteTestAPIKey(service.PlatformGrok, false), nil, "gpt-5.4")
	require.NoError(t, route.switchToOpenAI("first", false))
	require.ErrorIs(t, route.switchToOpenAI("second", false), errOpenAIGPT54FallbackAlreadySwitched)
	require.Equal(t, "first", route.fallbackReason())
}

func gpt54RouteTestAPIKey(primaryPlatform string, ambiguousOpenAI bool, onlyPlatforms ...string) *service.APIKey {
	primaryID := int64(11)
	primary := service.Group{ID: primaryID, Name: "primary", Platform: primaryPlatform, Status: service.StatusActive}
	groups := []service.Group{primary}

	platforms := onlyPlatforms
	if len(platforms) == 0 {
		platforms = []string{service.PlatformOpenAI}
	}
	for _, platform := range platforms {
		if platform == primaryPlatform {
			continue
		}
		groups = append(groups, service.Group{ID: 12, Name: "fallback", Platform: platform, Status: service.StatusActive})
	}
	if ambiguousOpenAI {
		groups = append(groups,
			service.Group{ID: 12, Name: "fallback-a", Platform: service.PlatformOpenAI, Status: service.StatusActive},
			service.Group{ID: 13, Name: "fallback-b", Platform: service.PlatformOpenAI, Status: service.StatusActive},
		)
	}

	return &service.APIKey{
		ID:       99,
		UserID:   7,
		GroupID:  &primaryID,
		Group:    &primary,
		GroupIDs: groupIDsForGPT54RouteTest(groups),
		Groups:   groups,
		User:     &service.User{ID: 7, Status: service.StatusActive},
	}
}

func groupIDsForGPT54RouteTest(groups []service.Group) []int64 {
	ids := make([]int64, 0, len(groups))
	for i := range groups {
		ids = append(ids, groups[i].ID)
	}
	return ids
}
