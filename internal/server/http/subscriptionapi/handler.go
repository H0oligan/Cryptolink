package subscriptionapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/server/http/common"
	"github.com/oxygenpay/oxygen/internal/server/http/middleware"
	"github.com/oxygenpay/oxygen/internal/service/merchant"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/service/subscription"
	"github.com/oxygenpay/oxygen/internal/service/user"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
)

type Handler struct {
	subscriptionService *subscription.Service
	paymentService      *payment.Service
	merchantService     *merchant.Service
	userService         *user.Service
	logger              *zerolog.Logger
	adminMerchantID     int64 // Admin merchant ID for receiving subscription payments
}

func New(
	subscriptionService *subscription.Service,
	paymentService *payment.Service,
	merchantService *merchant.Service,
	userService *user.Service,
	adminMerchantID int64,
	logger *zerolog.Logger,
) *Handler {
	log := logger.With().Str("channel", "subscription_api").Logger()

	return &Handler{
		subscriptionService: subscriptionService,
		paymentService:      paymentService,
		merchantService:     merchantService,
		userService:         userService,
		logger:              &log,
		adminMerchantID:     adminMerchantID,
	}
}

// Response structures

type PlanResponse struct {
	ID                   string          `json:"id"`
	Name                 string          `json:"name"`
	Description          string          `json:"description"`
	PriceUSD             decimal.Decimal `json:"price_usd"`
	BillingPeriod        string          `json:"billing_period"`
	MaxPaymentsMonthly   *int32          `json:"max_payments_monthly"`   // null = unlimited
	MaxMerchants         int32           `json:"max_merchants"`
	MaxAPICallsMonthly   *int32          `json:"max_api_calls_monthly"`  // null = unlimited
	Features             interface{}     `json:"features"`
}

type SubscriptionResponse struct {
	UUID               string            `json:"uuid"`
	PlanID             string            `json:"plan_id"`
	Status             string            `json:"status"`
	CurrentPeriodStart string            `json:"current_period_start"`
	CurrentPeriodEnd   string            `json:"current_period_end"`
	AutoRenew          bool              `json:"auto_renew"`
	Plan               *PlanResponse     `json:"plan,omitempty"`
}

type UsageResponse struct {
	PaymentCount       int32           `json:"payment_count"`
	PaymentVolumeUSD   decimal.Decimal `json:"payment_volume_usd"`
	APICallsCount      int32           `json:"api_calls_count"`
	PeriodStart        string          `json:"period_start"`
	PeriodEnd          string          `json:"period_end"`
}

type CurrentSubscriptionResponse struct {
	Subscription *SubscriptionResponse `json:"subscription"`
	Usage        *UsageResponse        `json:"usage"`
}

type UpgradeRequest struct {
	PlanID      string `json:"plan_id" validate:"required"`
	RedirectURL string `json:"redirect_url"`
}

type UpgradeResponse struct {
	SubscriptionUUID string          `json:"subscription_uuid"`
	PaymentURL       string          `json:"payment_url,omitempty"`
	PaymentUUID      string          `json:"payment_uuid,omitempty"`
	AmountDue        decimal.Decimal `json:"amount_due"`
	Currency         string          `json:"currency"`
}

// Helper functions

func (h *Handler) SubscriptionService() *subscription.Service {
	return h.subscriptionService
}

func planToResponse(plan *subscription.SubscriptionPlan) *PlanResponse {
	if plan == nil {
		return nil
	}

	var maxPayments *int32
	if plan.MaxPaymentsMonthly.Valid {
		val := plan.MaxPaymentsMonthly.Int32
		if val == -1 {
			maxPayments = nil // unlimited
		} else {
			maxPayments = &val
		}
	}

	var maxAPICalls *int32
	if plan.MaxAPICallsMonthly.Valid {
		val := plan.MaxAPICallsMonthly.Int32
		if val == -1 {
			maxAPICalls = nil // unlimited
		} else {
			maxAPICalls = &val
		}
	}

	return &PlanResponse{
		ID:                 plan.ID,
		Name:               plan.Name,
		Description:        plan.Description,
		PriceUSD:           plan.PriceUSD,
		BillingPeriod:      plan.BillingPeriod,
		MaxPaymentsMonthly: maxPayments,
		MaxMerchants:       plan.MaxMerchants,
		MaxAPICallsMonthly: maxAPICalls,
		Features:           plan.Features,
	}
}

