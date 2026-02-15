package subscription

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
)

// PaymentService interface for creating payments
// This will be implemented by the payment service
type PaymentService interface {
	CreatePayment(ctx context.Context, params PaymentParams) (*PaymentResult, error)
}

// PaymentParams represents payment creation parameters
type PaymentParams struct {
	MerchantID      int64
	Amount          decimal.Decimal
	Currency        string
	Description     string
	RedirectURL     string
	CustomerEmail   string
	Metadata        map[string]interface{}
}

// PaymentResult represents created payment details
type PaymentResult struct {
	ID        int64
	PublicID  uuid.UUID
	URL       string
}

// SubscriptionPaymentRequest represents a subscription upgrade request
type SubscriptionPaymentRequest struct {
	MerchantID  int64
	PlanID      string
	RedirectURL string
}

// SubscriptionPaymentResponse represents the payment details for subscription
type SubscriptionPaymentResponse struct {
	SubscriptionUUID uuid.UUID       `json:"subscription_uuid"`
	PaymentURL       string          `json:"payment_url"`
	PaymentUUID      uuid.UUID       `json:"payment_uuid"`
	AmountDue        decimal.Decimal `json:"amount_due"`
	Currency         string          `json:"currency"`
	ExpiresAt        time.Time       `json:"expires_at"`
}

// CreateSubscriptionPayment creates a payment for a subscription upgrade
func (s *Service) CreateSubscriptionPayment(ctx context.Context, req SubscriptionPaymentRequest, paymentService PaymentService, adminMerchantID int64) (*SubscriptionPaymentResponse, error) {
	// Get the plan
	plan, err := s.GetPlan(ctx, req.PlanID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get subscription plan")
	}

	// Free plans don't require payment
	if plan.PriceUSD.Equal(decimal.Zero) {
		// Directly create active subscription
		periodStart := time.Now()
		periodEnd := periodStart.AddDate(1, 0, 0) // 1 year for free plans

		sub, err := s.CreateSubscription(ctx, req.MerchantID, req.PlanID, periodStart, periodEnd)
		if err != nil {
			return nil, err
		}

		return &SubscriptionPaymentResponse{
			SubscriptionUUID: sub.UUID,
			PaymentURL:       "",
			PaymentUUID:      uuid.Nil,
			AmountDue:        decimal.Zero,
			Currency:         "USD",
			ExpiresAt:        periodEnd,
		}, nil
	}

	// Create pending subscription first
	periodStart := time.Now()
	var periodEnd time.Time
	if plan.BillingPeriod == "monthly" {
		periodEnd = periodStart.AddDate(0, 1, 0)
	} else if plan.BillingPeriod == "yearly" {
		periodEnd = periodStart.AddDate(1, 0, 0)
	} else {
		periodEnd = periodStart.AddDate(0, 1, 0) // Default to monthly
	}

	sub, err := s.CreateSubscription(ctx, req.MerchantID, req.PlanID, periodStart, periodEnd)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create subscription")
	}

	// Create metadata for payment
	metadata := map[string]interface{}{
		"subscription_id":   sub.UUID.String(),
		"subscription_plan": plan.ID,
		"billing_period":    plan.BillingPeriod,
		"merchant_id":       req.MerchantID,
	}

	// Create payment to admin merchant
	paymentParams := PaymentParams{
		MerchantID:  adminMerchantID,
		Amount:      plan.PriceUSD,
		Currency:    "USD",
		Description: fmt.Sprintf("Cryptolink %s Subscription - %s", plan.Name, plan.BillingPeriod),
		RedirectURL: req.RedirectURL,
		Metadata:    metadata,
	}

	payment, err := paymentService.CreatePayment(ctx, paymentParams)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create subscription payment")
	}

	return &SubscriptionPaymentResponse{
		SubscriptionUUID: sub.UUID,
		PaymentURL:       payment.URL,
		PaymentUUID:      payment.PublicID,
		AmountDue:        plan.PriceUSD,
		Currency:         "USD",
		ExpiresAt:        periodEnd,
	}, nil
}

