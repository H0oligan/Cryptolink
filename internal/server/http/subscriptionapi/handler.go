package subscriptionapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/cryptolink/cryptolink/internal/server/http/common"
	"github.com/cryptolink/cryptolink/internal/server/http/middleware"
	"github.com/cryptolink/cryptolink/internal/service/merchant"
	"github.com/cryptolink/cryptolink/internal/service/payment"
	"github.com/cryptolink/cryptolink/internal/service/subscription"
	"github.com/cryptolink/cryptolink/internal/service/user"
	"github.com/cryptolink/cryptolink/pkg/api-dashboard/v1/model"
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
	ID                   string           `json:"id"`
	Name                 string           `json:"name"`
	Description          string           `json:"description"`
	PriceUSD             decimal.Decimal  `json:"price_usd"`
	BillingPeriod        string           `json:"billing_period"`
	MaxPaymentsMonthly   *int32           `json:"max_payments_monthly"`       // null = unlimited
	MaxMerchants         int32            `json:"max_merchants"`
	MaxAPICallsMonthly   *int32           `json:"max_api_calls_monthly"`      // null = unlimited
	MaxVolumeMonthlyUSD  *decimal.Decimal `json:"max_volume_monthly_usd"`     // null = unlimited
	Features             interface{}      `json:"features"`
	IsActive             *bool            `json:"is_active,omitempty"`
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

	var maxVolume *decimal.Decimal
	if plan.MaxVolumeMonthlyUSD.Valid {
		maxVolume = &plan.MaxVolumeMonthlyUSD.Decimal
	}

	return &PlanResponse{
		ID:                  plan.ID,
		Name:                plan.Name,
		Description:         plan.Description,
		PriceUSD:            plan.PriceUSD,
		BillingPeriod:       plan.BillingPeriod,
		MaxPaymentsMonthly:  maxPayments,
		MaxMerchants:        plan.MaxMerchants,
		MaxAPICallsMonthly:  maxAPICalls,
		MaxVolumeMonthlyUSD: maxVolume,
		Features:            plan.Features,
	}
}

func planToAdminResponse(plan *subscription.SubscriptionPlan) *PlanResponse {
	r := planToResponse(plan)
	if r != nil {
		r.IsActive = &plan.IsActive
	}
	return r
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

// ===== Admin Plan CRUD =====

type CreatePlanRequest struct {
	ID                  string           `json:"id"`
	Name                string           `json:"name"`
	Description         string           `json:"description"`
	PriceUSD            decimal.Decimal  `json:"price_usd"`
	BillingPeriod       string           `json:"billing_period"`
	MaxPaymentsMonthly  *int32           `json:"max_payments_monthly"`
	MaxMerchants        int32            `json:"max_merchants"`
	MaxAPICallsMonthly  *int32           `json:"max_api_calls_monthly"`
	MaxVolumeMonthlyUSD *decimal.Decimal `json:"max_volume_monthly_usd"`
	Features            interface{}      `json:"features"`
	IsActive            bool             `json:"is_active"`
}

// ListAllPlans returns all plans including inactive (admin only)
// GET /api/dashboard/v1/admin/plans
func (h *Handler) ListAllPlans(c echo.Context) error {
	ctx := c.Request().Context()

	plans, err := h.subscriptionService.ListAllPlans(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &model.ErrorResponse{Message: "unable to list plans", Status: "internal_error"})
	}

	response := make([]*PlanResponse, len(plans))
	for i, plan := range plans {
		response[i] = planToAdminResponse(plan)
	}

	return c.JSON(200, response)
}

// CreatePlan creates a new subscription plan (admin only)
// POST /api/dashboard/v1/admin/plans
func (h *Handler) CreatePlan(c echo.Context) error {
	ctx := c.Request().Context()

	var req CreatePlanRequest
	if err := c.Bind(&req); err != nil {
		return common.ValidationErrorResponse(c, "invalid request body")
	}

	if req.ID == "" || req.Name == "" {
		return common.ValidationErrorResponse(c, "id and name are required")
	}

	plan := requestToPlan(&req)

	created, err := h.subscriptionService.CreatePlan(ctx, plan)
	if err != nil {
		h.logger.Error().Err(err).Str("plan_id", req.ID).Msg("failed to create plan")
		return c.JSON(http.StatusInternalServerError, &model.ErrorResponse{Message: "failed to create plan: " + err.Error(), Status: "internal_error"})
	}

	return c.JSON(http.StatusCreated, planToAdminResponse(created))
}

// UpdatePlan updates an existing plan (admin only)
// PUT /api/dashboard/v1/admin/plans/:planId
func (h *Handler) UpdatePlan(c echo.Context) error {
	ctx := c.Request().Context()
	planID := c.Param("planId")

	if planID == "" {
		return common.ValidationErrorResponse(c, "plan ID is required")
	}

	var req CreatePlanRequest
	if err := c.Bind(&req); err != nil {
		return common.ValidationErrorResponse(c, "invalid request body")
	}

	plan := requestToPlan(&req)

	updated, err := h.subscriptionService.UpdatePlan(ctx, planID, plan)
	if err != nil {
		h.logger.Error().Err(err).Str("plan_id", planID).Msg("failed to update plan")
		return c.JSON(http.StatusInternalServerError, &model.ErrorResponse{Message: "failed to update plan: " + err.Error(), Status: "internal_error"})
	}

	return c.JSON(200, planToAdminResponse(updated))
}

