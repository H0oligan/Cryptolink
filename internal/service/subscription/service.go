package subscription

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
)

type Service struct {
	db     *pgxpool.Pool
	logger *zerolog.Logger
}

var (
	ErrPlanNotFound         = errors.New("subscription plan not found")
	ErrSubscriptionNotFound = errors.New("subscription not found")
	ErrUsageNotFound        = errors.New("usage tracking not found")
	ErrLimitExceeded        = errors.New("plan limit exceeded")
)

func New(db *pgxpool.Pool, logger *zerolog.Logger) *Service {
	log := logger.With().Str("channel", "subscription_service").Logger()
	return &Service{
		db:     db,
		logger: &log,
	}
}

// ===== Subscription Plans =====

func (s *Service) GetPlan(ctx context.Context, planID string) (*SubscriptionPlan, error) {
	query := `SELECT id, name, description, price_usd, billing_period, max_payments_monthly,
	                 max_merchants, max_api_calls_monthly, max_volume_monthly_usd, features, is_active, created_at, updated_at
	          FROM subscription_plans WHERE id = $1 AND is_active = true`

	var plan SubscriptionPlan
	err := s.db.QueryRow(ctx, query, planID).Scan(
		&plan.ID, &plan.Name, &plan.Description, &plan.PriceUSD, &plan.BillingPeriod,
		&plan.MaxPaymentsMonthly, &plan.MaxMerchants, &plan.MaxAPICallsMonthly,
		&plan.MaxVolumeMonthlyUSD, &plan.Features, &plan.IsActive, &plan.CreatedAt, &plan.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPlanNotFound
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get subscription plan")
	}

	return &plan, nil
}

func (s *Service) ListPlans(ctx context.Context) ([]*SubscriptionPlan, error) {
	query := `SELECT id, name, description, price_usd, billing_period, max_payments_monthly,
	                 max_merchants, max_api_calls_monthly, max_volume_monthly_usd, features, is_active, created_at, updated_at
	          FROM subscription_plans WHERE is_active = true ORDER BY price_usd ASC`

	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list subscription plans")
	}
	defer rows.Close()

	var plans []*SubscriptionPlan
	for rows.Next() {
		var plan SubscriptionPlan
		err := rows.Scan(
			&plan.ID, &plan.Name, &plan.Description, &plan.PriceUSD, &plan.BillingPeriod,
			&plan.MaxPaymentsMonthly, &plan.MaxMerchants, &plan.MaxAPICallsMonthly,
			&plan.MaxVolumeMonthlyUSD, &plan.Features, &plan.IsActive, &plan.CreatedAt, &plan.UpdatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan subscription plan")
		}
		plans = append(plans, &plan)
	}

	return plans, nil
}

// ===== Merchant Subscriptions =====

func (s *Service) GetActiveSubscription(ctx context.Context, merchantID int64) (*MerchantSubscription, error) {
	query := `SELECT id, uuid, merchant_id, plan_id, status, current_period_start, current_period_end,
	                 payment_id, auto_renew, created_at, updated_at, cancelled_at
	          FROM merchant_subscriptions
	          WHERE merchant_id = $1 AND status IN ('active', 'pending_payment')
	          ORDER BY id DESC LIMIT 1`

	var sub MerchantSubscription
	err := s.db.QueryRow(ctx, query, merchantID).Scan(
		&sub.ID, &sub.UUID, &sub.MerchantID, &sub.PlanID, &sub.Status,
		&sub.CurrentPeriodStart, &sub.CurrentPeriodEnd, &sub.PaymentID,
		&sub.AutoRenew, &sub.CreatedAt, &sub.UpdatedAt, &sub.CancelledAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSubscriptionNotFound
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get active subscription")
	}

	// Load plan details
	sub.Plan, _ = s.GetPlan(ctx, sub.PlanID)

	return &sub, nil
}

