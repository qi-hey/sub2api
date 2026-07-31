package routes

import (
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	appmiddleware "github.com/Wei-Shaw/sub2api/internal/middleware"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RegisterUserRoutes 注册用户相关路由（需要认证）
func RegisterUserRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	jwtAuth servermiddleware.JWTAuthMiddleware,
	auditLog servermiddleware.AuditLogMiddleware,
	settingService *service.SettingService,
	redisClient *redis.Client,
) {
	rateLimiter := appmiddleware.NewRateLimiter(redisClient)
	// All loyalty mutations fail closed if Redis is unavailable.
	gameWalletCheckInLimiter := rateLimiter.LimitWithOptionsByIdentity(
		"game-wallet-checkin",
		10,
		time.Minute,
		appmiddleware.RateLimitOptions{FailureMode: appmiddleware.RateLimitFailClose},
		gameWalletUserRateLimitIdentity,
	)
	gameWalletSpinLimiter := rateLimiter.LimitWithOptionsByIdentity(
		"game-wallet-spin",
		60,
		time.Minute,
		appmiddleware.RateLimitOptions{FailureMode: appmiddleware.RateLimitFailClose},
		gameWalletUserRateLimitIdentity,
	)
	gameWalletClaimLimiter := rateLimiter.LimitWithOptionsByIdentity(
		"game-wallet-reward-claim",
		20,
		time.Minute,
		appmiddleware.RateLimitOptions{FailureMode: appmiddleware.RateLimitFailClose},
		gameWalletUserRateLimitIdentity,
	)
	gameWalletGiftLimiter := rateLimiter.LimitWithOptionsByIdentity(
		"game-wallet-gift",
		10,
		time.Minute,
		appmiddleware.RateLimitOptions{FailureMode: appmiddleware.RateLimitFailClose},
		gameWalletUserRateLimitIdentity,
	)
	gameWalletScoreLimiter := rateLimiter.LimitWithOptionsByIdentity(
		"game-wallet-score",
		120,
		time.Minute,
		appmiddleware.RateLimitOptions{FailureMode: appmiddleware.RateLimitFailClose},
		gameWalletUserRateLimitIdentity,
	)
	gameWalletLeaderboardLimiter := rateLimiter.LimitWithOptionsByIdentity(
		"game-wallet-leaderboard",
		120,
		time.Minute,
		appmiddleware.RateLimitOptions{FailureMode: appmiddleware.RateLimitFailClose},
		gameWalletUserRateLimitIdentity,
	)
	authenticated := v1.Group("")
	authenticated.Use(gin.HandlerFunc(jwtAuth))
	authenticated.Use(servermiddleware.BackendModeUserGuard(settingService))
	// 用户管理面变更类操作入审计（含 TOTP 启用/禁用、step-up 验证、密码修改等安全事件）
	authenticated.Use(gin.HandlerFunc(auditLog))
	{
		// 用户接口
		user := authenticated.Group("/user")
		{
			user.GET("/profile", h.User.GetProfile)
			user.GET("/game-wallet", h.GameWallet.GetWallet)
			user.POST("/game-wallet/check-in", gameWalletCheckInLimiter, h.GameWallet.CheckIn)
			user.POST("/game-wallet/gifts", gameWalletGiftLimiter, h.GameWallet.GiftCredits)
			user.POST("/game-wallet/slot/spin", gameWalletSpinLimiter, h.GameWallet.Spin)
			user.POST("/game-wallet/fruit/spin", gameWalletSpinLimiter, h.GameWallet.FruitSpin)
			user.POST("/game-wallet/slot/bonus/claim", gameWalletClaimLimiter, h.GameWallet.ClaimSlotBonus)
			user.GET("/game-wallet/leaderboard", gameWalletLeaderboardLimiter, h.GameWallet.GetLeaderboard)
			user.POST("/game-wallet/leaderboard/score", gameWalletScoreLimiter, h.GameWallet.SubmitLeaderboardScore)
			user.GET("/game-wallet/rewards", h.GameWallet.ListRewards)
			user.POST("/game-wallet/rewards/claim", gameWalletClaimLimiter, h.GameWallet.ClaimReward)
			user.GET("/game-wallet/transactions", h.GameWallet.ListTransactions)
			user.PUT("/password", h.User.ChangePassword)
			user.PUT("", h.User.UpdateProfile)
			user.GET("/aff", h.User.GetAffiliate)
			user.POST("/aff/transfer", h.User.TransferAffiliateQuota)
			user.POST("/account-bindings/email/send-code", h.User.SendEmailBindingCode)
			user.POST("/account-bindings/email", h.User.BindEmailIdentity)
			user.DELETE("/account-bindings/:provider", h.User.UnbindIdentity)
			user.POST("/auth-identities/bind/start", h.User.StartIdentityBinding)
			user.GET("/api-keys/:id/usage/daily", h.Usage.GetMyAPIKeyDailyUsage)
			user.GET("/platform-quotas", h.User.GetMyPlatformQuotas)

			// 通知邮箱管理
			notifyEmail := user.Group("/notify-email")
			{
				notifyEmail.POST("/send-code", h.User.SendNotifyEmailCode)
				notifyEmail.POST("/verify", h.User.VerifyNotifyEmail)
				notifyEmail.PUT("/toggle", h.User.ToggleNotifyEmail)
				notifyEmail.DELETE("", h.User.RemoveNotifyEmail)
			}

			// TOTP 双因素认证
			totp := user.Group("/totp")
			{
				totp.GET("/status", h.Totp.GetStatus)
				totp.GET("/verification-method", h.Totp.GetVerificationMethod)
				totp.POST("/send-code", h.Totp.SendVerifyCode)
				totp.POST("/setup", h.Totp.InitiateSetup)
				totp.POST("/enable", h.Totp.Enable)
				totp.POST("/disable", h.Totp.Disable)
				// 敏感操作二次验证：授予当前会话一段时间的 step-up 权限
				totp.POST("/step-up", h.Totp.StepUp)
			}
		}

		// API Key管理
		keys := authenticated.Group("/keys")
		{
			keys.GET("", h.APIKey.List)
			keys.GET("/:id", h.APIKey.GetByID)
			keys.POST("", h.APIKey.Create)
			keys.PUT("/:id", h.APIKey.Update)
			keys.DELETE("/:id", h.APIKey.Delete)
		}

		// 用户可用分组（非管理员接口）
		groups := authenticated.Group("/groups")
		{
			groups.GET("/available", h.APIKey.GetAvailableGroups)
			groups.GET("/rates", h.APIKey.GetUserGroupRates)
		}

		// 用户可用渠道（非管理员接口）
		channels := authenticated.Group("/channels")
		{
			channels.GET("/available", h.AvailableChannel.List)
		}

		// 使用记录
		usage := authenticated.Group("/usage")
		{
			usage.GET("", h.Usage.List)
			usage.GET("/errors", h.Usage.ListErrors)
			usage.GET("/errors/:id", h.Usage.GetErrorDetail)
			usage.GET("/:id", h.Usage.GetByID)
			usage.GET("/stats", h.Usage.Stats)
			// User dashboard endpoints
			usage.GET("/dashboard/stats", h.Usage.DashboardStats)
			usage.GET("/dashboard/trend", h.Usage.DashboardTrend)
			usage.GET("/dashboard/models", h.Usage.DashboardModels)
			usage.GET("/dashboard/snapshot-v2", h.Usage.DashboardSnapshotV2)
			usage.POST("/dashboard/api-keys-usage", h.Usage.DashboardAPIKeysUsage)
		}

		// 公告（用户可见）
		announcements := authenticated.Group("/announcements")
		{
			announcements.GET("", h.Announcement.List)
			announcements.POST("/:id/read", h.Announcement.MarkRead)
		}

		// 卡密兑换
		redeem := authenticated.Group("/redeem")
		{
			redeem.POST("", h.Redeem.Redeem)
			redeem.GET("/history", h.Redeem.GetHistory)
		}

		// 用户订阅
		subscriptions := authenticated.Group("/subscriptions")
		{
			subscriptions.GET("", h.Subscription.List)
			subscriptions.GET("/active", h.Subscription.GetActive)
			subscriptions.GET("/progress", h.Subscription.GetProgress)
			subscriptions.GET("/summary", h.Subscription.GetSummary)
		}

		// 渠道监控（用户只读）
		monitors := authenticated.Group("/channel-monitors")
		{
			monitors.GET("", h.ChannelMonitor.List)
			monitors.GET("/:id/status", h.ChannelMonitor.GetStatus)
		}
	}
}

func gameWalletUserRateLimitIdentity(c *gin.Context) string {
	subject, ok := servermiddleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		return ""
	}
	return "user:" + strconv.FormatInt(subject.UserID, 10)
}
