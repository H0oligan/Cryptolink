package http

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	mw "github.com/labstack/echo/v4/middleware"
	"github.com/cryptolink/cryptolink/internal/auth"
	v1 "github.com/cryptolink/cryptolink/internal/server/http/internalapi"
	"github.com/cryptolink/cryptolink/internal/server/http/emailapi"
	"github.com/cryptolink/cryptolink/internal/server/http/marketingapi"
	"github.com/cryptolink/cryptolink/internal/server/http/merchantapi"
	merchantauth "github.com/cryptolink/cryptolink/internal/server/http/merchantapi/auth"
	"github.com/cryptolink/cryptolink/internal/server/http/middleware"
	"github.com/cryptolink/cryptolink/internal/server/http/paymentapi"
	"github.com/cryptolink/cryptolink/internal/server/http/subscriptionapi"
	"github.com/cryptolink/cryptolink/internal/server/http/webhook"
	"github.com/cryptolink/cryptolink/internal/service/user"
)

// WithDashboardAPI setups routes for Merchant's Dashboard (app.o2pay.co)
func WithDashboardAPI(
	cfg Config,
	handler *merchantapi.Handler,
	authHandler *merchantauth.Handler,
	subscriptionHandler *subscriptionapi.Handler,
	emailHandler *emailapi.Handler,
	marketingHandler *marketingapi.Handler,
	tokensManager *auth.TokenAuthManager,
	users *user.Service,
	enableEmailAuth bool,
	enableGoogleAuth bool,
) Opt {
	return func(s *Server) {
		s.echo.Use(middleware.SecurityHeaders())

		guardsUsersMW := middleware.GuardsUsers()

		dashboardAPI := s.echo.Group(
			"/api/dashboard/v1",
			middleware.CORS(cfg.CORS),
			middleware.Session(cfg.Session),
			middleware.ResolvesUserBySession(users),
			middleware.ResolvesUserByToken(tokensManager, users),
			middleware.CSRF(cfg.CSRF),
		)

		authRL := mw.NewRateLimiterMemoryStore(10)
		authGroup := dashboardAPI.Group("/auth", mw.RateLimiter(authRL))

		// common auth routes
		authGroup.GET("/provider", authHandler.ListAvailableProviders)
		authGroup.GET("/csrf-cookie", authHandler.GetCookie)
		authGroup.GET("/me", authHandler.GetMe, guardsUsersMW)
		authGroup.PUT("/profile", authHandler.UpdateProfile, guardsUsersMW)
		authGroup.PUT("/password", authHandler.UpdatePassword, guardsUsersMW)
		authGroup.POST("/logout", authHandler.PostLogout, guardsUsersMW)

		// email auth routes
		if enableEmailAuth {
			authGroup.POST("/login", authHandler.PostLogin)
			authGroup.POST("/register", authHandler.PostRegister)
		}

		// email verification routes (always available)
		authGroup.GET("/verify-email", authHandler.VerifyEmail)
		authGroup.POST("/resend-verification", authHandler.ResendVerification, guardsUsersMW)

		// google auth routes
		if enableGoogleAuth {
			authGroup.GET("/redirect", authHandler.GetRedirect)
			authGroup.GET("/callback", authHandler.GetCallback)
		}

		dashboardAPI.GET("/fiat-currencies", handler.ListFiatCurrencies)

		dashboardAPI.GET("/merchant", handler.ListMerchants, guardsUsersMW)
		dashboardAPI.POST("/merchant", handler.CreateMerchant, guardsUsersMW)

		// Merchants
		merchantGroup := dashboardAPI.Group(
			"/merchant/:merchantId",
			guardsUsersMW,
			middleware.ResolvesMerchantByUUID(handler.MerchantService()),
			middleware.GuardsMerchants(),
		)

		// Merchant
		merchantGroup.GET("", handler.GetMerchant)
		merchantGroup.PUT("", handler.UpdateMerchant)
		merchantGroup.DELETE("", handler.DeleteMerchant)

		merchantGroup.PUT("/webhook", handler.UpdateMerchantWebhook)
		merchantGroup.PUT("/supported-method", handler.UpdateMerchantSupportedMethods)

		// Merchant Tokens (rate limited to prevent abuse)
		tokenRL := mw.NewRateLimiterMemoryStore(20) // 20 requests per second
		tokenGroup := merchantGroup.Group("/token", mw.RateLimiter(tokenRL))
		tokenGroup.GET("", handler.ListMerchantTokens)
		tokenGroup.POST("", handler.CreateMerchantToken)
		tokenGroup.DELETE("/:tokenId", handler.DeleteMerchantTokens)

		// Xpub Wallets
		merchantGroup.GET("/xpub-wallet", handler.ListXpubWallets)
		merchantGroup.POST("/xpub-wallet", handler.CreateXpubWallet)
		merchantGroup.GET("/xpub-wallet/:walletId", handler.GetXpubWallet)
		merchantGroup.DELETE("/xpub-wallet/:walletId", handler.DeleteXpubWallet)
		merchantGroup.POST("/xpub-wallet/:walletId/derive", handler.DeriveAddress)
		merchantGroup.GET("/xpub-wallet/:walletId/next-address", handler.GetNextAddress)
		merchantGroup.GET("/xpub-wallet/:walletId/addresses", handler.ListDerivedAddresses)

		// EVM Smart Contract Collector Wallets
		merchantGroup.GET("/evm-collector", handler.ListEvmCollectors)
		merchantGroup.POST("/evm-collector", handler.SetupEvmCollector)
		merchantGroup.GET("/evm-collector/:blockchain", handler.GetEvmCollector)
		merchantGroup.DELETE("/evm-collector/:blockchain", handler.DeleteEvmCollector)
		merchantGroup.GET("/evm-collector/:blockchain/balance", handler.GetEvmCollectorBalance)

		// Collector factory (for frontend to discover factory address before deploying)
		merchantGroup.GET("/collector-factory/:blockchain", handler.GetMerchantCollectorFactory)

		// Form
		merchantGroup.POST("/form", handler.CreateFormSubmission)

		// Currency
		merchantGroup.GET("/currency-convert", handler.GetCurrencyConvert)

		// Fee & fiat currency settings
		merchantGroup.GET("/fee-settings", handler.GetFeeSettings)
		merchantGroup.PUT("/fee-settings", handler.UpdateFeeSettings)

		// Subscription routes
		dashboardAPI.GET("/subscription/plans", subscriptionHandler.ListPlans)

		merchantGroup.GET("/subscription", subscriptionHandler.GetCurrentSubscription)
		merchantGroup.POST("/subscription/upgrade", subscriptionHandler.UpgradePlan)
		merchantGroup.POST("/subscription/cancel", subscriptionHandler.CancelSubscription)
		merchantGroup.GET("/subscription/usage", subscriptionHandler.GetUsageHistory)

		// Admin routes (super admin only)
		adminGroup := dashboardAPI.Group("/admin", guardsUsersMW, middleware.GuardsSuperAdmin())
		adminGroup.GET("/subscription/stats", subscriptionHandler.GetSystemStats)
		adminGroup.GET("/subscription/list", subscriptionHandler.ListAllSubscriptions)

		// Admin plan CRUD
		adminGroup.GET("/plans", subscriptionHandler.ListAllPlans)
		adminGroup.POST("/plans", subscriptionHandler.CreatePlan)
		adminGroup.GET("/plans/:planId", subscriptionHandler.GetPlanAdmin)
		adminGroup.PUT("/plans/:planId", subscriptionHandler.UpdatePlan)

		// Admin merchant & user management
		adminGroup.GET("/merchants", subscriptionHandler.ListAllMerchants)
		adminGroup.PUT("/merchants/:merchantId/plan", subscriptionHandler.AssignMerchantPlan)
		adminGroup.DELETE("/merchants/:merchantId", subscriptionHandler.AdminDeleteMerchant)
		adminGroup.GET("/users", subscriptionHandler.ListAllUsers)
		adminGroup.DELETE("/users/:userId", subscriptionHandler.AdminDeleteUser)

		// Admin email routes
		adminGroup.GET("/email/settings", emailHandler.GetSettings)
		adminGroup.PUT("/email/settings", emailHandler.UpdateSettings)
		adminGroup.POST("/email/send", emailHandler.SendEmail)
		adminGroup.POST("/email/test", emailHandler.TestEmail)
		adminGroup.GET("/email/log", emailHandler.GetLogs)

		// Admin contacts routes
		adminGroup.GET("/contacts", subscriptionHandler.ListAllContacts)
		adminGroup.GET("/contacts/export", subscriptionHandler.ExportContacts)

		// Admin marketing routes
		adminGroup.GET("/marketing/templates", marketingHandler.ListTemplates)
		adminGroup.GET("/marketing/templates/:templateId", marketingHandler.GetTemplate)
		adminGroup.GET("/marketing/campaigns", marketingHandler.ListCampaigns)
		adminGroup.POST("/marketing/campaigns", marketingHandler.CreateCampaign)
		adminGroup.GET("/marketing/campaigns/:campaignId", marketingHandler.GetCampaign)
		adminGroup.GET("/marketing/campaigns/:campaignId/recipients", marketingHandler.GetCampaignRecipients)
		adminGroup.POST("/marketing/campaigns/:campaignId/send", marketingHandler.SendCampaign)
		adminGroup.GET("/marketing/quota", marketingHandler.GetQuota)

		// Public unsubscribe route (no auth)
		dashboardAPI.GET("/marketing/unsubscribe", marketingHandler.Unsubscribe)

		// Admin collector factory routes
		adminGroup.GET("/collector-factories", handler.ListCollectorFactories)
		adminGroup.GET("/collector-factories/:blockchain", handler.GetCollectorFactory)
		adminGroup.POST("/collector-factories", handler.UpsertCollectorFactory)

		setupCommonMerchantRoutes(merchantGroup, handler)
	}
}

