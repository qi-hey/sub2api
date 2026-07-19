package middleware

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthGroupRoutingSelectsRequestLocalGroupAndRestoresBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	source := routableAPIKeyForMiddlewareTest()
	repo := fakeAPIKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) {
		return source, nil
	}}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	apiKeyService := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)
	body := `{"model":"gpt-5.4","input":"hello"}`

	router := gin.New()
	router.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(apiKeyService, nil, cfg)))
	router.POST("/v1/responses", func(c *gin.Context) {
		selected, ok := GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.Equal(t, int64(12), *selected.GroupID)
		require.Equal(t, service.PlatformGrok, selected.Group.Platform)
		group, ok := c.Request.Context().Value(ctxkey.Group).(*service.Group)
		require.True(t, ok)
		require.Equal(t, int64(12), group.ID)
		restored, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		require.Equal(t, body, string(restored))
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+source.Key)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, int64(2), *source.GroupID)
	require.Equal(t, service.PlatformOpenAI, source.Group.Platform)
}

func TestAPIKeyAuthGroupRoutingUsesSelectedGroupForSubscription(t *testing.T) {
	gin.SetMode(gin.TestMode)
	source := routableAPIKeyForMiddlewareTest()
	source.Group.SubscriptionType = service.SubscriptionTypeSubscription
	source.Groups[0].SubscriptionType = service.SubscriptionTypeSubscription
	source.Groups[2].SubscriptionType = service.SubscriptionTypeSubscription
	repo := fakeAPIKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) {
		return source, nil
	}}
	cfg := &config.Config{RunMode: config.RunModeStandard}
	apiKeyService := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)
	var subscriptionGroupID int64
	now := time.Now()
	subscriptionRepo := fakeGoogleSubscriptionRepo{getActive: func(_ context.Context, userID, groupID int64) (*service.UserSubscription, error) {
		subscriptionGroupID = groupID
		return &service.UserSubscription{
			ID:                 90,
			UserID:             userID,
			GroupID:            groupID,
			Status:             service.SubscriptionStatusActive,
			ExpiresAt:          now.Add(time.Hour),
			DailyWindowStart:   &now,
			WeeklyWindowStart:  &now,
			MonthlyWindowStart: &now,
		}, nil
	}}
	subscriptionService := service.NewSubscriptionService(nil, subscriptionRepo, nil, nil, cfg)
	t.Cleanup(subscriptionService.Stop)

	router := gin.New()
	router.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(apiKeyService, subscriptionService, cfg)))
	router.POST("/v1/responses", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.4"}`))
	req.Header.Set("x-api-key", source.Key)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, int64(12), subscriptionGroupID)
}

func TestAPIKeyAuthGroupRoutingHonorsForcedPlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)
	source := routableAPIKeyForMiddlewareTest()
	repo := fakeAPIKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) {
		return source, nil
	}}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	apiKeyService := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)

	router := gin.New()
	router.Use(ForcePlatform(service.PlatformAnthropic))
	router.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(apiKeyService, nil, cfg)))
	router.GET("/forced", func(c *gin.Context) {
		selected, ok := GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.Equal(t, int64(11), *selected.GroupID)
		c.Status(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/forced", nil)
	req.Header.Set("x-api-key", source.Key)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestAPIKeyAuthGroupRoutingForcedPlatformRejectsUnboundKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	source := routableAPIKeyForMiddlewareTest()
	source.GroupID = nil
	source.Group = nil
	source.GroupIDs = nil
	source.Groups = nil
	repo := fakeAPIKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) {
		return source, nil
	}}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	apiKeyService := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)

	router := gin.New()
	router.Use(ForcePlatform(service.PlatformAnthropic))
	router.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(apiKeyService, nil, cfg)))
	router.GET("/forced", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodGet, "/forced", nil)
	req.Header.Set("x-api-key", source.Key)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "API_KEY_GROUP_NOT_BOUND")
}