func (s *Service) CreateSubscription(ctx context.Context, merchantID int64, planID string, periodStart, periodEnd time.Time) (*MerchantSubscription, error) {
	// Verify plan exists
	plan, err := s.GetPlan(ctx, planID)
	if err != nil {
		return nil, err
	}

	query := `INSERT INTO merchant_subscriptions
	          (uuid, merchant_id, plan_id, status, current_period_start, current_period_end, auto_renew, created_at, updated_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	          RETURNING id, uuid, merchant_id, plan_id, status, current_period_start, current_period_end,
	                    payment_id, auto_renew, created_at, updated_at, cancelled_at`

	var sub MerchantSubscription
	status := StatusActive
	if plan.PriceUSD.GreaterThan(decimal.Zero) {
		status = StatusPendingPayment // Requires payment first
	}

	err = s.db.QueryRow(ctx, query,
		uuid.New(), merchantID, planID, status, periodStart, periodEnd, true, time.Now(), time.Now(),
	).Scan(
		&sub.ID, &sub.UUID, &sub.MerchantID, &sub.PlanID, &sub.Status,
		&sub.CurrentPeriodStart, &sub.CurrentPeriodEnd, &sub.PaymentID,
		&sub.AutoRenew, &sub.CreatedAt, &sub.UpdatedAt, &sub.CancelledAt,
	)

	if err != nil {
		return nil, errors.Wrap(err, "failed to create subscription")
	}

	sub.Plan = plan
	return &sub, nil
}

func (s *Service) ActivateSubscription(ctx context.Context, subscriptionID int64, paymentID int64) error {
	query := `UPDATE merchant_subscriptions
	          SET status = $2, payment_id = $3, updated_at = $4
	          WHERE id = $1`

	_, err := s.db.Exec(ctx, query, subscriptionID, StatusActive, paymentID, time.Now())
	if err != nil {
		return errors.Wrap(err, "failed to activate subscription")
	}

	return nil
}

func (s *Service) CancelSubscription(ctx context.Context, subscriptionID int64) error {
	query := `UPDATE merchant_subscriptions
	          SET auto_renew = false, cancelled_at = $2, updated_at = $3
	          WHERE id = $1`

	_, err := s.db.Exec(ctx, query, subscriptionID, time.Now(), time.Now())
	if err != nil {
		return errors.Wrap(err, "failed to cancel subscription")
	}

	return nil
}

// ===== Usage Tracking =====

func (s *Service) GetCurrentUsage(ctx context.Context, merchantID int64) (*UsageTracking, error) {
	query := `SELECT id, merchant_id, period_start, period_end, payment_count,
	                 payment_volume_usd, api_calls_count, created_at, updated_at
	          FROM usage_tracking
	          WHERE merchant_id = $1 AND period_start <= $2 AND period_end >= $2
	          ORDER BY period_start DESC LIMIT 1`

	var usage UsageTracking
	now := time.Now()
	err := s.db.QueryRow(ctx, query, merchantID, now).Scan(
		&usage.ID, &usage.MerchantID, &usage.PeriodStart, &usage.PeriodEnd,
		&usage.PaymentCount, &usage.PaymentVolumeUSD, &usage.APICallsCount,
		&usage.CreatedAt, &usage.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		// Create new usage period
		return s.createUsagePeriod(ctx, merchantID)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get current usage")
	}

	return &usage, nil
}

func (s *Service) createUsagePeriod(ctx context.Context, merchantID int64) (*UsageTracking, error) {
	now := time.Now()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	periodEnd := periodStart.AddDate(0, 1, 0).Add(-time.Second)

	query := `INSERT INTO usage_tracking
	          (merchant_id, period_start, period_end, payment_count, payment_volume_usd, api_calls_count, created_at, updated_at)
	          VALUES ($1, $2, $3, 0, 0, 0, $4, $5)
	          RETURNING id, merchant_id, period_start, period_end, payment_count, payment_volume_usd, api_calls_count, created_at, updated_at`

	var usage UsageTracking
	err := s.db.QueryRow(ctx, query, merchantID, periodStart, periodEnd, now, now).Scan(
		&usage.ID, &usage.MerchantID, &usage.PeriodStart, &usage.PeriodEnd,
		&usage.PaymentCount, &usage.PaymentVolumeUSD, &usage.APICallsCount,
		&usage.CreatedAt, &usage.UpdatedAt,
	)

	if err != nil {
		return nil, errors.Wrap(err, "failed to create usage period")
	}

	return &usage, nil
}