func subscriptionToResponse(sub *subscription.MerchantSubscription) *SubscriptionResponse {
	if sub == nil {
		return nil
	}

	return &SubscriptionResponse{
		UUID:               sub.UUID.String(),
		PlanID:             sub.PlanID,
		Status:             sub.Status,
		CurrentPeriodStart: sub.CurrentPeriodStart.Format("2006-01-02T15:04:05Z07:00"),
		CurrentPeriodEnd:   sub.CurrentPeriodEnd.Format("2006-01-02T15:04:05Z07:00"),
		AutoRenew:          sub.AutoRenew,
		Plan:               planToResponse(sub.Plan),
	}
}

func usageToResponse(usage *subscription.UsageTracking) *UsageResponse {
	if usage == nil {
		return nil
	}

	return &UsageResponse{
		PaymentCount:     usage.PaymentCount,
		PaymentVolumeUSD: usage.PaymentVolumeUSD,
		APICallsCount:    usage.APICallsCount,
		PeriodStart:      usage.PeriodStart.Format("2006-01-02T15:04:05Z07:00"),
		PeriodEnd:        usage.PeriodEnd.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// Handlers

// ListPlans returns all available subscription plans
// GET /api/dashboard/v1/subscription/plans
func (h *Handler) ListPlans(c echo.Context) error {
	ctx := c.Request().Context()

	plans, err := h.subscriptionService.ListPlans(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &model.ErrorResponse{Message: "unable to list subscription plans", Status: "internal_error"})
	}

	response := make([]*PlanResponse, len(plans))
	for i, plan := range plans {
		response[i] = planToResponse(plan)
	}

	return c.JSON(200, response)
}

// GetCurrentSubscription returns merchant's current subscription and usage
// GET /api/dashboard/v1/merchant/:merchantId/subscription
func (h *Handler) GetCurrentSubscription(c echo.Context) error {
	ctx := c.Request().Context()

	// Get merchant from context (set by middleware)
	m := middleware.ResolveMerchant(c)
	if m == nil {
		return c.JSON(http.StatusBadRequest, &model.ErrorResponse{Message: "merchant not found", Status: "not_found"})
	}

	// Get active subscription
	sub, err := h.subscriptionService.GetActiveSubscription(ctx, m.ID)
	if err != nil {
		if errors.Is(err, subscription.ErrSubscriptionNotFound) {
			return common.NotFoundResponse(c, "subscription not found")
		}
		return c.JSON(http.StatusInternalServerError, &model.ErrorResponse{Message: "unable to get subscription", Status: "internal_error"})
	}

	// Get current usage
	usage, err := h.subscriptionService.GetCurrentUsage(ctx, m.ID)
	if err != nil {
		h.logger.Warn().Err(err).Int64("merchant_id", m.ID).Msg("failed to get usage")
		// Don't fail the request, just return nil usage
		usage = nil
	}

	return c.JSON(200, CurrentSubscriptionResponse{
		Subscription: subscriptionToResponse(sub),
		Usage:        usageToResponse(usage),
	})
}

// UpgradePlan creates a subscription upgrade (creates payment if paid plan)
// POST /api/dashboard/v1/merchant/:merchantId/subscription/upgrade
func (h *Handler) UpgradePlan(c echo.Context) error {
	ctx := c.Request().Context()

	m := middleware.ResolveMerchant(c)
	if m == nil {
		return c.JSON(http.StatusBadRequest, &model.ErrorResponse{Message: "merchant not found", Status: "not_found"})
	}

	var req UpgradeRequest
	if err := c.Bind(&req); err != nil {
		return common.ValidationErrorResponse(c, "invalid request body")
	}

	if req.PlanID == "" {
		return common.ValidationErrorResponse(c, "plan_id is required")
	}

	// Get admin merchant ID
	if h.adminMerchantID == 0 {
		h.logger.Error().Msg("admin merchant ID not configured")
		return c.JSON(http.StatusInternalServerError, &model.ErrorResponse{Message: "subscription system not configured", Status: "internal_error"})
	}

	// Create payment flow adapter
	paymentAdapter := &PaymentServiceAdapter{
		paymentService: h.paymentService,
		merchantService: h.merchantService,
	}

	// Create subscription payment
	result, err := h.subscriptionService.CreateSubscriptionPayment(
		ctx,
		subscription.SubscriptionPaymentRequest{
			MerchantID:  m.ID,
			PlanID:      req.PlanID,
			RedirectURL: req.RedirectURL,
		},
		paymentAdapter,
		h.adminMerchantID,
	)

	if err != nil {
		h.logger.Error().Err(err).Int64("merchant_id", m.ID).Str("plan_id", req.PlanID).Msg("failed to create subscription payment")
		return c.JSON(http.StatusInternalServerError, &model.ErrorResponse{Message: "unable to create subscription payment", Status: "internal_error"})
	}

	return c.JSON(200, UpgradeResponse{
		SubscriptionUUID: result.SubscriptionUUID.String(),
		PaymentURL:       result.PaymentURL,
		PaymentUUID:      result.PaymentUUID.String(),
		AmountDue:        result.AmountDue,
		Currency:         result.Currency,
	})
}

// CancelSubscription cancels auto-renewal of subscription
// POST /api/dashboard/v1/merchant/:merchantId/subscription/cancel
func (h *Handler) CancelSubscription(c echo.Context) error {
	ctx := c.Request().Context()

	m := middleware.ResolveMerchant(c)
	if m == nil {
		return c.JSON(http.StatusBadRequest, &model.ErrorResponse{Message: "merchant not found", Status: "not_found"})
	}

	sub, err := h.subscriptionService.GetActiveSubscription(ctx, m.ID)
	if err != nil {
		if errors.Is(err, subscription.ErrSubscriptionNotFound) {
			return common.NotFoundResponse(c, "subscription not found")
		}
		return c.JSON(http.StatusInternalServerError, &model.ErrorResponse{Message: "unable to get subscription", Status: "internal_error"})
	}

	err = h.subscriptionService.CancelSubscription(ctx, sub.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &model.ErrorResponse{Message: "unable to cancel subscription", Status: "internal_error"})
	}

	return c.JSON(200, map[string]interface{}{
		"message": "subscription cancelled successfully",
		"auto_renew": false,
	})
}