// WithMerchantAPI setups Merchant's API routes (api.o2pay.co)
func WithMerchantAPI(handler *merchantapi.Handler, tokensManager *auth.TokenAuthManager) Opt {
	return func(s *Server) {
		merchantAPI := s.echo.Group(
			"/api/merchant/v1/merchant/:merchantId",
			middleware.ResolvesMerchantByToken(tokensManager, handler.MerchantService()),
			middleware.GuardsMerchants(),
		)

		setupCommonMerchantRoutes(merchantAPI, handler)
	}
}

// setupCommonMerchantRoutes setup shared routes between dashboardAPI and merchantAPI
// session auth: "/api/dashboard/v1/merchant/{merchant}/*"
// token auth: "/api/merchant/v1/merchant/{merchant}/*"
func setupCommonMerchantRoutes(g *echo.Group, handler *merchantapi.Handler) {
	// Payment routes (rate limited to prevent abuse)
	paymentRL := mw.NewRateLimiterMemoryStore(100) // 100 requests per second
	paymentGroup := g.Group("/payment", mw.RateLimiter(paymentRL))

	paymentGroup.GET("", handler.ListPayments)
	paymentGroup.GET("/:paymentId", handler.GetPayment)
	paymentGroup.POST("", handler.CreatePayment)
	paymentGroup.POST("/:paymentId/resolve", handler.ResolvePayment)
	paymentGroup.POST("/:paymentId/decline", handler.DeclinePayment)

	// Payment link routes (rate limited to prevent abuse)
	paymentLinkRL := mw.NewRateLimiterMemoryStore(50) // 50 requests per second
	paymentLinkGroup := g.Group("/payment-link", mw.RateLimiter(paymentLinkRL))

	paymentLinkGroup.GET("", handler.ListPaymentLinks)
	paymentLinkGroup.GET("/:paymentLinkId", handler.GetPaymentLink)
	paymentLinkGroup.DELETE("/:paymentLinkId", handler.DeletePaymentLink)
	paymentLinkGroup.POST("", handler.CreatePaymentLink)

	g.GET("/balance", handler.ListBalances)

	g.GET("/customer", handler.ListCustomers)
	g.GET("/customer/:customerId", handler.GetCustomerDetails)
}