func (s *Service) IncrementPaymentUsage(ctx context.Context, merchantID int64, volumeUSD decimal.Decimal) error {
	query := `UPDATE usage_tracking
	          SET payment_count = payment_count + 1,
	              payment_volume_usd = payment_volume_usd + $2,
	              updated_at = $3
	          WHERE merchant_id = $1 AND period_start <= $3 AND period_end >= $3`

	result, err := s.db.Exec(ctx, query, merchantID, volumeUSD, time.Now())
	if err != nil {
		return errors.Wrap(err, "failed to increment payment usage")
	}

	if result.RowsAffected() == 0 {
		// Create usage period if it doesn't exist
		_, err = s.createUsagePeriod(ctx, merchantID)
		if err != nil {
			return err
		}
		// Retry the update
		_, err = s.db.Exec(ctx, query, merchantID, volumeUSD, time.Now())
		return err
	}

	return nil
}

func (s *Service) IncrementAPIUsage(ctx context.Context, merchantID int64, count int32) error {
	query := `UPDATE usage_tracking
	          SET api_calls_count = api_calls_count + $2,
	              updated_at = $3
	          WHERE merchant_id = $1 AND period_start <= $3 AND period_end >= $3`

	result, err := s.db.Exec(ctx, query, merchantID, count, time.Now())
	if err != nil {
		return errors.Wrap(err, "failed to increment API usage")
	}

	if result.RowsAffected() == 0 {
		_, err = s.createUsagePeriod(ctx, merchantID)
		if err != nil {
			return err
		}
		_, err = s.db.Exec(ctx, query, merchantID, count, time.Now())
		return err
	}

	return nil
}

// ===== Plan Limit Checks =====

func (s *Service) CheckPaymentLimit(ctx context.Context, merchantID int64) error {
	sub, err := s.GetActiveSubscription(ctx, merchantID)
	if err != nil {
		return err
	}

	// Unlimited plan
	if !sub.Plan.MaxPaymentsMonthly.Valid || sub.Plan.MaxPaymentsMonthly.Int32 == -1 {
		return nil
	}

	usage, err := s.GetCurrentUsage(ctx, merchantID)
	if err != nil {
		return err
	}

	if usage.PaymentCount >= sub.Plan.MaxPaymentsMonthly.Int32 {
		return fmt.Errorf("%w: monthly payment limit of %d reached", ErrLimitExceeded, sub.Plan.MaxPaymentsMonthly.Int32)
	}

	return nil
}

func (s *Service) CheckAPILimit(ctx context.Context, merchantID int64) error {
	sub, err := s.GetActiveSubscription(ctx, merchantID)
	if err != nil {
		return err
	}

	// Unlimited plan
	if !sub.Plan.MaxAPICallsMonthly.Valid || sub.Plan.MaxAPICallsMonthly.Int32 == -1 {
		return nil
	}

	usage, err := s.GetCurrentUsage(ctx, merchantID)
	if err != nil {
		return err
	}

	if usage.APICallsCount >= sub.Plan.MaxAPICallsMonthly.Int32 {
		return fmt.Errorf("%w: monthly API call limit of %d reached", ErrLimitExceeded, sub.Plan.MaxAPICallsMonthly.Int32)
	}

	return nil
}

func (s *Service) CheckVolumeLimit(ctx context.Context, merchantID int64, additionalVolumeUSD decimal.Decimal) error {
	sub, err := s.GetActiveSubscription(ctx, merchantID)
	if err != nil {
		return err
	}

	// NULL = unlimited
	if !sub.Plan.MaxVolumeMonthlyUSD.Valid {
		return nil
	}

	usage, err := s.GetCurrentUsage(ctx, merchantID)
	if err != nil {
		return err
	}

	newTotal := usage.PaymentVolumeUSD.Add(additionalVolumeUSD)
	if newTotal.GreaterThan(sub.Plan.MaxVolumeMonthlyUSD.Decimal) {
		return fmt.Errorf("%w: monthly volume limit of $%s reached (current: $%s)",
			ErrLimitExceeded, sub.Plan.MaxVolumeMonthlyUSD.Decimal.StringFixed(2), usage.PaymentVolumeUSD.StringFixed(2))
	}

	return nil
}