// GetPlan returns a single plan by ID (admin only)
// GET /api/dashboard/v1/admin/plans/:planId
func (h *Handler) GetPlanAdmin(c echo.Context) error {
	ctx := c.Request().Context()
	planID := c.Param("planId")

	plan, err := h.subscriptionService.GetPlanByID(ctx, planID)
	if err != nil {
		return common.NotFoundResponse(c, "plan not found")
	}

	return c.JSON(200, planToAdminResponse(plan))
}

func requestToPlan(req *CreatePlanRequest) *subscription.SubscriptionPlan {
	plan := &subscription.SubscriptionPlan{
		ID:            req.ID,
		Name:          req.Name,
		Description:   req.Description,
		PriceUSD:      req.PriceUSD,
		BillingPeriod: req.BillingPeriod,
		MaxMerchants:  req.MaxMerchants,
		IsActive:      req.IsActive,
	}

	if req.MaxPaymentsMonthly != nil {
		plan.MaxPaymentsMonthly.Valid = true
		plan.MaxPaymentsMonthly.Int32 = *req.MaxPaymentsMonthly
	}

	if req.MaxAPICallsMonthly != nil {
		plan.MaxAPICallsMonthly.Valid = true
		plan.MaxAPICallsMonthly.Int32 = *req.MaxAPICallsMonthly
	}

	if req.MaxVolumeMonthlyUSD != nil {
		plan.MaxVolumeMonthlyUSD.Valid = true
		plan.MaxVolumeMonthlyUSD.Decimal = *req.MaxVolumeMonthlyUSD
	}

	if req.Features != nil {
		featuresBytes, _ := json.Marshal(req.Features)
		plan.Features = featuresBytes
	} else {
		plan.Features = []byte("{}")
	}

	return plan
}

// ===== Admin Merchant Plan Assignment =====

type AssignPlanRequest struct {
	PlanID string `json:"plan_id"`
}

// AssignMerchantPlan assigns a plan to a merchant (admin only)
// PUT /api/dashboard/v1/admin/merchants/:merchantId/plan
func (h *Handler) AssignMerchantPlan(c echo.Context) error {
	ctx := c.Request().Context()

	merchantIDStr := c.Param("merchantId")
	merchantID, err := strconv.ParseInt(merchantIDStr, 10, 64)
	if err != nil {
		return common.ValidationErrorResponse(c, "invalid merchant ID")
	}

	var req AssignPlanRequest
	if err := c.Bind(&req); err != nil {
		return common.ValidationErrorResponse(c, "invalid request body")
	}

	if req.PlanID == "" {
		return common.ValidationErrorResponse(c, "plan_id is required")
	}

	err = h.subscriptionService.AssignMerchantPlan(ctx, merchantID, req.PlanID)
	if err != nil {
		h.logger.Error().Err(err).Int64("merchant_id", merchantID).Str("plan_id", req.PlanID).Msg("failed to assign plan")
		return c.JSON(http.StatusInternalServerError, &model.ErrorResponse{
			Message: "failed to assign plan: " + err.Error(),
			Status:  "internal_error",
		})
	}

	return c.JSON(200, map[string]interface{}{
		"message":     "plan assigned successfully",
		"merchant_id": merchantID,
		"plan_id":     req.PlanID,
	})
}

// ===== Admin Merchant & User Listing =====

// ListAllMerchants returns all merchants with plan/usage info (admin only)
// GET /api/dashboard/v1/admin/merchants
func (h *Handler) ListAllMerchants(c echo.Context) error {
	ctx := c.Request().Context()

	limit := 50
	offset := 0
	if l := c.QueryParam("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if o := c.QueryParam("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	merchants, total, err := h.subscriptionService.ListAllMerchants(ctx, limit, offset)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to list merchants")
		return c.JSON(http.StatusInternalServerError, &model.ErrorResponse{Message: "unable to list merchants", Status: "internal_error"})
	}

	return c.JSON(200, map[string]interface{}{
		"results": merchants,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// ListAllUsers returns all users (admin only)
// GET /api/dashboard/v1/admin/users
func (h *Handler) ListAllUsers(c echo.Context) error {
	ctx := c.Request().Context()

	limit := 50
	offset := 0
	if l := c.QueryParam("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if o := c.QueryParam("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	users, total, err := h.subscriptionService.ListAllUsers(ctx, limit, offset)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to list users")
		return c.JSON(http.StatusInternalServerError, &model.ErrorResponse{Message: "unable to list users", Status: "internal_error"})
	}

	return c.JSON(200, map[string]interface{}{
		"results": users,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}