func TestAPIKeyAuthGroupRoutingUnboundKeyKeepsLegacyUnforcedBehavior(t *testing.T) {
	gin.SetMode(gin.TestMode)
	source := routableAPIKeyForMiddlewareTest()
	source.GroupID = nil
	source.Group = nil
	source.GroupIDs = nil
	source.Groups = nil
	repo := fakeAPIKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) {
		return source, nil
	}}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	apiKeyService := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)

	router := gin.New()
	router.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(apiKeyService, nil, cfg)))
	router.GET("/legacy", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodGet, "/legacy", nil)
	req.Header.Set("x-api-key", source.Key)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestAPIKeyAuthGroupRoutingReturnsStructuredUnboundError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	source := routableAPIKeyForMiddlewareTest()
	source.Groups = source.Groups[:2]
	source.GroupIDs = source.GroupIDs[:2]
	repo := fakeAPIKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) {
		return source, nil
	}}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	apiKeyService := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)

	router := gin.New()
	router.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(apiKeyService, nil, cfg)))
	router.POST("/v1/responses", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.4"}`))
	req.Header.Set("x-api-key", source.Key)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "API_KEY_GROUP_NOT_BOUND")
}

func TestGoogleAPIKeyAuthGroupRoutingSelectsBoundGeminiGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	source := routableAPIKeyForMiddlewareTest()
	source.GroupIDs = append(source.GroupIDs, 14)
	source.Groups = append(source.Groups, service.Group{
		ID:       14,
		Name:     "Gemini",
		Platform: service.PlatformGemini,
		Status:   service.StatusActive,
		Hydrated: true,
	})
	repo := fakeAPIKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) {
		return source, nil
	}}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	apiKeyService := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)

	router := gin.New()
	router.Use(APIKeyAuthWithSubscriptionGoogle(apiKeyService, nil, cfg))
	router.GET("/v1beta/models", func(c *gin.Context) {
		selected, ok := GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.Equal(t, int64(14), *selected.GroupID)
		c.Status(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)
	req.Header.Set("x-goog-api-key", source.Key)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestGoogleAPIKeyAuthGroupRoutingForcedPlatformRejectsUnboundKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	source := routableAPIKeyForMiddlewareTest()
	source.GroupID = nil
	source.Group = nil
	source.GroupIDs = nil
	source.Groups = nil
	repo := fakeAPIKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) {
		return source, nil
	}}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	apiKeyService := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)

	router := gin.New()
	router.Use(ForcePlatform(service.PlatformAntigravity))
	router.Use(APIKeyAuthWithSubscriptionGoogle(apiKeyService, nil, cfg))
	router.GET("/antigravity/v1beta/models", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodGet, "/antigravity/v1beta/models", nil)
	req.Header.Set("x-goog-api-key", source.Key)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), `"status":"INVALID_ARGUMENT"`)
}

func TestAPIKeyAuthGroupRoutingPreservesUnavailableDefaultContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	source := routableAPIKeyForMiddlewareTest()
	source.Group.Status = service.StatusDisabled
	source.Groups[0].Status = service.StatusDisabled
	repo := fakeAPIKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) {
		return source, nil
	}}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	apiKeyService := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)

	router := gin.New()
	router.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(apiKeyService, nil, cfg)))
	router.GET("/legacy", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodGet, "/legacy", nil)
	req.Header.Set("x-api-key", source.Key)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "GROUP_DISABLED")
}

func TestGoogleAPIKeyAuthGroupRoutingErrorUsesGoogleContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	source := routableAPIKeyForMiddlewareTest()
	repo := fakeAPIKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) {
		return source, nil
	}}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	apiKeyService := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)

	router := gin.New()
	router.Use(APIKeyAuthWithSubscriptionGoogle(apiKeyService, nil, cfg))
	router.GET("/v1beta/models", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)
	req.Header.Set("x-goog-api-key", source.Key)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{
		"error": {
			"code": 400,
			"message": "api key is not bound to the selected group",
			"status": "INVALID_ARGUMENT"
		}
	}`, rec.Body.String())
}

func TestRequestModelForAPIKeyRoutingRestoresValidAndMalformedBodies(t *testing.T) {
	for _, tt := range []struct {
		name      string
		body      string
		wantModel string
	}{
		{name: "string model", body: `{"model":"gpt-5.4","input":"hi"}`, wantModel: "gpt-5.4"},
		{name: "non string model", body: `{"model":54}`, wantModel: ""},
		{name: "incomplete model string", body: `{"model":"gpt-5.4`, wantModel: ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(tt.body))

			model := requestModelForAPIKeyRouting(req)

			require.Equal(t, tt.wantModel, model)
			restored, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			require.Equal(t, tt.body, string(restored))
		})
	}
}