// GetVolumePercentage returns the current volume usage percentage for a merchant
func (s *Service) GetVolumePercentage(ctx context.Context, merchantID int64) (float64, error) {
	sub, err := s.GetActiveSubscription(ctx, merchantID)
	if err != nil {
		return 0, err
	}

	if !sub.Plan.MaxVolumeMonthlyUSD.Valid || sub.Plan.MaxVolumeMonthlyUSD.Decimal.IsZero() {
		return 0, nil // unlimited
	}

	usage, err := s.GetCurrentUsage(ctx, merchantID)
	if err != nil {
		return 0, err
	}

	pct, _ := usage.PaymentVolumeUSD.Div(sub.Plan.MaxVolumeMonthlyUSD.Decimal).Mul(decimal.NewFromInt(100)).Float64()
	return pct, nil
}

// ===== Admin Plan CRUD =====

// ListAllPlans returns all plans including inactive ones (admin only)
func (s *Service) ListAllPlans(ctx context.Context) ([]*SubscriptionPlan, error) {
	query := `SELECT id, name, description, price_usd, billing_period, max_payments_monthly,
	                 max_merchants, max_api_calls_monthly, max_volume_monthly_usd, features, is_active, created_at, updated_at
	          FROM subscription_plans ORDER BY price_usd ASC`

	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list all plans")
	}
	defer rows.Close()

	var plans []*SubscriptionPlan
	for rows.Next() {
		var plan SubscriptionPlan
		err := rows.Scan(
			&plan.ID, &plan.Name, &plan.Description, &plan.PriceUSD, &plan.BillingPeriod,
			&plan.MaxPaymentsMonthly, &plan.MaxMerchants, &plan.MaxAPICallsMonthly,
			&plan.MaxVolumeMonthlyUSD, &plan.Features, &plan.IsActive, &plan.CreatedAt, &plan.UpdatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan plan")
		}
		plans = append(plans, &plan)
	}

	return plans, nil
}

// GetPlanByID returns a plan by ID regardless of active status (admin only)
func (s *Service) GetPlanByID(ctx context.Context, planID string) (*SubscriptionPlan, error) {
	query := `SELECT id, name, description, price_usd, billing_period, max_payments_monthly,
	                 max_merchants, max_api_calls_monthly, max_volume_monthly_usd, features, is_active, created_at, updated_at
	          FROM subscription_plans WHERE id = $1`

	var plan SubscriptionPlan
	err := s.db.QueryRow(ctx, query, planID).Scan(
		&plan.ID, &plan.Name, &plan.Description, &plan.PriceUSD, &plan.BillingPeriod,
		&plan.MaxPaymentsMonthly, &plan.MaxMerchants, &plan.MaxAPICallsMonthly,
		&plan.MaxVolumeMonthlyUSD, &plan.Features, &plan.IsActive, &plan.CreatedAt, &plan.UpdatedAt,
	)

	if err != nil {
		return nil, errors.Wrap(err, "failed to get plan")
	}

	return &plan, nil
}

// CreatePlan creates a new subscription plan (admin only)
func (s *Service) CreatePlan(ctx context.Context, plan *SubscriptionPlan) (*SubscriptionPlan, error) {
	query := `INSERT INTO subscription_plans
	          (id, name, description, price_usd, billing_period, max_payments_monthly,
	           max_merchants, max_api_calls_monthly, max_volume_monthly_usd, features, is_active, created_at, updated_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	          RETURNING id, name, description, price_usd, billing_period, max_payments_monthly,
	                    max_merchants, max_api_calls_monthly, max_volume_monthly_usd, features, is_active, created_at, updated_at`

	now := time.Now()
	var created SubscriptionPlan
	err := s.db.QueryRow(ctx, query,
		plan.ID, plan.Name, plan.Description, plan.PriceUSD, plan.BillingPeriod,
		plan.MaxPaymentsMonthly, plan.MaxMerchants, plan.MaxAPICallsMonthly,
		plan.MaxVolumeMonthlyUSD, plan.Features, plan.IsActive, now, now,
	).Scan(
		&created.ID, &created.Name, &created.Description, &created.PriceUSD, &created.BillingPeriod,
		&created.MaxPaymentsMonthly, &created.MaxMerchants, &created.MaxAPICallsMonthly,
		&created.MaxVolumeMonthlyUSD, &created.Features, &created.IsActive, &created.CreatedAt, &created.UpdatedAt,
	)

	if err != nil {
		return nil, errors.Wrap(err, "failed to create plan")
	}

	return &created, nil
}

