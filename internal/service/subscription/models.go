package subscription

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// SubscriptionPlan represents a subscription pricing tier
type SubscriptionPlan struct {
	ID                   string          `json:"id"`
	Name                 string          `json:"name"`
	Description          string          `json:"description"`
	PriceUSD             decimal.Decimal `json:"price_usd"`
	BillingPeriod        string          `json:"billing_period"`
	MaxPaymentsMonthly   sql.NullInt32   `json:"max_payments_monthly"`   // -1 or NULL = unlimited
	MaxMerchants         int32           `json:"max_merchants"`
	MaxAPICallsMonthly   sql.NullInt32   `json:"max_api_calls_monthly"`  // -1 or NULL = unlimited
	Features             json.RawMessage `json:"features"`
	IsActive             bool            `json:"is_active"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}

// MerchantSubscription represents an active subscription for a merchant
type MerchantSubscription struct {
	ID                 int64          `json:"id"`
	UUID               uuid.UUID      `json:"uuid"`
	MerchantID         int64          `json:"merchant_id"`
	PlanID             string         `json:"plan_id"`
	Status             string         `json:"status"` // active, pending_payment, expired, cancelled
	CurrentPeriodStart time.Time      `json:"current_period_start"`
	CurrentPeriodEnd   time.Time      `json:"current_period_end"`
	PaymentID          sql.NullInt64  `json:"payment_id"`
	AutoRenew          bool           `json:"auto_renew"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	CancelledAt        sql.NullTime   `json:"cancelled_at"`

	// Joined fields
	Plan               *SubscriptionPlan `json:"plan,omitempty"`
}

// UsageTracking tracks merchant usage for billing and limits
type UsageTracking struct {
	ID                 int64           `json:"id"`
	MerchantID         int64           `json:"merchant_id"`
	PeriodStart        time.Time       `json:"period_start"`
	PeriodEnd          time.Time       `json:"period_end"`
	PaymentCount       int32           `json:"payment_count"`
	PaymentVolumeUSD   decimal.Decimal `json:"payment_volume_usd"`
	APICallsCount      int32           `json:"api_calls_count"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// Status constants
const (
	StatusActive         = "active"
	StatusPendingPayment = "pending_payment"
	StatusExpired        = "expired"
	StatusCancelled      = "cancelled"
)

// Plan IDs
const (
	PlanFree       = "free"
	PlanBasic      = "basic"
	PlanPro        = "pro"
	PlanEnterprise = "enterprise"
)
