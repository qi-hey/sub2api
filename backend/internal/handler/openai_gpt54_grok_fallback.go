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
	errOpenAIToGrokFallbackNotEligible     = errors.New("openai to grok fallback is not eligible")
	errOpenAIToGrokFallbackOutputStarted   = errors.New("openai to grok fallback cannot switch after output")
	errOpenAIToGrokFallbackAlreadySwitched = errors.New("openai to grok fallback already switched")
	errOpenAIToGrokFallbackSubscription    = errors.New("openai to grok fallback subscription is unavailable")
)

type openAIFallbackSubscriptionStore interface {
	GetActiveSubscription(ctx context.Context, userID, groupID int64) (*service.UserSubscription, error)
}

const (
	openAIToGrokPrimaryRouteIndex = iota
	openAIToGrokFallbackRouteIndex
)

// openAIToGrokFallbackRoute owns request-local state for the one-way OpenAI-to-Grok
// transition. The authenticated API key remains immutable.
type openAIToGrokFallbackRoute struct {
	sourceAPIKey        *service.APIKey
	fallbackAPIKey      *service.APIKey
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

func newOpenAIToGrokFallbackRoute(apiKey *service.APIKey, subscription *service.UserSubscription, requestedModel string) *openAIToGrokFallbackRoute {
	platform := ""
	if apiKey != nil && apiKey.Group != nil {
		platform = apiKey.Group.Platform
	}
	var fallbackAPIKey *service.APIKey
	eligible := service.IsOpenAIToGrokFallbackModel(requestedModel) && platform == service.PlatformOpenAI
	if eligible {
		resolved, err := service.ResolveAPIKeyRequestPlatform(apiKey, service.PlatformGrok)
		if err != nil {
			eligible = false
		} else {
			fallbackAPIKey = resolved
		}
	}
	return &openAIToGrokFallbackRoute{
		sourceAPIKey:        apiKey,
		fallbackAPIKey:      fallbackAPIKey,
		currentAPIKey:       apiKey,
		currentSubscription: subscription,
		requestedModel:      strings.ToLower(strings.TrimSpace(requestedModel)),
		eligible:            eligible,
		failedByRoute: [2]map[int64]struct{}{
			openAIToGrokPrimaryRouteIndex:  make(map[int64]struct{}),
			openAIToGrokFallbackRouteIndex: make(map[int64]struct{}),
		},
		retriesByRoute: [2]map[int64]int{
			openAIToGrokPrimaryRouteIndex:  make(map[int64]int),
			openAIToGrokFallbackRouteIndex: make(map[int64]int),
		},
	}
}

func markOpenAIToGrokFallbackEligibility(c *gin.Context, route *openAIToGrokFallbackRoute) {
	service.SetOpenAIToGrokFallbackEligible(c, route != nil && route.canFallback())
}

func (r *openAIToGrokFallbackRoute) canFallback() bool {
	return r != nil && r.eligible && !r.didSwitch
}

func (r *openAIToGrokFallbackRoute) apiKey() *service.APIKey {
	if r == nil {
		return nil
	}
	return r.currentAPIKey
}

func (r *openAIToGrokFallbackRoute) subscription() *service.UserSubscription {
	if r == nil {
		return nil
	}
	return r.currentSubscription
}

func (r *openAIToGrokFallbackRoute) setSubscription(subscription *service.UserSubscription) {
	if r != nil {
		r.currentSubscription = subscription
	}
}

func (r *openAIToGrokFallbackRoute) platform() string {
	if r == nil || r.currentAPIKey == nil || r.currentAPIKey.Group == nil {
		return service.PlatformOpenAI
	}
	return openAICompatibleRequestPlatform(r.currentAPIKey)
}

func (r *openAIToGrokFallbackRoute) switched() bool {
	return r != nil && r.didSwitch
}

func (r *openAIToGrokFallbackRoute) fallbackReason() string {
	if r == nil {
		return ""
	}
	return r.reason
}

func (r *openAIToGrokFallbackRoute) failedAccountIDs() map[int64]struct{} {
	if r == nil {
		return nil
	}
	return r.failedByRoute[r.currentRoute]
}

func (r *openAIToGrokFallbackRoute) sameAccountRetries() map[int64]int {
	if r == nil {
		return nil
	}
	return r.retriesByRoute[r.currentRoute]
}

func (r *openAIToGrokFallbackRoute) primaryFailedAccountIDs() map[int64]struct{} {
	if r == nil {
		return nil
	}
	return r.failedByRoute[openAIToGrokPrimaryRouteIndex]
}

func (r *openAIToGrokFallbackRoute) primarySameAccountRetries() map[int64]int {
	if r == nil {
		return nil
	}
	return r.retriesByRoute[openAIToGrokPrimaryRouteIndex]
}

func shouldExhaustOpenAIBeforeGrokFallback(failoverErr *service.UpstreamFailoverError) bool {
	if failoverErr == nil {
		return false
	}
	return failoverErr.StatusCode == 401 || failoverErr.StatusCode == 403
}

func (r *openAIToGrokFallbackRoute) switchToGrok(reason string, outputStarted bool) error {
	if r == nil || !r.eligible {
		return errOpenAIToGrokFallbackNotEligible
	}
	if r.didSwitch {
		return errOpenAIToGrokFallbackAlreadySwitched
	}
	if outputStarted {
		return errOpenAIToGrokFallbackOutputStarted
	}

	if r.fallbackAPIKey == nil {
		return errOpenAIToGrokFallbackNotEligible
	}
	r.currentAPIKey = r.fallbackAPIKey
	r.currentSubscription = nil
	r.currentRoute = openAIToGrokFallbackRouteIndex
	r.didSwitch = true
	r.reason = strings.TrimSpace(reason)
	return nil
}

func (h *OpenAIGatewayHandler) switchOpenAIToGrokFallbackRoute(c *gin.Context, route *openAIToGrokFallbackRoute, reason string, outputStarted bool) error {
	if route == nil || !route.canFallback() {
		return errOpenAIToGrokFallbackNotEligible
	}
	if outputStarted {
		return errOpenAIToGrokFallbackOutputStarted
	}

	fallbackAPIKey := route.fallbackAPIKey
	if fallbackAPIKey == nil {
		return errOpenAIToGrokFallbackNotEligible
	}
	subscription, err := h.loadOpenAIToGrokFallbackSubscription(c.Request.Context(), fallbackAPIKey)
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
	if err := route.switchToGrok(reason, false); err != nil {
		return err
	}
	route.setSubscription(subscription)
	applyOpenAIToGrokFallbackRouteToContext(c, route)
	return nil
}

func (h *OpenAIGatewayHandler) restoreOpenAIToGrokFallbackContinuity(
	c *gin.Context,
	route *openAIToGrokFallbackRoute,
	sessionHash string,
	previousResponseID string,
	requiredCapability service.OpenAIEndpointCapability,
	requireCompact bool,
) (bool, error) {
	if h == nil || h.gatewayService == nil || c == nil || c.Request == nil || route == nil || !route.canFallback() {
		return false, nil
	}
	fallbackAPIKey := route.fallbackAPIKey
	if fallbackAPIKey == nil {
		return false, nil
	}
	if !h.gatewayService.HasOpenAICompatibleRouteContinuity(
		c.Request.Context(),
		fallbackAPIKey.GroupID,
		service.PlatformGrok,
		sessionHash,
		previousResponseID,
		route.requestedModel,
		requiredCapability,
		requireCompact,
	) {
		return false, nil
	}
	subscription, err := h.loadOpenAIToGrokFallbackSubscription(c.Request.Context(), fallbackAPIKey)
	if err != nil {
		return false, err
	}
	if err := route.switchToGrok("grok_route_continuity", false); err != nil {
		return false, err
	}
	route.setSubscription(subscription)
	applyOpenAIToGrokFallbackRouteToContext(c, route)
	return true, nil
}

func (h *OpenAIGatewayHandler) loadOpenAIToGrokFallbackSubscription(ctx context.Context, apiKey *service.APIKey) (*service.UserSubscription, error) {
	if apiKey == nil || apiKey.Group == nil || !apiKey.Group.IsSubscriptionType() {
		return nil, nil
	}
	if h == nil || h.fallbackSubscriptionStore == nil || apiKey.User == nil {
		return nil, errOpenAIToGrokFallbackSubscription
	}
	subscription, err := h.fallbackSubscriptionStore.GetActiveSubscription(ctx, apiKey.User.ID, apiKey.Group.ID)
	if err != nil {
		return nil, err
	}
	if subscription == nil {
		return nil, errOpenAIToGrokFallbackSubscription
	}
	return subscription, nil
}

func applyOpenAIToGrokFallbackRouteToContext(c *gin.Context, route *openAIToGrokFallbackRoute) {
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