// UpdatePlan updates an existing subscription plan (admin only)
func (s *Service) UpdatePlan(ctx context.Context, planID string, plan *SubscriptionPlan) (*SubscriptionPlan, error) {
	query := `UPDATE subscription_plans SET
	          name = $2, description = $3, price_usd = $4, billing_period = $5,
	          max_payments_monthly = $6, max_merchants = $7, max_api_calls_monthly = $8,
	          max_volume_monthly_usd = $9, features = $10, is_active = $11, updated_at = $12
	          WHERE id = $1
	          RETURNING id, name, description, price_usd, billing_period, max_payments_monthly,
	                    max_merchants, max_api_calls_monthly, max_volume_monthly_usd, features, is_active, created_at, updated_at`

	var updated SubscriptionPlan
	err := s.db.QueryRow(ctx, query,
		planID, plan.Name, plan.Description, plan.PriceUSD, plan.BillingPeriod,
		plan.MaxPaymentsMonthly, plan.MaxMerchants, plan.MaxAPICallsMonthly,
		plan.MaxVolumeMonthlyUSD, plan.Features, plan.IsActive, time.Now(),
	).Scan(
		&updated.ID, &updated.Name, &updated.Description, &updated.PriceUSD, &updated.BillingPeriod,
		&updated.MaxPaymentsMonthly, &updated.MaxMerchants, &updated.MaxAPICallsMonthly,
		&updated.MaxVolumeMonthlyUSD, &updated.Features, &updated.IsActive, &updated.CreatedAt, &updated.UpdatedAt,
	)

	if err != nil {
		return nil, errors.Wrap(err, "failed to update plan")
	}

	return &updated, nil
}

// ===== Admin Queries =====

// AdminMerchantInfo represents merchant info for admin view
type AdminMerchantInfo struct {
	ID             int64           `json:"id"`
	UUID           string          `json:"uuid"`
	Name           string          `json:"name"`
	Website        string          `json:"website"`
	CreatorEmail   string          `json:"creator_email"`
	CreatorName    string          `json:"creator_name"`
	ActivePlanID   *string         `json:"active_plan_id"`
	ActivePlanName *string         `json:"active_plan_name"`
	MonthlyVolume  decimal.Decimal `json:"monthly_volume_usd"`
	PaymentCount   int32           `json:"payment_count"`
	CreatedAt      time.Time       `json:"created_at"`
}

// ListAllMerchants returns all merchants with their plan and usage (admin only)
func (s *Service) ListAllMerchants(ctx context.Context, limit, offset int) ([]*AdminMerchantInfo, int, error) {
	if limit <= 0 {
		limit = 50
	}

	countQuery := `SELECT COUNT(*) FROM merchants`
	var total int
	err := s.db.QueryRow(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to count merchants")
	}

	query := `SELECT m.id, m.uuid, m.name, COALESCE(m.website, ''), u.email, COALESCE(u.name, ''),
	                 ms.plan_id, sp.name,
	                 COALESCE(ut.payment_volume_usd, 0), COALESCE(ut.payment_count, 0),
	                 m.created_at
	          FROM merchants m
	          JOIN users u ON m.creator_id = u.id
	          LEFT JOIN merchant_subscriptions ms ON ms.merchant_id = m.id AND ms.status IN ('active', 'pending_payment')
	          LEFT JOIN subscription_plans sp ON ms.plan_id = sp.id
	          LEFT JOIN usage_tracking ut ON ut.merchant_id = m.id AND ut.period_start <= NOW() AND ut.period_end >= NOW()
	          ORDER BY m.created_at DESC
	          LIMIT $1 OFFSET $2`

	rows, err := s.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to list all merchants")
	}
	defer rows.Close()

	var merchants []*AdminMerchantInfo
	for rows.Next() {
		var m AdminMerchantInfo
		err := rows.Scan(
			&m.ID, &m.UUID, &m.Name, &m.Website, &m.CreatorEmail, &m.CreatorName,
			&m.ActivePlanID, &m.ActivePlanName,
			&m.MonthlyVolume, &m.PaymentCount, &m.CreatedAt,
		)
		if err != nil {
			return nil, 0, errors.Wrap(err, "failed to scan merchant")
		}
		merchants = append(merchants, &m)
	}

	return merchants, total, nil
}