// WithPaymentAPI setups routes public-facing payment api (pay.o2pay.co)
func WithPaymentAPI(handler *paymentapi.Handler, cfg Config) Opt {
	return func(s *Server) {
		paymentAPI := s.echo.Group(
			"/api/payment/v1",
			middleware.CORS(cfg.CORS),
			middleware.Session(cfg.Session),
			middleware.CSRF(cfg.CSRF),
		)

		paymentAPI.GET("/csrf-cookie", handler.GetCookie)
		paymentAPI.GET("/currency-convert", handler.GetExchangeRate)

		paymentGroup := paymentAPI.Group(
			"/payment/:paymentId",
			middleware.ResolvesPaymentByPublicID(paymentapi.ParamPaymentID, handler.PaymentService()),
			middleware.GuardsPayment(),
			middleware.RestrictsArchivedPayments(),
		)

		paymentGroup.GET("", handler.GetPayment)
		paymentGroup.PUT("", handler.LockPaymentOptions)
		paymentGroup.POST("/customer", handler.CreateCustomer)
		paymentGroup.POST("/method", handler.CreatePaymentMethod)

		paymentGroup.GET("/supported-method", handler.GetSupportedMethods)

		paymentLinkGroup := paymentAPI.Group("/payment-link")

		paymentRL := mw.NewRateLimiterMemoryStore(1)

		paymentLinkGroup.GET("/:paymentLinkSlug", handler.GetPaymentLink)
		paymentLinkGroup.POST("/:paymentLinkSlug/payment", handler.CreatePaymentFromLink, mw.RateLimiter(paymentRL))
	}
}