func TestRequestModelForAPIKeyRoutingPreservesBodyLimitError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.4"}`))
	req.Body = http.MaxBytesReader(httptest.NewRecorder(), req.Body, 4)

	model := requestModelForAPIKeyRouting(req)

	require.Empty(t, model)
	_, err := io.ReadAll(req.Body)
	var maxErr *http.MaxBytesError
	require.True(t, errors.As(err, &maxErr))
}

func TestRequestModelForAPIKeyRoutingSkipsNonJSONBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", strings.NewReader(`{"model":"gpt-5.4"}`))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=test")

	model := requestModelForAPIKeyRouting(req)

	require.Empty(t, model)
	restored, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, `{"model":"gpt-5.4"}`, string(restored))
}

func TestRequestModelForAPIKeyRoutingSpoolsLargeBodyWithLateModel(t *testing.T) {
	body := `{"input":"` + strings.Repeat("x", 2<<20) + `","model":"gpt-5.4"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req.Body = io.NopCloser(strings.NewReader(body))

	model := requestModelForAPIKeyRouting(req)

	require.Equal(t, "gpt-5.4", model)
	replay, ok := req.Body.(*replayedRequestBody)
	require.True(t, ok)
	require.LessOrEqual(t, replay.memoryBytes, apiKeyRoutingBodyMemoryBytes)
	require.NotEmpty(t, replay.tempPath)
	_, err := os.Stat(replay.tempPath)
	require.NoError(t, err)
	restored, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, body, string(restored))
	require.NoError(t, req.Body.Close())
	_, err = os.Stat(replay.tempPath)
	require.True(t, os.IsNotExist(err))
}

func TestRequestModelForAPIKeyRoutingRejectsExcessiveJSONDepth(t *testing.T) {
	body := `{"input":` + strings.Repeat("[", 300) + `0` + strings.Repeat("]", 300) + `,"model":"gpt-5.4"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))

	model := requestModelForAPIKeyRouting(req)

	require.Empty(t, model)
	restored, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, body, string(restored))
}

func TestRequestBodyLimitCleansUnreadAPIKeyRoutingSpool(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"input":"` + strings.Repeat("x", 2<<20) + `","model":"gpt-5.4"}`
	tempPath := ""
	router := gin.New()
	router.Use(RequestBodyLimit(4 << 20))
	router.POST("/v1/responses", func(c *gin.Context) {
		require.Equal(t, "gpt-5.4", requestModelForAPIKeyRouting(c.Request))
		replay, ok := c.Request.Body.(*replayedRequestBody)
		require.True(t, ok)
		tempPath = replay.tempPath
		c.Status(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.NotEmpty(t, tempPath)
	_, err := os.Stat(tempPath)
	require.True(t, os.IsNotExist(err))
}

func TestRequestModelForAPIKeyRoutingRejectsWhenSpoolBudgetIsFull(t *testing.T) {
	body := `{"input":"` + strings.Repeat("x", 2<<20) + `","model":"gpt-5.4"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	previous := apiKeyRoutingActiveSpoolBytes.Swap(apiKeyRoutingSpoolBudgetBytes)
	t.Cleanup(func() { apiKeyRoutingActiveSpoolBytes.Store(previous) })

	model, err := requestModelForAPIKeyRoutingWithError(req)

	require.Empty(t, model)
	require.Equal(t, http.StatusServiceUnavailable, infraerrors.Code(err))
	require.Equal(t, "API_KEY_GROUP_ROUTING_BUSY", infraerrors.Reason(err))
	restored, readErr := io.ReadAll(req.Body)
	require.NoError(t, readErr)
	require.Equal(t, body, string(restored))
}

func routableAPIKeyForMiddlewareTest() *service.APIKey {
	defaultID := int64(2)
	user := &service.User{
		ID:          7,
		Role:        service.RoleUser,
		Status:      service.StatusActive,
		Balance:     10,
		Concurrency: 3,
	}
	groups := []service.Group{
		{ID: 2, Name: "Codex", Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true},
		{ID: 11, Name: "Claude", Platform: service.PlatformAnthropic, Status: service.StatusActive, Hydrated: true},
		{ID: 12, Name: "Grok", Platform: service.PlatformGrok, Status: service.StatusActive, Hydrated: true},
	}
	return &service.APIKey{
		ID:       100,
		UserID:   user.ID,
		Key:      "multi-group-key",
		Status:   service.StatusActive,
		User:     user,
		GroupID:  &defaultID,
		Group:    &groups[0],
		GroupIDs: []int64{2, 11, 12},
		Groups:   groups,
	}
}