// GetUsageHistory returns usage history for merchant
// GET /api/dashboard/v1/merchant/:merchantId/subscription/usage
func (h *Handler) GetUsageHistory(c echo.Context) error {
	ctx := c.Request().Context()

	m := middleware.ResolveMerchant(c)
	if m == nil {
		return c.JSON(http.StatusBadRequest, &model.ErrorResponse{Message: "merchant not found", Status: "not_found"})
	}

	history, err := h.subscriptionService.GetUsageHistory(ctx, m.ID, 12)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &model.ErrorResponse{Message: "unable to get usage history", Status: "internal_error"})
	}

	response := make([]*UsageResponse, len(history))
	for i, usage := range history {
		response[i] = usageToResponse(usage)
	}

	return c.JSON(200, response)
}

// Admin Handlers

// ListAllSubscriptions returns all subscriptions (admin only)
// GET /api/dashboard/v1/admin/subscriptions
func (h *Handler) ListAllSubscriptions(c echo.Context) error {
	ctx := c.Request().Context()

	subs, err := h.subscriptionService.ListAllSubscriptions(ctx, 100, 0)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &model.ErrorResponse{Message: "unable to list subscriptions", Status: "internal_error"})
	}

	response := make([]*SubscriptionResponse, len(subs))
	for i, sub := range subs {
		response[i] = subscriptionToResponse(sub)
	}

	return c.JSON(200, response)
}

// GetSystemStats returns system-wide statistics (admin only)
// GET /api/dashboard/v1/admin/stats
func (h *Handler) GetSystemStats(c echo.Context) error {
	ctx := c.Request().Context()

	stats, err := h.subscriptionService.GetSystemStats(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &model.ErrorResponse{Message: "unable to get system stats", Status: "internal_error"})
	}

	return c.JSON(200, stats)
}