func WithWebhookAPI(handler *webhook.Handler) Opt {
	return func(s *Server) {
		webhookAPI := s.echo.Group("/api/webhook/v1")
		webhookAPI.POST("/tatum/:networkId/:walletId", handler.ReceiveWebhook)
	}
}

func WithAuthDebug(files fs.FS) Opt {
	return func(s *Server) {
		s.echo.FileFS("/internal/auth-debug", "index.html", files)
	}
}

func WithDocs(files fs.FS) Opt {
	return func(s *Server) {
		s.echo.StaticFS("/internal/docs", files)
	}
}

func WithInternalAPI(h *v1.Handler) Opt {
	return func(s *Server) {
		internal := s.echo.Group("/internal/v1")

		internal.GET("/router", h.GetRouter)

		admin := internal.Group("/admin")
		admin.POST("/job", h.RunSchedulerJob)

		admin.POST("/blockchain/fee", h.CalculateTransactionFee)
		admin.POST("/blockchain/broadcast", h.BroadcastTransaction)
		admin.GET("/blockchain/receipt", h.GetTransactionReceipt)
	}
}

const (
	dashboardPrefix = "/dashboard"
	paymentsPrefix  = "/p"
)

func WithEmbeddedFrontend(dashboardUI, paymentsUI fs.FS) Opt {
	return func(s *Server) {
		spaRouter(s.echo, dashboardPrefix, dashboardUI)
		spaRouter(s.echo, paymentsPrefix, paymentsUI)
	}
}

func spaRouter(e *echo.Echo, prefix string, files fs.FS) {
	e.Group(prefix, noCacheHTML(), mw.StaticWithConfig(mw.StaticConfig{
		Root:       "/",
		Index:      "index.html",
		HTML5:      true,
		Filesystem: http.FS(files),
	}))
}

// noCacheHTML prevents Cloudflare and browsers from caching HTML responses
// so that new SPA builds are picked up immediately. JS/CSS assets have
// content-hashed filenames and can be cached indefinitely.
func noCacheHTML() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)
			ct := c.Response().Header().Get("Content-Type")
			if strings.Contains(ct, "text/html") {
				c.Response().Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			}
			return err
		}
	}
}
