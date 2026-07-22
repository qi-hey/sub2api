package handler

import (
	"context"
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

var (
	errOpenAIGPT54FallbackNotEligible     = errors.New("gpt-5.4 grok fallback is not eligible")
	errOpenAIGPT54FallbackOutputStarted   = errors.New("gpt-5.4 grok fallback cannot switch after output")
	errOpenAIGPT54FallbackAlreadySwitched = errors.New("gpt-5.4 grok fallback already switched")
	errOpenAIGPT54FallbackSubscription    = errors.New("gpt-5.4 openai fallback subscription is unavailable")
)

type openAIFallbackSubscriptionStore interface {
	GetActiveSubscription(ctx context.Context, userID, groupID int64) (*service.UserSubscription, error)
}

const (
	openAIGPT54PrimaryRoute = iota
	openAIGPT54FallbackRoute
)

// openAIGPT54Route owns request-local state for the one-way Grok-to-OpenAI
// transition. The authenticated API key remains immutable.
type openAIGPT54Route struct {
	sourceAPIKey        *service.APIKey
	currentAPIKey       *service.APIKey
	currentSubscription *service.UserSubscription
	requestedModel      string
	eligible            bool
	currentRoute        int
	didSwitch           bool
	reason              string
	failedByRoute       [2]map[int64]struct{}
	retriesByRoute      [2]map[int64]int
}

func newOpenAIGPT54Route(apiKey *service.APIKey, subscription *service.UserSubscription, requestedModel string) *openAIGPT54Route {
	platform := ""
	if apiKey != nil && apiKey.Group != nil {
		platform = apiKey.Group.Platform
	}
	return &openAIGPT54Route{
		sourceAPIKey:        apiKey,
		currentAPIKey:       apiKey,
		currentSubscription: subscription,
		requestedModel:      strings.ToLower(strings.TrimSpace(requestedModel)),
		eligible:            strings.EqualFold(strings.TrimSpace(requestedModel), "gpt-5.4") && platform == service.PlatformGrok,
		failedByRoute: [2]map[int64]struct{}{
			openAIGPT54PrimaryRoute:  make(map[int64]struct{}),
			openAIGPT54FallbackRoute: make(map[int64]struct{}),
		},
		retriesByRoute: [2]map[int64]int{
			openAIGPT54PrimaryRoute:  make(map[int64]int),
			openAIGPT54FallbackRoute: make(map[int64]int),
		},
	}
}

func (r *openAIGPT54Route) canFallback() bool {
	return r != nil && r.eligible && !r.didSwitch
}

func (r *openAIGPT54Route) apiKey() *service.APIKey {
	if r == nil {
		return nil
	}
	return r.currentAPIKey
}

func (r *openAIGPT54Route) subscription() *service.UserSubscription {
	if r == nil {
		return nil
	}
	return r.currentSubscription
}

func (r *openAIGPT54Route) setSubscription(subscription *service.UserSubscription) {
	if r != nil {
		r.currentSubscription = subscription
	}
}

func (r *openAIGPT54Route) platform() string {
	if r == nil || r.currentAPIKey == nil || r.currentAPIKey.Group == nil {
		return service.PlatformOpenAI
	}
	return openAICompatibleRequestPlatform(r.currentAPIKey)
}

func (r *openAIGPT54Route) switched() bool {
	return r != nil && r.didSwitch
}

func (r *openAIGPT54Route) fallbackReason() string {
	if r == nil {
		return ""
	}
	return r.reason
}

func (r *openAIGPT54Route) failedAccountIDs() map[int64]struct{} {
	if r == nil {
		return nil
	}
	return r.failedByRoute[r.currentRoute]
}

func (r *openAIGPT54Route) sameAccountRetries() map[int64]int {
	if r == nil {
		return nil
	}
	return r.retriesByRoute[r.currentRoute]
}

func (r *openAIGPT54Route) primaryFailedAccountIDs() map[int64]struct{} {
	if r == nil {
		return nil
	}
	return r.failedByRoute[openAIGPT54PrimaryRoute]
}

func (r *openAIGPT54Route) primarySameAccountRetries() map[int64]int {
	if r == nil {
		return nil
	}
	return r.retriesByRoute[openAIGPT54PrimaryRoute]
}

func (r *openAIGPT54Route) switchToOpenAI(reason string, outputStarted bool) error {
	if r == nil || !r.eligible {
		return errOpenAIGPT54FallbackNotEligible
	}
	if r.didSwitch {
		return errOpenAIGPT54FallbackAlreadySwitched
	}
	if outputStarted {
		return errOpenAIGPT54FallbackOutputStarted
	}

	fallbackAPIKey, err := service.ResolveAPIKeyRequestPlatform(r.sourceAPIKey, service.PlatformOpenAI)
	if err != nil {
		return err
	}
	r.currentAPIKey = fallbackAPIKey
	r.currentSubscription = nil
	r.currentRoute = openAIGPT54FallbackRoute
	r.didSwitch = true
	r.reason = strings.TrimSpace(reason)
	return nil
}

func (h *OpenAIGatewayHandler) switchOpenAIGPT54Route(c *gin.Context, route *openAIGPT54Route, reason string, outputStarted bool) error {
	if route == nil || !route.canFallback() {
		return errOpenAIGPT54FallbackNotEligible
	}
	if outputStarted {
		return errOpenAIGPT54FallbackOutputStarted
	}

	fallbackAPIKey, err := service.ResolveAPIKeyRequestPlatform(route.sourceAPIKey, service.PlatformOpenAI)
	if err != nil {
		return err
	}
	subscription, err := h.loadOpenAIFallbackSubscription(c.Request.Context(), fallbackAPIKey)
	if err != nil {
		return err
	}
	if h != nil && h.billingCacheService != nil {
		if err := h.billingCacheService.CheckBillingEligibilityForRouteSwitch(
			c.Request.Context(),
			fallbackAPIKey.User,
			fallbackAPIKey,
			fallbackAPIKey.Group,
			subscription,
			service.QuotaPlatform(c.Request.Context(), fallbackAPIKey),
		); err != nil {
			return err
		}
	}
	if err := route.switchToOpenAI(reason, false); err != nil {
		return err
	}
	route.setSubscription(subscription)
	applyOpenAIGPT54RouteToContext(c, route)
	return nil
}

func (h *OpenAIGatewayHandler) restoreOpenAIGPT54RouteContinuity(
	c *gin.Context,
	route *openAIGPT54Route,
	sessionHash string,
	previousResponseID string,
	requiredCapability service.OpenAIEndpointCapability,
	requireCompact bool,
) (bool, error) {
	if h == nil || h.gatewayService == nil || c == nil || c.Request == nil || route == nil || !route.canFallback() {
		return false, nil
	}
	fallbackAPIKey, err := service.ResolveAPIKeyRequestPlatform(route.sourceAPIKey, service.PlatformOpenAI)
	if err != nil {
		return false, nil
	}
	if !h.gatewayService.HasOpenAIRouteContinuity(
		c.Request.Context(),
		fallbackAPIKey.GroupID,
		sessionHash,
		previousResponseID,
		route.requestedModel,
		requiredCapability,
		requireCompact,
	) {
		return false, nil
	}
	subscription, err := h.loadOpenAIFallbackSubscription(c.Request.Context(), fallbackAPIKey)
	if err != nil {
		return false, err
	}
	if err := route.switchToOpenAI("openai_route_continuity", false); err != nil {
		return false, err
	}
	route.setSubscription(subscription)
	applyOpenAIGPT54RouteToContext(c, route)
	return true, nil
}

func (h *OpenAIGatewayHandler) loadOpenAIFallbackSubscription(ctx context.Context, apiKey *service.APIKey) (*service.UserSubscription, error) {
	if apiKey == nil || apiKey.Group == nil || !apiKey.Group.IsSubscriptionType() {
		return nil, nil
	}
	if h == nil || h.fallbackSubscriptionStore == nil || apiKey.User == nil {
		return nil, errOpenAIGPT54FallbackSubscription
	}
	subscription, err := h.fallbackSubscriptionStore.GetActiveSubscription(ctx, apiKey.User.ID, apiKey.Group.ID)
	if err != nil {
		return nil, err
	}
	if subscription == nil {
		return nil, errOpenAIGPT54FallbackSubscription
	}
	return subscription, nil
}

func applyOpenAIGPT54RouteToContext(c *gin.Context, route *openAIGPT54Route) {
	if c == nil || c.Request == nil || route == nil || route.apiKey() == nil {
		return
	}
	apiKey := route.apiKey()
	c.Set(string(middleware2.ContextKeyAPIKey), apiKey)
	middleware2.SetOpsFallbackAPIKey(c, apiKey)
	c.Set(string(middleware2.ContextKeySubscription), route.subscription())
	if apiKey.Group != nil {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.Group, apiKey.Group))
	}
}