// AdminUserInfo represents user info for admin view
type AdminUserInfo struct {
	ID            int64     `json:"id"`
	UUID          string    `json:"uuid"`
	Email         string    `json:"email"`
	Name          string    `json:"name"`
	IsSuperAdmin  bool      `json:"is_super_admin"`
	MerchantCount int       `json:"merchant_count"`
	CreatedAt     time.Time `json:"created_at"`
}

// ListAllUsers returns all users (admin only)
func (s *Service) ListAllUsers(ctx context.Context, limit, offset int) ([]*AdminUserInfo, int, error) {
	if limit <= 0 {
		limit = 50
	}

	countQuery := `SELECT COUNT(*) FROM users`
	var total int
	err := s.db.QueryRow(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to count users")
	}

	query := `SELECT u.id, u.uuid, u.email, COALESCE(u.name, ''), COALESCE(u.is_super_admin, false),
	                 COUNT(m.id),
	                 u.created_at
	          FROM users u
	          LEFT JOIN merchants m ON m.creator_id = u.id
	          GROUP BY u.id, u.uuid, u.email, u.name, u.is_super_admin, u.created_at
	          ORDER BY u.created_at DESC
	          LIMIT $1 OFFSET $2`

	rows, err := s.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to list all users")
	}
	defer rows.Close()

	var users []*AdminUserInfo
	for rows.Next() {
		var u AdminUserInfo
		err := rows.Scan(
			&u.ID, &u.UUID, &u.Email, &u.Name, &u.IsSuperAdmin,
			&u.MerchantCount, &u.CreatedAt,
		)
		if err != nil {
			return nil, 0, errors.Wrap(err, "failed to scan user")
		}
		users = append(users, &u)
	}

	return users, total, nil
}

// AssignMerchantPlan assigns or changes a merchant's subscription plan (admin only)
func (s *Service) AssignMerchantPlan(ctx context.Context, merchantID int64, planID string) error {
	// Verify plan exists
	_, err := s.GetPlanByID(ctx, planID)
	if err != nil {
		return errors.Wrap(err, "invalid plan")
	}

	// Check if merchant exists
	var exists bool
	err = s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM merchants WHERE id = $1)`, merchantID).Scan(&exists)
	if err != nil || !exists {
		return errors.New("merchant not found")
	}

	// Deactivate any existing subscription
	_, err = s.db.Exec(ctx,
		`UPDATE merchant_subscriptions SET status = 'cancelled' WHERE merchant_id = $1 AND status IN ('active', 'pending_payment')`,
		merchantID)
	if err != nil {
		return errors.Wrap(err, "failed to deactivate existing subscription")
	}

	// Create new active subscription
	now := time.Now().UTC()
	periodEnd := now.AddDate(0, 1, 0) // 1 month from now

	_, err = s.db.Exec(ctx,
		`INSERT INTO merchant_subscriptions (uuid, merchant_id, plan_id, status, current_period_start, current_period_end, created_at, updated_at)
		 VALUES ($1, $2, $3, 'active', $4, $5, $6, $6)`,
		uuid.New().String(), merchantID, planID, now, periodEnd, now)
	if err != nil {
		return errors.Wrap(err, "failed to create subscription")
	}

	s.logger.Info().Int64("merchant_id", merchantID).Str("plan_id", planID).Msg("admin assigned plan to merchant")
	return nil
}

func (s *Service) CheckMerchantLimit(ctx context.Context, userID int64, currentCount int) error {
	// Get user's active subscription via any merchant they own
	query := `SELECT ms.plan_id
	          FROM merchant_subscriptions ms
	          JOIN merchants m ON ms.merchant_id = m.id
	          WHERE m.creator_id = $1 AND ms.status = 'active'
	          LIMIT 1`

	var planID string
	err := s.db.QueryRow(ctx, query, userID).Scan(&planID)
	if errors.Is(err, pgx.ErrNoRows) {
		// No subscription, use free plan limits
		planID = PlanFree
	} else if err != nil {
		return errors.Wrap(err, "failed to check merchant limit")
	}

	plan, err := s.GetPlan(ctx, planID)
	if err != nil {
		return err
	}

	if plan.MaxMerchants == -1 {
		return nil // Unlimited
	}

	if currentCount >= int(plan.MaxMerchants) {
		return fmt.Errorf("%w: merchant limit of %d reached for plan %s", ErrLimitExceeded, plan.MaxMerchants, plan.Name)
	}

	return nil
}