// HandlePaymentWebhook processes payment webhook for subscription activation
func (s *Service) HandlePaymentWebhook(ctx context.Context, paymentID int64, paymentMetadata string) error {
	// Fetch payment metadata from database if not provided
	var metadataBytes []byte
	if paymentMetadata == "" {
		query := `SELECT metadata FROM payments WHERE id = $1`
		err := s.db.QueryRow(ctx, query, paymentID).Scan(&metadataBytes)
		if err != nil {
			return errors.Wrap(err, "failed to fetch payment metadata")
		}
		if metadataBytes == nil {
			// No metadata, not a subscription payment
			return nil
		}
	} else {
		metadataBytes = []byte(paymentMetadata)
	}

	// Parse metadata
	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		// Not valid JSON metadata, ignore
		return nil
	}

	// Check if this is a subscription payment
	subscriptionIDStr, ok := metadata["subscription_id"].(string)
	if !ok || subscriptionIDStr == "" {
		// Not a subscription payment
		return nil
	}

	subscriptionUUID, err := uuid.Parse(subscriptionIDStr)
	if err != nil {
		return errors.Wrap(err, "invalid subscription UUID in payment metadata")
	}

	// Get subscription
	query := `SELECT id, status FROM merchant_subscriptions WHERE uuid = $1`
	var subscriptionID int64
	var status string
	err = s.db.QueryRow(ctx, query, subscriptionUUID).Scan(&subscriptionID, &status)
	if err != nil {
		return errors.Wrap(err, "failed to find subscription")
	}

	// Only activate if pending payment
	if status != StatusPendingPayment {
		s.logger.Info().
			Int64("subscription_id", subscriptionID).
			Str("status", status).
			Msg("subscription not in pending_payment status, skipping activation")
		return nil
	}

	// Activate subscription
	err = s.ActivateSubscription(ctx, subscriptionID, paymentID)
	if err != nil {
		return errors.Wrap(err, "failed to activate subscription")
	}

	s.logger.Info().
		Int64("subscription_id", subscriptionID).
		Int64("payment_id", paymentID).
		Msg("subscription activated successfully")

	return nil
}

// GetSubscriptionByUUID gets a subscription by its UUID
func (s *Service) GetSubscriptionByUUID(ctx context.Context, subscriptionUUID uuid.UUID) (*MerchantSubscription, error) {
	query := `SELECT id, uuid, merchant_id, plan_id, status, current_period_start, current_period_end,
	                 payment_id, auto_renew, created_at, updated_at, cancelled_at
	          FROM merchant_subscriptions WHERE uuid = $1`

	var sub MerchantSubscription
	err := s.db.QueryRow(ctx, query, subscriptionUUID).Scan(
		&sub.ID, &sub.UUID, &sub.MerchantID, &sub.PlanID, &sub.Status,
		&sub.CurrentPeriodStart, &sub.CurrentPeriodEnd, &sub.PaymentID,
		&sub.AutoRenew, &sub.CreatedAt, &sub.UpdatedAt, &sub.CancelledAt,
	)

	if err != nil {
		return nil, errors.Wrap(err, "failed to get subscription")
	}

	// Load plan details
	sub.Plan, _ = s.GetPlan(ctx, sub.PlanID)

	return &sub, nil
}

// ListSubscriptionHistory returns all subscriptions for a merchant
func (s *Service) ListSubscriptionHistory(ctx context.Context, merchantID int64) ([]*MerchantSubscription, error) {
	query := `SELECT id, uuid, merchant_id, plan_id, status, current_period_start, current_period_end,
	                 payment_id, auto_renew, created_at, updated_at, cancelled_at
	          FROM merchant_subscriptions
	          WHERE merchant_id = $1
	          ORDER BY created_at DESC`

	rows, err := s.db.Query(ctx, query, merchantID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list subscription history")
	}
	defer rows.Close()

	var subscriptions []*MerchantSubscription
	for rows.Next() {
		var sub MerchantSubscription
		err := rows.Scan(
			&sub.ID, &sub.UUID, &sub.MerchantID, &sub.PlanID, &sub.Status,
			&sub.CurrentPeriodStart, &sub.CurrentPeriodEnd, &sub.PaymentID,
			&sub.AutoRenew, &sub.CreatedAt, &sub.UpdatedAt, &sub.CancelledAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan subscription")
		}

		// Load plan details
		sub.Plan, _ = s.GetPlan(ctx, sub.PlanID)
		subscriptions = append(subscriptions, &sub)
	}

	return subscriptions, nil
}

// GetUsageHistory returns usage history for a merchant
func (s *Service) GetUsageHistory(ctx context.Context, merchantID int64, limit int) ([]*UsageTracking, error) {
	if limit <= 0 {
		limit = 12 // Default to 12 months
	}

	query := `SELECT id, merchant_id, period_start, period_end, payment_count,
	                 payment_volume_usd, api_calls_count, created_at, updated_at
	          FROM usage_tracking
	          WHERE merchant_id = $1
	          ORDER BY period_start DESC
	          LIMIT $2`

	rows, err := s.db.Query(ctx, query, merchantID, limit)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get usage history")
	}
	defer rows.Close()

	var usages []*UsageTracking
	for rows.Next() {
		var usage UsageTracking
		err := rows.Scan(
			&usage.ID, &usage.MerchantID, &usage.PeriodStart, &usage.PeriodEnd,
			&usage.PaymentCount, &usage.PaymentVolumeUSD, &usage.APICallsCount,
			&usage.CreatedAt, &usage.UpdatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan usage")
		}
		usages = append(usages, &usage)
	}

	return usages, nil
}

// Admin Functions

// ListAllSubscriptions returns all subscriptions (admin only)
func (s *Service) ListAllSubscriptions(ctx context.Context, limit, offset int) ([]*MerchantSubscription, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `SELECT id, uuid, merchant_id, plan_id, status, current_period_start, current_period_end,
	                 payment_id, auto_renew, created_at, updated_at, cancelled_at
	          FROM merchant_subscriptions
	          ORDER BY created_at DESC
	          LIMIT $1 OFFSET $2`

	rows, err := s.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list all subscriptions")
	}
	defer rows.Close()

	var subscriptions []*MerchantSubscription
	for rows.Next() {
		var sub MerchantSubscription
		err := rows.Scan(
			&sub.ID, &sub.UUID, &sub.MerchantID, &sub.PlanID, &sub.Status,
			&sub.CurrentPeriodStart, &sub.CurrentPeriodEnd, &sub.PaymentID,
			&sub.AutoRenew, &sub.CreatedAt, &sub.UpdatedAt, &sub.CancelledAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan subscription")
		}

		// Load plan details
		sub.Plan, _ = s.GetPlan(ctx, sub.PlanID)
		subscriptions = append(subscriptions, &sub)
	}

	return subscriptions, nil
}

// GetSystemStats returns system-wide statistics (admin only)
func (s *Service) GetSystemStats(ctx context.Context) (*SystemStats, error) {
	query := `SELECT
	    COUNT(DISTINCT ms.merchant_id) as total_merchants,
	    COUNT(DISTINCT CASE WHEN ms.status = 'active' AND ms.plan_id != 'free' THEN ms.merchant_id END) as paying_merchants,
	    SUM(CASE WHEN ms.status = 'active' AND ms.plan_id = 'free' THEN 1 ELSE 0 END) as free_tier_count,
	    SUM(CASE WHEN ms.status = 'active' AND ms.plan_id = 'basic' THEN 1 ELSE 0 END) as basic_tier_count,
	    SUM(CASE WHEN ms.status = 'active' AND ms.plan_id = 'pro' THEN 1 ELSE 0 END) as pro_tier_count,
	    SUM(CASE WHEN ms.status = 'active' AND ms.plan_id = 'enterprise' THEN 1 ELSE 0 END) as enterprise_tier_count
	FROM merchant_subscriptions ms
	WHERE ms.status IN ('active', 'pending_payment')`

	var stats SystemStats
	err := s.db.QueryRow(ctx, query).Scan(
		&stats.TotalMerchants,
		&stats.PayingMerchants,
		&stats.FreeTierCount,
		&stats.BasicTierCount,
		&stats.ProTierCount,
		&stats.EnterpriseTierCount,
	)

	if err != nil {
		return nil, errors.Wrap(err, "failed to get system stats")
	}

	// Get revenue stats
	revenueQuery := `SELECT
	    SUM(sp.price_usd) as monthly_revenue
	FROM merchant_subscriptions ms
	JOIN subscription_plans sp ON ms.plan_id = sp.id
	WHERE ms.status = 'active' AND ms.plan_id != 'free'`

	err = s.db.QueryRow(ctx, revenueQuery).Scan(&stats.MonthlyRevenue)
	if err != nil {
		stats.MonthlyRevenue = decimal.Zero
	}

	return &stats, nil
}

// SystemStats represents system-wide subscription statistics
type SystemStats struct {
	TotalMerchants       int             `json:"total_merchants"`
	PayingMerchants      int             `json:"paying_merchants"`
	FreeTierCount        int             `json:"free_tier_count"`
	BasicTierCount       int             `json:"basic_tier_count"`
	ProTierCount         int             `json:"pro_tier_count"`
	EnterpriseTierCount  int             `json:"enterprise_tier_count"`
	MonthlyRevenue       decimal.Decimal `json:"monthly_revenue"`
}
